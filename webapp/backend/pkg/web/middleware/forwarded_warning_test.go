package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func forwardedRouter(trusted []string) (*gin.Engine, *logtest.Hook) {
	gin.SetMode(gin.TestMode)
	logger, hook := logtest.NewNullLogger()
	r := gin.New()
	_ = r.SetTrustedProxies(trusted)
	r.Use(ForwardedForWarningMiddleware(logger))
	r.GET("/", func(c *gin.Context) {})
	return r, hook
}

func forwardedRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	return req
}

func TestForwardedForWarning_OnceForUntrustedPeer(t *testing.T) {
	r, hook := forwardedRouter(nil)
	for i := 0; i < 3; i++ {
		r.ServeHTTP(httptest.NewRecorder(), forwardedRequest())
	}
	require.Len(t, hook.AllEntries(), 1)
	require.Contains(t, hook.LastEntry().Message, "10.0.0.5")
	require.NotContains(t, hook.LastEntry().Message, "203.0.113.9", "header value must not be logged")
}

func TestForwardedForWarning_SilentForTrustedPeer(t *testing.T) {
	r, hook := forwardedRouter([]string{"10.0.0.0/8"})
	r.ServeHTTP(httptest.NewRecorder(), forwardedRequest())
	require.Empty(t, hook.AllEntries())
}

func TestForwardedForWarning_SilentWithoutHeader(t *testing.T) {
	r, hook := forwardedRouter(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	r.ServeHTTP(httptest.NewRecorder(), req)
	require.Empty(t, hook.AllEntries())
}
