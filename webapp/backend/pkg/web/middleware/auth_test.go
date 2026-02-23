package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const testAPIToken = "test-secret-api-token"
const testJWTSecret = "test-jwt-secret-for-signing"
const testMetricsToken = "test-metrics-scrape-token"
const pathSummary = "/api/summary"
const pathMetrics = "/api/metrics"
const pathHealth = "/api/health"
const bearerPrefix = "Bearer "

// setupRouter creates a test gin router with the auth middleware and a simple
// protected handler that returns 200 on success.
func setupRouter(t *testing.T, authEnabled bool, apiToken string, jwtSecret string, metricsToken string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool("web.auth.enabled").Return(authEnabled).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.token").Return(apiToken).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.jwt_secret").Return(jwtSecret).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.auth.jwt_expiry_hours").Return(24).AnyTimes()
	fakeConfig.EXPECT().GetString("web.metrics.token").Return(metricsToken).AnyTimes()

	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(middleware.AuthMiddleware(fakeConfig, logger))

	// Protected endpoint used by most tests
	r.GET(pathSummary, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Metrics endpoint
	r.GET(pathMetrics, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Public endpoints
	r.GET(pathHealth, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	r.HEAD(pathHealth, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/api/auth/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	r.POST("/api/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	return r
}

// --- Phase 1: General auth tests ---

func TestAuthMiddleware_AuthDisabled_AllRequestsPass(t *testing.T) {
	router := setupRouter(t, false, "", "", "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_NoToken_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "Authentication required")
}

func TestAuthMiddleware_AuthEnabled_ValidAPIToken_Returns200(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	req.Header.Set("Authorization", bearerPrefix+testAPIToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_InvalidToken_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	req.Header.Set("Authorization", bearerPrefix+"wrong-token")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "Invalid or expired token")
}

func TestAuthMiddleware_AuthEnabled_ValidJWT_Returns200(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	// Generate a valid JWT using the same secret
	jwtToken, _, err := auth.GenerateJWT(testJWTSecret, 24)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	req.Header.Set("Authorization", bearerPrefix+jwtToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_ExpiredJWT_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	// Generate a JWT with 0 hours expiry (already expired)
	jwtToken, _, err := auth.GenerateJWT(testJWTSecret, 0)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	req.Header.Set("Authorization", bearerPrefix+jwtToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_AuthEnabled_WrongSecretJWT_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	// Generate a JWT signed with a different secret
	jwtToken, _, err := auth.GenerateJWT("different-secret", 24)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	req.Header.Set("Authorization", bearerPrefix+jwtToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// Public route tests: these should be accessible without any token even when auth is enabled.

func TestAuthMiddleware_AuthEnabled_PublicRoute_Health(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathHealth, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_PublicRoute_HealthHEAD(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("HEAD", pathHealth, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_PublicRoute_AuthStatus(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_PublicRoute_AuthLogin(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/auth/login", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// Basepath variation: public routes should work even with a basepath prefix.
func TestAuthMiddleware_AuthEnabled_PublicRoute_WithBasepath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool("web.auth.enabled").Return(true).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.token").Return(testAPIToken).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.jwt_secret").Return(testJWTSecret).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.auth.jwt_expiry_hours").Return(24).AnyTimes()
	fakeConfig.EXPECT().GetString("web.metrics.token").Return("").AnyTimes()

	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(middleware.AuthMiddleware(fakeConfig, logger))

	// Simulate a basepath-prefixed route
	base := r.Group("/scrutiny")
	base.GET(pathHealth, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/scrutiny/api/health", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MalformedAuthHeader_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	req.Header.Set("Authorization", "NotBearer some-token")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Phase 2: Metrics token tests ---

func TestAuthMiddleware_MetricsToken_NotSet_MetricsOpen(t *testing.T) {
	// Auth disabled, no metrics token: metrics endpoint is open
	router := setupRouter(t, false, "", "", "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathMetrics, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MetricsToken_Set_RequiresToken(t *testing.T) {
	// Auth disabled, metrics token set: 401 without token
	router := setupRouter(t, false, "", "", testMetricsToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathMetrics, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Contains(t, response["error"], "Metrics endpoint requires authentication")
}

func TestAuthMiddleware_MetricsToken_Set_ValidToken_Returns200(t *testing.T) {
	// Auth disabled, metrics token set: 200 with correct token
	router := setupRouter(t, false, "", "", testMetricsToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathMetrics, nil)
	req.Header.Set("Authorization", bearerPrefix+testMetricsToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MetricsToken_Set_InvalidToken_Returns401(t *testing.T) {
	// Auth disabled, metrics token set: 401 with wrong token
	router := setupRouter(t, false, "", "", testMetricsToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathMetrics, nil)
	req.Header.Set("Authorization", bearerPrefix+"wrong-metrics-token")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_MetricsToken_WithPhase1Auth_MetricsTokenAccepted(t *testing.T) {
	// Both auth enabled and metrics token set: metrics token grants access to /api/metrics
	router := setupRouter(t, true, testAPIToken, testJWTSecret, testMetricsToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathMetrics, nil)
	req.Header.Set("Authorization", bearerPrefix+testMetricsToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MetricsToken_WithPhase1Auth_APITokenStillWorks(t *testing.T) {
	// Both auth enabled and metrics token set: Phase 1 API token still works for /api/metrics
	router := setupRouter(t, true, testAPIToken, testJWTSecret, testMetricsToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathMetrics, nil)
	req.Header.Set("Authorization", bearerPrefix+testAPIToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// --- Frontend bypass tests ---

func TestAuthMiddleware_FrontendRoutesbypassAuth(t *testing.T) {
	// Auth enabled: non-API routes (frontend static files, SPA) should pass through
	router := setupRouter(t, true, testAPIToken, testJWTSecret, "")

	// Register frontend routes like the real server does
	router.GET("/web", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"page": "index"})
	})
	router.GET("/web/*filepath", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"page": "spa"})
	})
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"page": "root"})
	})

	tests := []struct {
		name string
		path string
	}{
		{"root", "/"},
		{"web index", "/web"},
		{"web SPA route", "/web/dashboard"},
		{"web static file", "/web/main.js"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.path, nil)
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, "frontend route %s should bypass auth", tt.path)
		})
	}
}

func TestAuthMiddleware_MetricsToken_OnlyAffectsMetrics(t *testing.T) {
	// Auth disabled, metrics token set: other endpoints remain open
	router := setupRouter(t, false, "", "", testMetricsToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathSummary, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "non-metrics endpoints should not require metrics token")
}
