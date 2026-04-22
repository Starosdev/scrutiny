package mdadm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	basecollector "github.com/analogj/scrutiny/collector/pkg/collector"
	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/errors"
	"github.com/analogj/scrutiny/collector/pkg/mdadm/detect"
	"github.com/analogj/scrutiny/collector/pkg/mdadm/models"
	"github.com/sirupsen/logrus"
)

// Collector handles MDADM RAID array collection
type Collector struct {
	config      config.Interface
	logger      *logrus.Entry
	apiEndpoint *url.URL
	httpClient  *http.Client
}

// CreateCollector creates a new MDADM collector
func CreateCollector(appConfig config.Interface, logger *logrus.Entry, apiEndpoint string) (*Collector, error) {
	apiEndpointUrl, err := url.Parse(apiEndpoint)
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

	c := &Collector{
		config:      appConfig,
		logger:      logger,
		apiEndpoint: apiEndpointUrl,
		httpClient:  basecollector.NewAuthHTTPClient(timeout, apiToken),
	}

	return c, nil
}

// Run executes the MDADM collection
func (c *Collector) Run() error {
	c.logger.Infoln("Starting MDADM array collection")

	// Detect arrays
	detector := detect.Detect{
		Logger: c.logger,
		Config: c.config,
	}

	arrays, metrics, err := detector.Start()
	if err != nil {
		return err
	}

	if len(arrays) == 0 {
		c.logger.Infoln("No MDADM arrays found")
		return nil
	}

	c.logger.Infof("Found %d MDADM array(s)", len(arrays))

	// Register arrays with API
	arrayWrapper, err := c.RegisterArrays(arrays)
	if err != nil {
		return err
	}

	if !arrayWrapper.Success {
		c.logger.Errorln("An error occurred while registering arrays")
		return errors.ApiServerCommunicationError("An error occurred while registering arrays")
	}

	// Upload metrics for each registered array
	for i, array := range arrays {
		if err := c.UploadMetrics(array, metrics[i]); err != nil {
			c.logger.Errorf("Failed to upload metrics for array %s (%s): %v", array.Name, array.UUID, err)
			// Continue with other arrays
		}
	}

	c.logger.Infoln("MDADM collection completed")
	return nil
}

// RegisterArrays registers detected arrays with the API
func (c *Collector) RegisterArrays(arrays []models.MDADMArray) (*models.MDADMArrayWrapper, error) {
	c.logger.Infoln("Sending detected arrays to API for registration")

	apiEndpoint, _ := url.Parse(c.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse("api/mdadm/arrays/register")

	wrapper := models.MDADMArrayWrapper{
		Data: arrays,
	}

	jsonData, err := json.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal arrays: %w", err)
	}

	c.logger.Debugf("Registering arrays: %s", string(jsonData))

	resp, err := c.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Errorf("Failed to register arrays: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		c.logger.Errorln("Authentication failed (HTTP 401). Check API token.")
	}

	var responseWrapper models.MDADMArrayWrapper
	if err := json.NewDecoder(resp.Body).Decode(&responseWrapper); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &responseWrapper, nil
}

// UploadMetrics uploads metrics for a specific array
func (c *Collector) UploadMetrics(array models.MDADMArray, metrics models.MDADMMetrics) error {
	c.logger.Infof("Uploading metrics for array %s (%s)", array.Name, array.UUID)

	apiEndpoint, _ := url.Parse(c.apiEndpoint.String())
	// Use UUID in the endpoint path
	apiEndpoint, _ = apiEndpoint.Parse(fmt.Sprintf("api/mdadm/array/%s/metrics", array.UUID))

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal array metrics: %w", err)
	}

	c.logger.Debugf("Uploading array metrics: %s", string(jsonData))

	resp, err := c.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Errorf("Failed to upload metrics for array %s: %v", array.Name, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		c.logger.Errorln("Authentication failed (HTTP 401).")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	c.logger.Infof("Successfully uploaded metrics for array %s", array.Name)
	return nil
}
