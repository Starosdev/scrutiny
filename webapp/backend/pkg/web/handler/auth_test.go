package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const testAPIToken = "test-secret-api-token"

// setupAuthStatusRouter creates a router with just the AuthStatus endpoint.
// The CONFIG context value is injected via middleware to match production behavior.
func setupAuthStatusRouter(t *testing.T, authEnabled bool) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool("web.auth.enabled").Return(authEnabled).AnyTimes()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Next()
	})
	r.GET("/api/auth/status", handler.AuthStatus)
	return r
}

// setupLoginRouter creates a router with the Login endpoint and configurable auth settings.
func setupLoginRouter(t *testing.T, authEnabled bool, apiToken string, jwtSecret string, expiryHours int) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool("web.auth.enabled").Return(authEnabled).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.token").Return(apiToken).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.jwt_secret").Return(jwtSecret).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.auth.jwt_expiry_hours").Return(expiryHours).AnyTimes()

	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.POST("/api/auth/login", handler.Login)
	return r
}

// --- AuthStatus tests ---

func TestAuthStatus_AuthDisabled(t *testing.T) {
	router := setupAuthStatusRouter(t, false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, true, response["success"])
	require.Equal(t, false, response["auth_enabled"])
}

func TestAuthStatus_AuthEnabled(t *testing.T) {
	router := setupAuthStatusRouter(t, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, true, response["success"])
	require.Equal(t, true, response["auth_enabled"])
}

// --- Login tests ---

func TestLogin_AuthDisabled(t *testing.T) {
	router := setupLoginRouter(t, false, "", "", 24)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"token": "anything"}`)
	req, _ := http.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, true, response["success"])
	require.Equal(t, false, response["auth_enabled"])
	require.Contains(t, response["message"], "not enabled")
}

func TestLogin_ValidToken(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, "jwt-secret", 24)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"token": "` + testAPIToken + `"}`)
	req, _ := http.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, true, response["success"])
	require.NotEmpty(t, response["token"], "should return a JWT token")
	require.NotEmpty(t, response["expires_at"], "should return expiration time")
	require.Equal(t, "Bearer", response["token_type"])
}

func TestLogin_InvalidToken(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, "jwt-secret", 24)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"token": "wrong-token"}`)
	req, _ := http.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "Invalid token")
}

func TestLogin_MissingTokenField(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, "jwt-secret", 24)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{}`)
	req, _ := http.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_EmptyBody(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, "jwt-secret", 24)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/auth/login", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
