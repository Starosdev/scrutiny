package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func clientIPFor(t *testing.T, proxies []string, remoteAddr string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	configureTrustedProxies(r, proxies, logrus.NewEntry(logrus.New()))
	var got string
	r.GET("/ip", func(c *gin.Context) { got = c.ClientIP() })

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func TestTrustedProxies_DefaultIgnoresForwardedFor(t *testing.T) {
	require.Equal(t, "10.0.0.5", clientIPFor(t, nil, "10.0.0.5:4444"))
	require.Equal(t, "10.0.0.5", clientIPFor(t, []string{}, "10.0.0.5:4444"))
}

func TestTrustedProxies_ConfiguredProxyIsHonoured(t *testing.T) {
	require.Equal(t, "203.0.113.9", clientIPFor(t, []string{"10.0.0.0/8"}, "10.0.0.5:4444"))
}

func TestTrustedProxies_UntrustedPeerCannotSpoof(t *testing.T) {
	require.Equal(t, "198.51.100.7", clientIPFor(t, []string{"10.0.0.0/8"}, "198.51.100.7:4444"))
}

func TestTrustedProxies_InvalidEntryFallsBackToNone(t *testing.T) {
	require.Equal(t, "10.0.0.5", clientIPFor(t, []string{"not-an-ip"}, "10.0.0.5:4444"))
}

func TestValidateTrustedProxies(t *testing.T) {
	require.NoError(t, validateTrustedProxies([]string{"10.0.0.1", "192.168.0.0/16", "::1"}))
	require.Error(t, validateTrustedProxies([]string{"10.0.0.1", "proxy.local"}))
}
