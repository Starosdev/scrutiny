package collector_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/analogj/scrutiny/collector/pkg/collector"
	"github.com/stretchr/testify/require"
)

func TestNewAuthHTTPClient_NoToken(t *testing.T) {
	t.Parallel()

	client := collector.NewAuthHTTPClient(60, "")

	// With no token, Transport should be nil (uses http.DefaultTransport)
	require.Nil(t, client.Transport, "transport should be nil when no token is provided")
}

func TestNewAuthHTTPClient_WithToken(t *testing.T) {
	t.Parallel()

	token := "test-secret-token"
	client := collector.NewAuthHTTPClient(60, token)

	// Transport should be set
	require.NotNil(t, client.Transport, "transport should be set when token is provided")

	// Verify the token is injected into requests
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, "Bearer "+token, receivedAuth)
}

func TestNewHTTPClient_NoAuth(t *testing.T) {
	t.Parallel()

	client := collector.NewHTTPClient(60)

	// Verify no auth header is injected
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Empty(t, receivedAuth, "no auth header should be set without token")
}
