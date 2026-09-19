package mqtt

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type connectionClient struct {
	pahomqtt.Client
	open        bool
	publishes   atomic.Int32
	disconnects int
}

func (c *connectionClient) IsConnected() bool      { return true }
func (c *connectionClient) IsConnectionOpen() bool { return c.open }
func (c *connectionClient) Publish(_ string, _ byte, _ bool, _ interface{}) pahomqtt.Token {
	c.publishes.Add(1)
	return completedToken{}
}
func (c *connectionClient) Disconnect(uint) { c.disconnects++ }

type completedToken struct{ pahomqtt.Token }

func (completedToken) WaitTimeout(_ time.Duration) bool { return true }
func (completedToken) Error() error                     { return nil }

func testLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logrus.NewEntry(logger)
}

func TestClientRequiresOpenConnection(t *testing.T) {
	require.False(t, (&Client{}).IsConnected())
	transport := &connectionClient{}
	client := &Client{client: transport, logger: testLogger()}
	require.False(t, client.IsConnected(), "Paho reports connected during ConnectRetry")
	require.ErrorIs(t, client.Publish("test", "payload", true), ErrNotConnected)
	require.Zero(t, transport.publishes.Load())
	transport.open = true
	require.True(t, client.IsConnected())
	require.NoError(t, client.Publish("test", "payload", true))
	require.EqualValues(t, 1, transport.publishes.Load())
}

func TestDisconnectStopsInitialConnectionRetry(t *testing.T) {
	transport := &connectionClient{}
	client := &Client{client: transport, logger: testLogger()}
	client.Disconnect()
	require.Equal(t, 1, transport.disconnects)
	require.Zero(t, transport.publishes.Load(), "offline status must not wait on a nonexistent socket")
}

func TestReconnectCallbackCanReplaceItself(t *testing.T) {
	transport := &connectionClient{open: true}
	client := &Client{client: transport, logger: testLogger()}
	calls := 0
	client.SetOnReconnect(func() {
		calls++
		client.SetOnReconnect(nil)
	})
	client.onConnect(transport)
	client.onConnect(transport)
	require.Equal(t, 1, calls)
	require.EqualValues(t, 2, transport.publishes.Load())
}

func TestReconnectCallbackConcurrentUpdate(t *testing.T) {
	transport := &connectionClient{open: true}
	client := &Client{client: transport, logger: testLogger()}
	var calls atomic.Int32
	callback := func() { calls.Add(1) }
	client.SetOnReconnect(callback)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for range 100 {
			client.SetOnReconnect(callback)
		}
	}()
	go func() {
		defer workers.Done()
		for range 100 {
			client.onConnect(transport)
		}
	}()
	workers.Wait()
	require.EqualValues(t, 100, calls.Load())
}
