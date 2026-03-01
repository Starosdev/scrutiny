package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/sirupsen/logrus"
)

// Publisher manages MQTT publishing for Home Assistant integration.
type Publisher struct {
	client      *Client
	logger      *logrus.Entry
	config      config.Interface
	topicPrefix string
	retain      bool
	mu          sync.RWMutex
}

// NewPublisher creates a new MQTT publisher.
func NewPublisher(cfg config.Interface, logger *logrus.Entry) *Publisher {
	clientCfg := ClientConfig{
		Broker:      cfg.GetString("web.mqtt.broker"),
		Username:    cfg.GetString("web.mqtt.username"),
		Password:    cfg.GetString("web.mqtt.password"),
		ClientID:    cfg.GetString("web.mqtt.client_id"),
		TopicPrefix: cfg.GetString("web.mqtt.topic_prefix"),
		QoS:         cfg.GetInt("web.mqtt.qos"),
		Retain:      cfg.GetBool("web.mqtt.retain"),
	}

	return &Publisher{
		client:      NewClient(&clientCfg, logger),
		logger:      logger,
		config:      cfg,
		topicPrefix: clientCfg.TopicPrefix,
		retain:      clientCfg.Retain,
	}
}

// Connect establishes the MQTT connection.
func (p *Publisher) Connect() error {
	return p.client.Connect()
}

// Disconnect cleanly disconnects from the MQTT broker.
func (p *Publisher) Disconnect() {
	p.client.Disconnect()
}

// PublishDiscovery publishes HA MQTT Discovery config messages for a device.
func (p *Publisher) PublishDiscovery(device *models.Device) {
	if !p.client.IsConnected() {
		return
	}

	messages := BuildDiscoveryMessages(device, p.topicPrefix)
	for _, msg := range messages {
		if err := p.client.Publish(msg.Topic, msg.Payload, true); err != nil {
			p.logger.Warnf("MQTT: failed to publish discovery for %s: %v", device.WWN, err)
		}
	}
	p.logger.Debugf("MQTT: published discovery for device %s (%s)", device.WWN, device.ModelName)
}

// PublishDeviceState publishes a state update for a device asynchronously.
func (p *Publisher) PublishDeviceState(wwn string, device *models.Device, smartData *measurements.Smart) {
	go func() {
		p.mu.RLock()
		defer p.mu.RUnlock()

		if !p.client.IsConnected() {
			return
		}

		payload := buildStatePayload(device, smartData)
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			p.logger.Warnf("MQTT: failed to marshal state for %s: %v", wwn, err)
			return
		}

		topic := stateTopic(wwn)
		if err := p.client.Publish(topic, string(payloadJSON), p.retain); err != nil {
			p.logger.Warnf("MQTT: failed to publish state for %s: %v", wwn, err)
			return
		}
		p.logger.Debugf("MQTT: published state for device %s", wwn)
	}()
}

// RemoveDevice removes a device from HA by publishing empty discovery messages.
func (p *Publisher) RemoveDevice(device *models.Device) {
	if !p.client.IsConnected() {
		return
	}

	messages := BuildRemoveMessages(device, p.topicPrefix)
	for _, msg := range messages {
		if err := p.client.Publish(msg.Topic, msg.Payload, true); err != nil {
			p.logger.Warnf("MQTT: failed to remove discovery for %s: %v", device.WWN, err)
		}
	}

	// Also clear the state topic
	topic := stateTopic(device.WWN)
	if err := p.client.Publish(topic, "", true); err != nil {
		p.logger.Warnf("MQTT: failed to clear state for %s: %v", device.WWN, err)
	}

	p.logger.Debugf("MQTT: removed device %s from Home Assistant", device.WWN)
}

// LoadInitialData publishes discovery and state for all active devices from the database.
func (p *Publisher) LoadInitialData(deviceRepo database.DeviceRepo, ctx context.Context) error {
	start := time.Now()
	p.logger.Info("MQTT: loading initial device data...")

	summary, err := deviceRepo.GetSummary(ctx)
	if err != nil {
		return fmt.Errorf("MQTT: failed to load device summary: %w", err)
	}

	smartDataMap := p.fetchLatestSmartData(deviceRepo, ctx, summary)

	published := 0
	for _, deviceSummary := range summary {
		device := deviceSummary.Device
		if device.Archived {
			continue
		}

		p.PublishDiscovery(&device)
		p.publishStateSync(device.WWN, &device, smartDataMap)
		published++
	}

	p.logger.Infof("MQTT: published discovery and state for %d devices in %v", published, time.Since(start))
	return nil
}

func (p *Publisher) fetchLatestSmartData(deviceRepo database.DeviceRepo, ctx context.Context, summary map[string]*models.DeviceSummary) map[string][]measurements.Smart {
	smartDataMap := make(map[string][]measurements.Smart)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, deviceSummary := range summary {
		wwn := deviceSummary.Device.WWN
		wg.Add(1)
		go func(w string) {
			defer wg.Done()
			smarts, err := deviceRepo.GetSmartAttributeHistory(ctx, w, "forever", 1, 0, nil)
			if err == nil && len(smarts) > 0 {
				mu.Lock()
				smartDataMap[w] = smarts
				mu.Unlock()
			}
		}(wwn)
	}
	wg.Wait()
	return smartDataMap
}

func (p *Publisher) publishStateSync(wwn string, device *models.Device, smartDataMap map[string][]measurements.Smart) {
	smartResults, ok := smartDataMap[wwn]
	if !ok || len(smartResults) == 0 {
		return
	}

	payload := buildStatePayload(device, &smartResults[0])
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		p.logger.Warnf("MQTT: failed to marshal initial state for %s: %v", wwn, err)
		return
	}
	topic := stateTopic(wwn)
	if err := p.client.Publish(topic, string(payloadJSON), p.retain); err != nil {
		p.logger.Warnf("MQTT: failed to publish initial state for %s: %v", wwn, err)
	}
}

// DeviceStatusString converts a DeviceStatus to a human-readable string.
func DeviceStatusString(status pkg.DeviceStatus) string {
	switch status {
	case pkg.DeviceStatusPassed:
		return "Passed"
	case pkg.DeviceStatusFailedSmart:
		return "Failed (SMART)"
	case pkg.DeviceStatusFailedScrutiny:
		return "Failed (Scrutiny)"
	default:
		if pkg.DeviceStatusHas(status, pkg.DeviceStatusFailedSmart) && pkg.DeviceStatusHas(status, pkg.DeviceStatusFailedScrutiny) {
			return "Failed (Both)"
		}
		return "Unknown"
	}
}

// StatePayload represents the JSON state published to MQTT.
type StatePayload struct {
	Status          string `json:"status"`
	Problem         string `json:"problem"`
	LastUpdated     string `json:"last_updated"`
	Temperature     int64  `json:"temperature"`
	PowerOnHours    int64  `json:"power_on_hours"`
	PowerCycleCount int64  `json:"power_cycle_count"`
}

func buildStatePayload(device *models.Device, smartData *measurements.Smart) StatePayload {
	problem := "OFF"
	if device.DeviceStatus != pkg.DeviceStatusPassed {
		problem = "ON"
	}

	return StatePayload{
		Temperature:     smartData.Temp,
		Status:          DeviceStatusString(device.DeviceStatus),
		PowerOnHours:    smartData.PowerOnHours,
		PowerCycleCount: smartData.PowerCycleCount,
		Problem:         problem,
		LastUpdated:     smartData.Date.Format(time.RFC3339),
	}
}
