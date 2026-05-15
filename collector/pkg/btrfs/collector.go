package btrfs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	basecollector "github.com/analogj/scrutiny/collector/pkg/collector"
	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Collector struct {
	config      config.Interface
	logger      *logrus.Entry
	apiEndpoint *url.URL
	httpClient  *http.Client
}

func CreateCollector(appConfig config.Interface, logger *logrus.Entry, apiEndpoint string) (*Collector, error) {
	apiEndpointURL, err := url.Parse(apiEndpoint)
	if err != nil {
		return nil, err
	}

	timeout := 60
	if appConfig != nil && appConfig.IsSet("api.timeout") {
		timeout = appConfig.GetAPITimeout()
	}

	apiToken := ""
	if appConfig != nil {
		apiToken = appConfig.GetAPIToken()
	}

	return &Collector{
		config:      appConfig,
		logger:      logger,
		apiEndpoint: apiEndpointURL,
		httpClient:  basecollector.NewAuthHTTPClient(timeout, apiToken),
	}, nil
}

func (c *Collector) Run() error {
	c.logger.Infoln("Starting Btrfs filesystem collection")

	detector := Detect{
		Logger: c.logger,
		Config: c.config,
	}

	filesystems, err := detector.Start()
	if err != nil {
		return err
	}
	if len(filesystems) == 0 {
		c.logger.Infoln("No Btrfs filesystems found")
		return nil
	}

	c.logger.Infof("Found %d Btrfs filesystem(s)", len(filesystems))

	valid := make([]Filesystem, 0, len(filesystems))
	for i := range filesystems {
		filesystem := filesystems[i]
		if filesystem.UUID != "" {
			valid = append(valid, filesystem)
		}
	}

	wrapper, err := c.RegisterFilesystems(valid)
	if err != nil {
		return err
	}
	if !wrapper.Success {
		c.logger.Errorln("An error occurred while registering Btrfs filesystems")
		return errors.ApiServerCommunicationError("An error occurred while registering Btrfs filesystems")
	}

	for i := range wrapper.Data {
		filesystem := &wrapper.Data[i]
		if err := c.UploadMetrics(filesystem); err != nil {
			c.logger.Errorf("Failed to upload metrics for filesystem %s: %v", filesystem.UUID, err)
		}
	}

	c.logger.Infoln("Btrfs filesystem collection completed")
	return nil
}

func (c *Collector) RegisterFilesystems(filesystems []Filesystem) (*FilesystemWrapper, error) {
	c.logger.Infoln("Sending detected Btrfs filesystems to API for registration")

	apiEndpoint, _ := url.Parse(c.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse("api/btrfs/filesystems/register")

	wrapper := FilesystemWrapper{
		Data: filesystems,
	}

	jsonData, err := json.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Btrfs filesystems: %w", err)
	}

	resp, err := c.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Errorf("Failed to register Btrfs filesystems: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		c.logger.Errorln("Authentication failed (HTTP 401). Check that api.token in collector-btrfs.yaml matches web.auth.token in scrutiny.yaml.")
	}

	var responseWrapper FilesystemWrapper
	if err := json.NewDecoder(resp.Body).Decode(&responseWrapper); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &responseWrapper, nil
}

func (c *Collector) UploadMetrics(filesystem *Filesystem) error {
	c.logger.Infof("Uploading metrics for Btrfs filesystem %s", filesystem.UUID)

	apiEndpoint, _ := url.Parse(c.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse(fmt.Sprintf("api/btrfs/filesystem/%s/metrics", strings.ToLower(filesystem.UUID)))

	jsonData, err := json.Marshal(filesystem)
	if err != nil {
		return fmt.Errorf("failed to marshal Btrfs filesystem metrics: %w", err)
	}

	resp, err := c.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Errorf("Failed to upload metrics for filesystem %s: %v", filesystem.UUID, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		c.logger.Errorln("Authentication failed (HTTP 401). Check that api.token in collector-btrfs.yaml matches web.auth.token in scrutiny.yaml.")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	return nil
}
