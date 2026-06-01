package collector

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	collectorconfig "github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestMetricsPublishRetriesTransportErrors(t *testing.T) {
	t.Parallel()

	var attempts int32
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if atomic.AddInt32(&attempts, 1) == 1 {
				return nil, fmt.Errorf("dial tcp 10.0.0.10:8080: connect: connection refused")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			}, nil
		}),
	}

	collector := newTestMetricsCollector(t, client, "http://example.com/", 1, 0)

	err := collector.Publish("device-1", []byte(`{"smartctl":{}}`))

	require.NoError(t, err)
	require.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

func TestMetricsPublishRetriesTransientHTTPStatus(t *testing.T) {
	t.Parallel()

	var attempts int32
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if atomic.AddInt32(&attempts, 1) == 1 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Status:     "503 Service Unavailable",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`temporarily unavailable`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			}, nil
		}),
	}

	collector := newTestMetricsCollector(t, client, "http://example.com/", 1, 0)

	err := collector.Publish("device-1", []byte(`{"smartctl":{}}`))

	require.NoError(t, err)
	require.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

func TestMetricsPublishDoesNotRetryNonRetriableHTTPStatus(t *testing.T) {
	t.Parallel()

	var attempts int32
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&attempts, 1)
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`bad payload`)),
			}, nil
		}),
	}

	collector := newTestMetricsCollector(t, client, "http://example.com/", 3, 0)

	err := collector.Publish("device-1", []byte(`{"smartctl":{}}`))

	require.Error(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))

	statusErr, ok := err.(*httpStatusError)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusErr.StatusCode)
}

func TestMetricsPublishReusesConnectionsAcrossSequentialRequests(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		remoteAddrs = map[string]struct{}{}
	)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state != http.StateNew {
			return
		}
		mu.Lock()
		remoteAddrs[conn.RemoteAddr().String()] = struct{}{}
		mu.Unlock()
	}
	server.Start()
	defer server.Close()

	collector := newTestMetricsCollector(t, server.Client(), server.URL+"/", 0, 0)
	for i := 0; i < 5; i++ {
		require.NoError(t, collector.Publish("device-1", []byte(`{"smartctl":{}}`)))
	}

	mu.Lock()
	connectionCount := len(remoteAddrs)
	mu.Unlock()

	require.Equal(t, 1, connectionCount)
}

func newTestMetricsCollector(t *testing.T, client *http.Client, endpoint string, retryCount int, retryDelay int) MetricsCollector {
	t.Helper()

	cfg, err := collectorconfig.Create()
	require.NoError(t, err)
	cfg.Set(configKeyMetricsAPIRetryCount, retryCount)
	cfg.Set(configKeyMetricsAPIRetryDelay, retryDelay)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	parsedURL, err := url.Parse(endpoint)
	require.NoError(t, err)

	return MetricsCollector{
		config:      cfg,
		apiEndpoint: parsedURL,
		BaseCollector: BaseCollector{
			logger:     logrus.NewEntry(logger),
			httpClient: client,
		},
	}
}
