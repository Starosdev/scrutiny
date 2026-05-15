package filesystem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	basecollector "github.com/analogj/scrutiny/collector/pkg/collector"
	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

// Collector handles filesystem capacity collection.
type Collector struct {
	config      config.Interface
	logger      *logrus.Entry
	apiEndpoint *url.URL
	httpClient  *http.Client
	now         func() time.Time
}

// CreateCollector creates a new filesystem collector.
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
		now:         time.Now,
	}, nil
}

// Run executes filesystem capacity collection and upload.
func (c *Collector) Run() error {
	c.logger.Infoln("Starting filesystem capacity collection")

	hostID := c.config.GetString("host.id")
	if hostID == "" {
		hostID = "default"
	}

	snapshots, hostStatus, err := CollectLinuxSnapshots(hostID, c.now())
	if err != nil {
		return err
	}

	payload := models.FilesystemSummaryUpload{
		Filesystems: snapshots,
		Hosts:       []models.FilesystemHostStatus{hostStatus},
	}

	c.logger.Infof("Collected %d filesystem snapshot(s) for host %s with status %s", len(snapshots), hostStatus.HostID, hostStatus.Status)
	return c.upload(payload)
}

func (c *Collector) upload(payload models.FilesystemSummaryUpload) error {
	apiEndpoint, _ := url.Parse(c.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse("api/filesystems/summary")

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal filesystem summary: %w", err)
	}

	c.logger.Debugf("Uploading filesystem summary: %s", string(jsonData))

	resp, err := c.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to upload filesystem summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		c.logger.Errorln("Authentication failed (HTTP 401). Check that api.token in collector-filesystem.yaml matches web.auth.token in scrutiny.yaml.")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("filesystem summary API returned status %d", resp.StatusCode)
	}

	c.logger.Infoln("Filesystem capacity collection completed")
	return nil
}
