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
const testAdminUsername = "admin"
const testAdminPassword = "test-admin-password"
const testJWTSecret = "jwt-secret"
const pathAuthStatus = "/api/auth/status"
const pathAuthLogin = "/api/auth/login"
const headerContentType = "Content-Type"
const mimeJSON = "application/json"

// setupAuthStatusRouter creates a router with just the AuthStatus endpoint.
func setupAuthStatusRouter(t *testing.T, authEnabled bool, adminPassword string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool("web.auth.enabled").Return(authEnabled).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.admin_password").Return(adminPassword).AnyTimes()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Next()
	})
	r.GET(pathAuthStatus, handler.AuthStatus)
	return r
}

// setupLoginRouter creates a router with the Login endpoint and configurable auth settings.
func setupLoginRouter(t *testing.T, authEnabled bool, apiToken string, jwtSecret string, expiryHours int, adminUsername string, adminPassword string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool("web.auth.enabled").Return(authEnabled).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.token").Return(apiToken).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.jwt_secret").Return(jwtSecret).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.auth.jwt_expiry_hours").Return(expiryHours).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.admin_username").Return(adminUsername).AnyTimes()
	fakeConfig.EXPECT().GetString("web.auth.admin_password").Return(adminPassword).AnyTimes()

	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.POST(pathAuthLogin, handler.Login)
	return r
}

// --- AuthStatus tests ---

func TestAuthStatus_AuthDisabled(t *testing.T) {
	router := setupAuthStatusRouter(t, false, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathAuthStatus, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, true, response["success"])
	require.Equal(t, false, response["auth_enabled"])
	require.Nil(t, response["login_methods"], "login_methods should not be present when auth is disabled")
}

func TestAuthStatus_AuthEnabled_TokenOnly(t *testing.T) {
	router := setupAuthStatusRouter(t, true, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathAuthStatus, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, true, response["success"])
	require.Equal(t, true, response["auth_enabled"])

	methods, ok := response["login_methods"].([]interface{})
	require.True(t, ok, "login_methods should be an array")
	require.Equal(t, 1, len(methods))
	require.Equal(t, "token", methods[0])
}

func TestAuthStatus_AuthEnabled_TokenAndPassword(t *testing.T) {
	router := setupAuthStatusRouter(t, true, testAdminPassword)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", pathAuthStatus, nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	methods, ok := response["login_methods"].([]interface{})
	require.True(t, ok, "login_methods should be an array")
	require.Equal(t, 2, len(methods))
	require.Equal(t, "token", methods[0])
	require.Equal(t, "password", methods[1])
}

// --- Token login tests ---

func TestLogin_AuthDisabled(t *testing.T) {
	router := setupLoginRouter(t, false, "", "", 24, "", "")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"token": "anything"}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
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
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, "")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"token": "` + testAPIToken + `"}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
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
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, "")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"token": "wrong-token"}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "Invalid token")
}

func TestLogin_EmptyBody(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", pathAuthLogin, nil)
	req.Header.Set(headerContentType, mimeJSON)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_NoCredentials(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, "")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Contains(t, response["error"], "Provide either")
}

// --- Password login tests ---

func TestLogin_ValidPassword(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, testAdminPassword)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"username": "` + testAdminUsername + `", "password": "` + testAdminPassword + `"}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, true, response["success"])
	require.NotEmpty(t, response["token"])
	require.Equal(t, "Bearer", response["token_type"])
}

func TestLogin_InvalidPassword(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, testAdminPassword)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"username": "admin", "password": "wrong-password"}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Contains(t, response["error"], "Invalid username or password")
}

func TestLogin_InvalidUsername(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, testAdminPassword)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"username": "wrong-user", "password": "` + testAdminPassword + `"}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_PasswordNotConfigured(t *testing.T) {
	router := setupLoginRouter(t, true, testAPIToken, testJWTSecret, 24, testAdminUsername, "")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"username": "admin", "password": "some-password"}`)
	req, _ := http.NewRequest("POST", pathAuthLogin, body)
	req.Header.Set(headerContentType, mimeJSON)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Contains(t, response["error"], "Password login is not configured")
}
