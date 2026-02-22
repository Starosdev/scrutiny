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

// setupRouter creates a test gin router with the auth middleware and a simple
// protected handler that returns 200 on success.
func setupRouter(t *testing.T, authEnabled bool, apiToken string, jwtSecret string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool("web.auth.enabled").Return(authEnabled).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.token").Return(apiToken).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.jwt_secret").Return(jwtSecret).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.auth.jwt_expiry_hours").Return(24).AnyTimes()

	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(middleware.AuthMiddleware(fakeConfig, logger))

	// Protected endpoint used by most tests
	r.GET("/api/summary", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Public endpoints
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	r.HEAD("/api/health", func(c *gin.Context) {
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

func TestAuthMiddleware_AuthDisabled_AllRequestsPass(t *testing.T) {
	router := setupRouter(t, false, "", "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthDisabled_NoTokenRequired(t *testing.T) {
	router := setupRouter(t, false, "", "")

	// No Authorization header at all -- should still pass
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_NoToken_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "Authentication required")
}

func TestAuthMiddleware_AuthEnabled_ValidAPIToken_Returns200(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_InvalidToken_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "Invalid or expired token")
}

func TestAuthMiddleware_AuthEnabled_ValidJWT_Returns200(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	// Generate a valid JWT using the same secret
	jwtToken, _, err := auth.GenerateJWT(testJWTSecret, 24)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_ExpiredJWT_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	// Generate a JWT with 0 hours expiry (already expired)
	jwtToken, _, err := auth.GenerateJWT(testJWTSecret, 0)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_AuthEnabled_WrongSecretJWT_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	// Generate a JWT signed with a different secret
	jwtToken, _, err := auth.GenerateJWT("different-secret", 24)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// Public route tests: these should be accessible without any token even when auth is enabled.

func TestAuthMiddleware_AuthEnabled_PublicRoute_Health(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_PublicRoute_HealthHEAD(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("HEAD", "/api/health", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_PublicRoute_AuthStatus(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AuthEnabled_PublicRoute_AuthLogin(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

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

	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(middleware.AuthMiddleware(fakeConfig, logger))

	// Simulate a basepath-prefixed route
	base := r.Group("/scrutiny")
	base.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/scrutiny/api/health", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MalformedAuthHeader_Returns401(t *testing.T) {
	router := setupRouter(t, true, testAPIToken, testJWTSecret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/summary", nil)
	req.Header.Set("Authorization", "NotBearer some-token")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
