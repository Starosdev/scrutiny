package mqtt

import (
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

const (
	availabilityTopic   = "scrutiny/availability"
	availabilityOnline  = "online"
	availabilityOffline = "offline"

	defaultConnectTimeout = 10 * time.Second
	defaultPublishTimeout = 5 * time.Second
	defaultKeepAlive      = 60 * time.Second
)

// Client wraps the paho MQTT client with Scrutiny-specific configuration.
type Client struct {
	client pahomqtt.Client
	logger *logrus.Entry
	qos    byte
}

// ClientConfig holds MQTT connection parameters.
type ClientConfig struct {
	Broker      string
	Username    string
	Password    string
	ClientID    string
	TopicPrefix string
	QoS         int
	Retain      bool
}

// NewClient creates a new MQTT client configured for Scrutiny.
func NewClient(cfg *ClientConfig, logger *logrus.Entry) *Client {
	c := &Client{
		logger: logger,
		qos:    byte(cfg.QoS),
	}

	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	opts.SetKeepAlive(defaultKeepAlive)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(30 * time.Second)
	opts.SetMaxReconnectInterval(5 * time.Minute)

	// Last Will and Testament: publish "offline" if we disconnect unexpectedly
	opts.SetWill(availabilityTopic, availabilityOffline, byte(cfg.QoS), true)

	opts.SetOnConnectHandler(func(_ pahomqtt.Client) {
		logger.Info("MQTT connected to broker")
		// Publish online status on every (re)connect
		if err := c.publish(availabilityTopic, availabilityOnline, true); err != nil {
			logger.Warnf("MQTT: failed to publish online status: %v", err)
		}
	})

	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		logger.Warnf("MQTT connection lost: %v", err)
	})

	opts.SetReconnectingHandler(func(_ pahomqtt.Client, _ *pahomqtt.ClientOptions) {
		logger.Info("MQTT reconnecting to broker...")
	})

	c.client = pahomqtt.NewClient(opts)
	return c
}

// Connect establishes the connection to the MQTT broker.
func (c *Client) Connect() error {
	token := c.client.Connect()
	if !token.WaitTimeout(defaultConnectTimeout) {
		return fmt.Errorf("MQTT connect timed out after %v", defaultConnectTimeout)
	}
	if token.Error() != nil {
		return fmt.Errorf("MQTT connect failed: %w", token.Error())
	}
	return nil
}

// Disconnect cleanly disconnects from the MQTT broker.
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		// Publish offline before disconnecting
		if err := c.publish(availabilityTopic, availabilityOffline, true); err != nil {
			c.logger.Warnf("MQTT: failed to publish offline status: %v", err)
		}
		c.client.Disconnect(1000) // 1 second grace period
		c.logger.Info("MQTT disconnected from broker")
	}
}

// Publish sends a message to the given topic.
func (c *Client) Publish(topic string, payload string, retained bool) error {
	return c.publish(topic, payload, retained)
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	return c.client != nil && c.client.IsConnected()
}

func (c *Client) publish(topic string, payload string, retained bool) error {
	token := c.client.Publish(topic, c.qos, retained, payload)
	if !token.WaitTimeout(defaultPublishTimeout) {
		return fmt.Errorf("MQTT publish to %s timed out", topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("MQTT publish to %s failed: %w", topic, token.Error())
	}
	return nil
}
