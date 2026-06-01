package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type BaseCollector struct {
	logger     *logrus.Entry
	httpClient *http.Client
}

type httpStatusError struct {
	Status     string
	Body       string
	StatusCode int
}

func (e *httpStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("unexpected API status: %s", e.Status)
	}
	return fmt.Sprintf("unexpected API status: %s: %s", e.Status, e.Body)
}

// authTransport is an http.RoundTripper that injects a Bearer token into every request.
type authTransport struct {
	base  http.RoundTripper
	token string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

// NewHTTPClient creates an HTTP client with the specified timeout in seconds
func NewHTTPClient(timeoutSeconds int) *http.Client {
	return &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
}

// NewAuthHTTPClient creates an HTTP client that injects a Bearer token when apiToken is non-empty.
func NewAuthHTTPClient(timeoutSeconds int, apiToken string) *http.Client {
	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	if apiToken != "" {
		client.Transport = &authTransport{token: apiToken, base: http.DefaultTransport}
	}
	return client
}

func (c *BaseCollector) getJson(url string, target interface{}) error {

	r, err := c.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer drainAndClose(r.Body)

	if err := validateAPIResponse(c.logger, r); err != nil {
		return err
	}

	return json.NewDecoder(r.Body).Decode(target)
}

func (c *BaseCollector) postJson(url string, body interface{}, target interface{}) error {
	requestBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	r, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	defer drainAndClose(r.Body)

	if err := validateAPIResponse(c.logger, r); err != nil {
		return err
	}

	return json.NewDecoder(r.Body).Decode(target)
}

func validateAPIResponse(logger *logrus.Entry, resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized && logger != nil {
		logger.Errorln("Authentication failed (HTTP 401). Check that api.token in collector.yaml matches web.auth.token in scrutiny.yaml.")
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return newHTTPStatusError(resp)
}

func newHTTPStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	bodyText := strings.TrimSpace(string(body))
	if len(bodyText) > 256 {
		bodyText = bodyText[:256] + "...(truncated)"
	}
	return &httpStatusError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       bodyText,
	}
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

// LogSmartctlExitCode logs each set bit in the smartctl exit code bitmask.
// Fatal bits (0x01, 0x02) are logged at ERROR; health-related bits (0x08,
// 0x10, 0x20) at WARN; purely informational bits (0x04, 0x40, 0x80) at INFO.
// http://www.linuxguide.it/command_line/linux-manpage/do.php?file=smartctl#sect7
func (c *BaseCollector) LogSmartctlExitCode(exitCode int, deviceName string) {
	if exitCode&0x01 != 0 {
		c.logger.Errorf("smartctl could not parse command line for %s", deviceName)
	}
	if exitCode&0x02 != 0 {
		c.logger.Errorf("smartctl could not open device %s", deviceName)
	}
	if exitCode&0x04 != 0 {
		c.logger.Infof("smartctl detected a checksum error for %s (bit 0x04)", deviceName)
	}
	if exitCode&0x08 != 0 {
		c.logger.Warnf("smartctl detected a failing disk for %s (bit 0x08)", deviceName)
	}
	if exitCode&0x10 != 0 {
		c.logger.Warnf("smartctl detected a disk in pre-fail for %s (bit 0x10)", deviceName)
	}
	if exitCode&0x20 != 0 {
		c.logger.Warnf("smartctl detected a disk close to failure for %s (bit 0x20)", deviceName)
	}
	if exitCode&0x40 != 0 {
		c.logger.Infof("smartctl error log contains records of errors for %s (bit 0x40)", deviceName)
	}
	if exitCode&0x80 != 0 {
		c.logger.Infof("smartctl self-test log contains errors for %s (bit 0x80)", deviceName)
	}
}
