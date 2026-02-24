package collector

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type BaseCollector struct {
	logger     *logrus.Entry
	httpClient *http.Client
}

// authTransport is an http.RoundTripper that injects a Bearer token into every request.
type authTransport struct {
	token string
	base  http.RoundTripper
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
	defer r.Body.Close()

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
	defer r.Body.Close()

	if r.StatusCode == 401 {
		c.logger.Errorln("Authentication failed (HTTP 401). Check that api.token in collector.yaml matches web.auth.token in scrutiny.yaml.")
	}

	return json.NewDecoder(r.Body).Decode(target)
}

// http://www.linuxguide.it/command_line/linux-manpage/do.php?file=smartctl#sect7
func (c *BaseCollector) LogSmartctlExitCode(exitCode int) {
	if exitCode&0x01 != 0 {
		c.logger.Errorln("smartctl could not parse commandline")
	} else if exitCode&0x02 != 0 {
		c.logger.Errorln("smartctl could not open device")
	} else if exitCode&0x04 != 0 {
		c.logger.Errorln("smartctl detected a checksum error")
	} else if exitCode&0x08 != 0 {
		c.logger.Errorln("smartctl detected a failing disk ")
	} else if exitCode&0x10 != 0 {
		c.logger.Errorln("smartctl detected a disk in pre-fail")
	} else if exitCode&0x20 != 0 {
		c.logger.Errorln("smartctl detected a disk close to failure")
	} else if exitCode&0x40 != 0 {
		c.logger.Errorln("smartctl detected a error log with errors")
	} else if exitCode&0x80 != 0 {
		c.logger.Errorln("smartctl detected a self test log with errors")
	}
}
