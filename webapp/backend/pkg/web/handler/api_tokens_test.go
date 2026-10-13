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

func getVersion(t *testing.T, scope string) map[string]any {
	t.Helper()
	ctrl := gomock.NewController(t)
	cfg := mock_config.NewMockInterface(ctrl)
	cfg.EXPECT().GetBool("web.auth.enabled").Return(true).AnyTimes()
	cfg.EXPECT().GetBool("web.zfs.allow_pool_modifications").Return(false).AnyTimes()
	cfg.EXPECT().GetBool("web.metrics.enabled").Return(true).AnyTimes()
	cfg.EXPECT().GetBool("web.mqtt.enabled").Return(false).AnyTimes()
	r := setupTokensRouter(t, mock_database.NewMockDeviceRepo(ctrl), cfg)
	if scope != "" {
		r = gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("CONFIG", cfg)
			c.Set("AUTH_SCOPE", scope)
		})
		r.GET("/api/version", handler.GetVersion)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/version", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestGetVersion_AnonymousGetsMinimalFields(t *testing.T) {
	resp := getVersion(t, "")
	require.Equal(t, handler.APIVersion, resp["api_version"])
	require.NotEmpty(t, resp["version"])
	require.Equal(t, true, resp["auth_required"])
	require.NotContains(t, resp, "capabilities")
}

func TestGetVersion_AuthenticatedGetsCapabilities(t *testing.T) {
	for _, scope := range []string{models.APITokenScopeRead, models.APITokenScopeCollector, models.APITokenScopeAdmin} {
		resp := getVersion(t, scope)
		caps, ok := resp["capabilities"].(map[string]any)
		require.True(t, ok, "scope %s should see capabilities", scope)
		require.Equal(t, true, caps["scoped_tokens"])
		require.Equal(t, true, caps["metrics_enabled"])
		require.Equal(t, false, caps["mqtt_enabled"])
		require.Contains(t, caps, "collector_trigger_enabled")
		require.Contains(t, caps, "zfs_pool_modifications_allowed")
	}
}

func TestCreateAPIToken_ExpiryStoredInUTC(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	var stored *models.ApiToken
	repo.EXPECT().CreateAPIToken(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, tok *models.ApiToken) error {
		stored = tok
		return nil
	})
	r := setupTokensRouter(t, repo, mock_config.NewMockInterface(ctrl))
	exp := time.Now().Add(48 * time.Hour).In(time.FixedZone("x", 5*3600)).Format(time.RFC3339)
	require.Equal(t, http.StatusCreated, postJSON(r, tokensPath, `{"name":"x","scope":"read","expires_at":"`+exp+`"}`).Code)
	require.Equal(t, time.UTC, stored.ExpiresAt.Location())
}

func TestGetSettings_RedactsSecretsBelowAdmin(t *testing.T) {
	const pushURL = "https://kuma.example/api/push/SECRETTOKEN?status=up"
	for scope, wantSecret := range map[string]bool{
		models.APITokenScopeRead:      false,
		models.APITokenScopeCollector: false,
		"":                            false,
		models.APITokenScopeAdmin:     true,
	} {
		ctrl := gomock.NewController(t)
		repo := mock_database.NewMockDeviceRepo(ctrl)
		settings := &models.Settings{}
		settings.Metrics.UptimeKumaPushURL = pushURL
		repo.EXPECT().LoadSettings(gomock.Any()).Return(settings, nil)
		cfg := mock_config.NewMockInterface(ctrl)
		cfg.EXPECT().GetBool(gomock.Any()).Return(false).AnyTimes()

		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("LOGGER", logrus.WithField("test", t.Name()))
			c.Set("DEVICE_REPOSITORY", repo)
			c.Set("CONFIG", cfg)
			if scope != "" {
				c.Set("AUTH_SCOPE", scope)
			}
		})
		r.GET("/api/settings", handler.GetSettings)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/settings", nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, wantSecret, bytes.Contains(w.Body.Bytes(), []byte("SECRETTOKEN")), "scope %q", scope)
		require.Equal(t, pushURL, settings.Metrics.UptimeKumaPushURL, "redaction must not mutate the loaded settings")
	}
}
