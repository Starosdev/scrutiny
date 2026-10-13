package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/middleware"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

const (
	pathScopeRead      = "/api/summary"
	pathScopeCollector = "/api/devices/register"
	pathScopeAdmin     = "/api/settings"
	scopedPlaintext    = "scr_plaintext-token-for-tests"
)

func setupScopeRouter(t *testing.T, repo *mock_database.MockDeviceRepo, authEnabled bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	cfg := mock_config.NewMockInterface(ctrl)
	cfg.EXPECT().GetBool("web.auth.enabled").Return(authEnabled).AnyTimes()
	cfg.EXPECT().GetBool("web.docs.public").Return(false).AnyTimes()
	cfg.EXPECT().GetString("web.auth.token").Return(testAPIToken).AnyTimes()
	cfg.EXPECT().GetString("web.auth.jwt_secret").Return(testJWTSecret).AnyTimes()
	cfg.EXPECT().GetString("web.metrics.token").Return("").AnyTimes()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Next()
	})
	r.Use(middleware.AuthMiddleware(cfg, logrus.WithField("test", t.Name())))

	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) }
	r.Group("", middleware.RequireScope(models.APITokenScopeRead)).GET(pathScopeRead, ok)
	r.Group("", middleware.RequireScope(models.APITokenScopeCollector)).POST(pathScopeCollector, ok)
	r.Group("", middleware.RequireScope(models.APITokenScopeAdmin)).GET(pathScopeAdmin, ok)
	r.Group("", middleware.RequireScope(models.APITokenScopeAdmin)).POST(pathScopeAdmin, ok)
	return r
}

func doScoped(r *gin.Engine, method, path, bearer string) int {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", bearerPrefix+bearer)
	}
	r.ServeHTTP(w, req)
	return w.Code
}

// expectToken makes the repository return a token with the given scope for scopedPlaintext.
func expectToken(repo *mock_database.MockDeviceRepo, scope string) {
	repo.EXPECT().GetActiveAPITokenByHash(gomock.Any(), auth.HashToken(scopedPlaintext)).
		Return(&models.ApiToken{ID: 7, Scope: scope, TokenHash: auth.HashToken(scopedPlaintext)}, nil).AnyTimes()
	repo.EXPECT().TouchAPIToken(gomock.Any(), uint(7)).Return(nil).AnyTimes()
}

func TestScopedToken_Matrix(t *testing.T) {
	tests := []struct {
		name   string
		scope  string
		method string
		path   string
		want   int
	}{
		{"read reads", models.APITokenScopeRead, "GET", pathScopeRead, http.StatusOK},
		{"read cannot upload", models.APITokenScopeRead, "POST", pathScopeCollector, http.StatusForbidden},
		{"read cannot read admin route", models.APITokenScopeRead, "GET", pathScopeAdmin, http.StatusForbidden},
		{"read cannot POST settings", models.APITokenScopeRead, "POST", pathScopeAdmin, http.StatusForbidden},
		{"collector uploads", models.APITokenScopeCollector, "POST", pathScopeCollector, http.StatusOK},
		{"collector cannot read", models.APITokenScopeCollector, "GET", pathScopeRead, http.StatusForbidden},
		{"collector cannot read settings", models.APITokenScopeCollector, "GET", pathScopeAdmin, http.StatusForbidden},
		{"admin reads", models.APITokenScopeAdmin, "GET", pathScopeRead, http.StatusOK},
		{"admin uploads", models.APITokenScopeAdmin, "POST", pathScopeCollector, http.StatusOK},
		{"admin writes settings", models.APITokenScopeAdmin, "POST", pathScopeAdmin, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mock_database.NewMockDeviceRepo(ctrl)
			expectToken(repo, tt.scope)
			r := setupScopeRouter(t, repo, true)
			require.Equal(t, tt.want, doScoped(r, tt.method, tt.path, scopedPlaintext))
		})
	}
}

func TestScopedToken_UnknownRevokedOrExpiredIsUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	// The repository filters revoked and expired tokens, so all three look like a miss here.
	repo.EXPECT().GetActiveAPITokenByHash(gomock.Any(), gomock.Any()).Return(nil, gorm.ErrRecordNotFound).AnyTimes()
	r := setupScopeRouter(t, repo, true)

	require.Equal(t, http.StatusUnauthorized, doScoped(r, "GET", pathScopeRead, scopedPlaintext))
	require.Equal(t, http.StatusUnauthorized, doScoped(r, "GET", pathScopeRead, ""))
}

func TestScopedToken_LegacyScopeIsRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	expectToken(repo, "full") // the column default; not a valid scope
	r := setupScopeRouter(t, repo, true)
	require.Equal(t, http.StatusUnauthorized, doScoped(r, "GET", pathScopeRead, scopedPlaintext))
}

func TestMasterTokenAndJWTAreAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl) // no calls expected: master/JWT never hit the table
	r := setupScopeRouter(t, repo, true)

	require.Equal(t, http.StatusOK, doScoped(r, "POST", pathScopeAdmin, testAPIToken))
	require.Equal(t, http.StatusOK, doScoped(r, "POST", pathScopeCollector, testAPIToken))

	jwtToken, _, err := auth.GenerateJWT(testJWTSecret, 1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, doScoped(r, "POST", pathScopeAdmin, jwtToken))
}

func TestAuthDisabledPassesScopeChecks(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	r := setupScopeRouter(t, repo, false)
	require.Equal(t, http.StatusOK, doScoped(r, "POST", pathScopeAdmin, ""))
}

func TestRequireScopeWithoutAuthMiddlewareFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Group("", middleware.RequireScope(models.APITokenScopeRead)).GET(pathScopeRead, func(c *gin.Context) { c.Status(http.StatusOK) })
	require.Equal(t, http.StatusForbidden, doScoped(r, "GET", pathScopeRead, ""))
}

func TestScopedToken_TouchedOnUse(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetActiveAPITokenByHash(gomock.Any(), auth.HashToken(scopedPlaintext)).
		Return(&models.ApiToken{ID: 7, Scope: models.APITokenScopeRead, TokenHash: auth.HashToken(scopedPlaintext), ExpiresAt: ptrTime(time.Now().Add(time.Hour))}, nil)
	repo.EXPECT().TouchAPIToken(gomock.Any(), uint(7)).Return(nil).Times(1)
	r := setupScopeRouter(t, repo, true)
	require.Equal(t, http.StatusOK, doScoped(r, "GET", pathScopeRead, scopedPlaintext))
}

func ptrTime(t time.Time) *time.Time { return &t }
