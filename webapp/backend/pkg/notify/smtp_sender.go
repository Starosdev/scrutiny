package notify

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	shoutrrrsmtp "github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
)

func (n *Notify) SendSMTPNotification(rawURL string) error {
	serviceURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("failed to parse smtp notification URL: %w", err)
	}

	config, err := n.buildSMTPConfig(serviceURL)
	if err != nil {
		return err
	}

	n.Logger.Infof("Sending SMTP notification to %s", config.Host)

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	client, err := openSMTPClient(ctx, config)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := negotiateSMTP(client, config); err != nil {
		return err
	}

	for _, toAddress := range config.ToAddresses {
		if err := sendSMTPRecipient(client, config, toAddress, n.Payload.Message, n.Payload.HTMLMessage); err != nil {
			return err
		}
	}

	if err := client.Quit(); err != nil && !strings.Contains(err.Error(), "short response") {
		return fmt.Errorf("smtp quit failed: %w", err)
	}
	return nil
}

// buildSMTPConfig constructs the shoutrrr SMTP config from the service URL and current payload,
// applying defaults for auth type and subject.
func (n *Notify) buildSMTPConfig(serviceURL *url.URL) (*shoutrrrsmtp.Config, error) {
	config := &shoutrrrsmtp.Config{
		Port:        shoutrrrsmtp.DefaultSMTPPort,
		ToAddresses: nil,
		Subject:     "",
		Auth:        shoutrrrsmtp.AuthTypes.Unknown,
		UseStartTLS: true,
		UseHTML:     n.Payload.HTMLMessage != "",
		Encryption:  shoutrrrsmtp.EncMethods.Auto,
		ClientHost:  "localhost",
		Timeout:     10 * time.Second,
	}
	if err := config.SetURL(serviceURL); err != nil {
		return nil, fmt.Errorf("failed to parse smtp config: %w", err)
	}
	if config.Auth == shoutrrrsmtp.AuthTypes.Unknown {
		if config.Username != "" {
			config.Auth = shoutrrrsmtp.AuthTypes.Plain
		} else {
			config.Auth = shoutrrrsmtp.AuthTypes.None
		}
	}
	config.Subject = n.Payload.Subject
	config.FixEmailTags()
	return config, nil
}

// negotiateSMTP performs the EHLO/STARTTLS/AUTH handshake on an open SMTP client.
func negotiateSMTP(client *smtp.Client, config *shoutrrrsmtp.Config) error {
	if err := client.Hello(resolveClientHost(config.ClientHost)); err != nil {
		return fmt.Errorf("smtp hello failed: %w", err)
	}

	if config.UseStartTLS && !smtpUseImplicitTLS(config) {
		if err := startTLSIfSupported(client, config); err != nil {
			return err
		}
	}

	auth, err := smtpAuthForConfig(config)
	if err != nil {
		return err
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}
	return nil
}

// resolveClientHost returns the EHLO client hostname, resolving "auto"/empty to the OS hostname.
func resolveClientHost(clientHost string) string {
	if clientHost != "auto" && clientHost != "" {
		return clientHost
	}
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "localhost"
}

// startTLSIfSupported upgrades the connection via STARTTLS when the server advertises it,
// erroring only when STARTTLS is required but unavailable.
func startTLSIfSupported(client *smtp.Client, config *shoutrrrsmtp.Config) error {
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{
			ServerName:         config.Host,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
			InsecureSkipVerify: config.SkipTLSVerify,
		}); err != nil {
			return fmt.Errorf("smtp starttls failed: %w", err)
		}
		return nil
	}
	if config.RequireStartTLS {
		return fmt.Errorf("smtp server does not support STARTTLS")
	}
	return nil
}

func openSMTPClient(ctx context.Context, config *shoutrrrsmtp.Config) (*smtp.Client, error) {
	addr := net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.Port), 10))

	var (
		conn net.Conn
		err  error
	)
	if smtpUseImplicitTLS(config) {
		dialer := &tls.Dialer{
			Config: &tls.Config{
				ServerName:         config.Host,
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: config.SkipTLSVerify,
			},
		}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		dialer := &net.Dialer{}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("smtp connect failed: %w", err)
	}

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp client init failed: %w", err)
	}
	return client, nil
}

func smtpUseImplicitTLS(config *shoutrrrsmtp.Config) bool {
	switch config.Encryption {
	case shoutrrrsmtp.EncMethods.None, shoutrrrsmtp.EncMethods.ExplicitTLS:
		return false
	case shoutrrrsmtp.EncMethods.ImplicitTLS:
		return true
	default:
		return config.Port == shoutrrrsmtp.ImplicitTLSPort
	}
}

