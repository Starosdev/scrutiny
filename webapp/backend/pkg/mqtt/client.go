package mqtt

import (
	"errors"
	"fmt"
	"sync"
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

// ErrNotConnected indicates that no MQTT connection is currently open.
var ErrNotConnected = errors.New("MQTT client is not connected")

// Client wraps the paho MQTT client with Scrutiny-specific configuration.
type Client struct {
	client      pahomqtt.Client
	logger      *logrus.Entry
	onReconnect func()
	callbackMu  sync.RWMutex
	qos         byte
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

	opts.SetOnConnectHandler(c.onConnect)

	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		logger.Warnf("MQTT connection lost: %v", err)
	})

	opts.SetReconnectingHandler(func(_ pahomqtt.Client, _ *pahomqtt.ClientOptions) {
		logger.Info("MQTT reconnecting to broker...")
	})

	c.client = pahomqtt.NewClient(opts)
	return c
}

func (c *Client) onConnect(_ pahomqtt.Client) {
	c.logger.Info("MQTT connected to broker")
	if err := c.publish(availabilityTopic, availabilityOnline, true); err != nil {
		c.logger.Warnf("MQTT: failed to publish online status: %v", err)
	}
	c.callbackMu.RLock()
	onReconnect := c.onReconnect
	c.callbackMu.RUnlock()
	if onReconnect != nil {
		onReconnect()
	}
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
	if c.client == nil {
		return
	}
	if c.IsConnected() {
		// Publish offline before disconnecting
		if err := c.publish(availabilityTopic, availabilityOffline, true); err != nil {
			c.logger.Warnf("MQTT: failed to publish offline status: %v", err)
		}
	}
	// Stop connection retries even when the initial connection never succeeded.
	c.client.Disconnect(1000) // 1 second grace period
	c.logger.Info("MQTT disconnected from broker")
}

// Publish sends a message to the given topic.
func (c *Client) Publish(topic string, payload string, retained bool) error {
	return c.publish(topic, payload, retained)
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	// IsConnected also reports true during the initial ConnectRetry backoff.
	return c.client != nil && c.client.IsConnectionOpen()
}

// SetOnReconnect registers a callback invoked every time the connection is
// (re)established, including after an automatic reconnect succeeds.
func (c *Client) SetOnReconnect(fn func()) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.onReconnect = fn
}

func (c *Client) publish(topic string, payload string, retained bool) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}
	token := c.client.Publish(topic, c.qos, retained, payload)
	if !token.WaitTimeout(defaultPublishTimeout) {
		return fmt.Errorf("MQTT publish to %s timed out", topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("MQTT publish to %s failed: %w", topic, token.Error())
	}
	return nil
}
