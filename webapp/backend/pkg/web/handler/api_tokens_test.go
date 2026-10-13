package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

const tokensPath = "/api/settings/tokens"

func setupTokensRouter(t *testing.T, repo *mock_database.MockDeviceRepo, cfg *mock_config.MockInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("CONFIG", cfg)
	})
	r.GET(tokensPath, handler.ListAPITokens)
	r.POST(tokensPath, handler.CreateAPIToken)
	r.DELETE(tokensPath+"/:id", handler.RevokeAPIToken)
	r.GET("/api/version", handler.GetVersion)
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestCreateAPIToken_ReturnsPlaintextOnceAndStoresOnlyHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	var stored *models.ApiToken
	repo.EXPECT().CreateAPIToken(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, tok *models.ApiToken) error {
		tok.ID = 3
		tok.CreatedAt = time.Now()
		stored = tok
		return nil
	})
	r := setupTokensRouter(t, repo, mock_config.NewMockInterface(ctrl))

	w := postJSON(r, tokensPath, `{"name":"  phone  ","scope":"read"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	var resp struct {
		Token    string         `json:"token"`
		APIToken map[string]any `json:"api_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Contains(t, resp.Token, "scr_")
	require.NotNil(t, stored)
	require.Equal(t, "phone", stored.Name)
	require.Equal(t, auth.HashToken(resp.Token), stored.TokenHash)
	require.NotContains(t, stored.TokenHash, resp.Token)
	require.NotContains(t, resp.APIToken, "token")
	require.NotContains(t, resp.APIToken, "token_hash")
	require.NotContains(t, w.Body.String(), stored.TokenHash, "the hash must not be returned either")
}

func TestListAPITokens_NeverReturnsTokenMaterial(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().ListAPITokens(gomock.Any()).Return([]models.ApiToken{
		{ID: 1, Name: "phone", Scope: models.APITokenScopeRead, TokenHash: "secret-hash-value"},
	}, nil)
	r := setupTokensRouter(t, repo, mock_config.NewMockInterface(ctrl))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", tokensPath, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "secret-hash-value")
	require.NotContains(t, w.Body.String(), "token_hash")
	require.Contains(t, w.Body.String(), `"name":"phone"`)
}

func TestCreateAPIToken_Validation(t *testing.T) {
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	bodies := map[string]string{
		"missing name":  `{"scope":"read"}`,
		"blank name":    `{"name":"   ","scope":"read"}`,
		"bad scope":     `{"name":"x","scope":"full"}`,
		"missing scope": `{"name":"x"}`,
		"past expiry":   `{"name":"x","scope":"read","expires_at":"` + past + `"}`,
		"malformed":     `{`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mock_database.NewMockDeviceRepo(ctrl) // CreateAPIToken must not be called
			r := setupTokensRouter(t, repo, mock_config.NewMockInterface(ctrl))
			require.Equal(t, http.StatusBadRequest, postJSON(r, tokensPath, body).Code)
		})
	}
}

func TestRevokeAPIToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().RevokeAPIToken(gomock.Any(), uint(4)).Return(nil)
	repo.EXPECT().RevokeAPIToken(gomock.Any(), uint(5)).Return(gorm.ErrRecordNotFound)
	r := setupTokensRouter(t, repo, mock_config.NewMockInterface(ctrl))

	del := func(id string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", tokensPath+"/"+id, nil)
		r.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, http.StatusOK, del("4"))
	require.Equal(t, http.StatusNotFound, del("5"))
	require.Equal(t, http.StatusBadRequest, del("abc"))
}

func TestGetVersion_Shape(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := mock_config.NewMockInterface(ctrl)
	cfg.EXPECT().GetBool("web.auth.enabled").Return(true)
	cfg.EXPECT().GetBool("web.zfs.allow_pool_modifications").Return(false).AnyTimes()
	cfg.EXPECT().GetBool("web.metrics.enabled").Return(true)
	cfg.EXPECT().GetBool("web.mqtt.enabled").Return(false)
	r := setupTokensRouter(t, mock_database.NewMockDeviceRepo(ctrl), cfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/version", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success      bool            `json:"success"`
		Version      string          `json:"version"`
		APIVersion   string          `json:"api_version"`
		Capabilities map[string]bool `json:"capabilities"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.NotEmpty(t, resp.Version)
	require.Equal(t, handler.APIVersion, resp.APIVersion)
	require.Equal(t, true, resp.Capabilities["auth_required"])
	require.Equal(t, true, resp.Capabilities["scoped_tokens"])
	require.Equal(t, true, resp.Capabilities["metrics_enabled"])
	require.Equal(t, false, resp.Capabilities["mqtt_enabled"])
	require.Contains(t, resp.Capabilities, "collector_trigger_enabled")
	require.Contains(t, resp.Capabilities, "zfs_pool_modifications_allowed")
}