func smtpAuthForConfig(config *shoutrrrsmtp.Config) (smtp.Auth, error) {
	switch config.Auth {
	case shoutrrrsmtp.AuthTypes.None:
		return nil, nil
	case shoutrrrsmtp.AuthTypes.Plain:
		return smtp.PlainAuth("", config.Username, config.Password, config.Host), nil
	case shoutrrrsmtp.AuthTypes.CRAMMD5:
		return smtp.CRAMMD5Auth(config.Username, config.Password), nil
	case shoutrrrsmtp.AuthTypes.OAuth2:
		return shoutrrrsmtp.OAuth2Auth(config.Username, config.Password), nil
	default:
		return nil, fmt.Errorf("unsupported smtp auth type: %s", config.Auth.String())
	}
}

func sendSMTPRecipient(client *smtp.Client, config *shoutrrrsmtp.Config, toAddress, textBody, htmlBody string) error {
	if err := client.Mail(config.FromAddress); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(toAddress); err != nil {
		return fmt.Errorf("smtp RCPT TO failed: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}

	body, err := buildSMTPMessage(config, toAddress, textBody, htmlBody)
	if err != nil {
		_ = writer.Close()
		return err
	}

	if _, err := writer.Write(body.Bytes()); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp data close failed: %w", err)
	}
	return nil
}

// smtpHeaders builds the common SMTP message headers for a recipient and content type.
func smtpHeaders(config *shoutrrrsmtp.Config, toAddress, contentType string) map[string]string {
	return map[string]string{
		"Subject":      config.Subject,
		"Date":         time.Now().Format(time.RFC1123Z),
		"To":           toAddress,
		"From":         formatSMTPFrom(config),
		"MIME-Version": "1.0",
		"Content-Type": contentType,
	}
}

// buildSMTPMessage renders the full RFC822 message body, multipart/alternative when an HTML body
// is present, otherwise plain text.
func buildSMTPMessage(config *shoutrrrsmtp.Config, toAddress, textBody, htmlBody string) (*bytes.Buffer, error) {
	var body bytes.Buffer

	if htmlBody == "" {
		if err := writeSMTPHeaders(&body, smtpHeaders(config, toAddress, "text/plain; charset=UTF-8")); err != nil {
			return nil, err
		}
		body.WriteString(normalizeSMTPBody(textBody))
		return &body, nil
	}

	boundary, err := randomSMTPBoundary()
	if err != nil {
		return nil, err
	}
	contentType := fmt.Sprintf("multipart/alternative; boundary=%q", boundary)
	if err := writeSMTPHeaders(&body, smtpHeaders(config, toAddress, contentType)); err != nil {
		return nil, err
	}
	if err := writeSMTPPart(&body, boundary, "text/plain; charset=UTF-8", textBody); err != nil {
		return nil, err
	}
	if err := writeSMTPPart(&body, boundary, "text/html; charset=UTF-8", htmlBody); err != nil {
		return nil, err
	}
	fmt.Fprintf(&body, "--%s--\r\n", boundary)
	return &body, nil
}

func formatSMTPFrom(config *shoutrrrsmtp.Config) string {
	if config.FromName == "" {
		return config.FromAddress
	}
	return fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress)
}

func writeSMTPHeaders(buf *bytes.Buffer, headers map[string]string) error {
	for key, value := range headers {
		if _, err := fmt.Fprintf(buf, "%s: %s\r\n", textproto.CanonicalMIMEHeaderKey(key), value); err != nil {
			return fmt.Errorf("failed to write smtp headers: %w", err)
		}
	}
	_, err := buf.WriteString("\r\n")
	return err
}

func writeSMTPPart(buf *bytes.Buffer, boundary, contentType, body string) error {
	if _, err := fmt.Fprintf(buf, "--%s\r\n", boundary); err != nil {
		return fmt.Errorf("failed to write smtp boundary: %w", err)
	}
	if _, err := fmt.Fprintf(buf, "Content-Type: %s\r\n\r\n", contentType); err != nil {
		return fmt.Errorf("failed to write smtp content type: %w", err)
	}
	if _, err := buf.WriteString(normalizeSMTPBody(body)); err != nil {
		return fmt.Errorf("failed to write smtp body: %w", err)
	}
	if !strings.HasSuffix(body, "\n") && !strings.HasSuffix(body, "\r\n") {
		_, _ = buf.WriteString("\r\n")
	}
	return nil
}

func normalizeSMTPBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	return body
}

func randomSMTPBoundary() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate smtp boundary: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
