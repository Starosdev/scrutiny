package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

// The health check log names the configured relational database (#880).
func TestHealthCheck_LogsConfiguredDatabase(t *testing.T) {
	for dbType, want := range map[string]string{
		"postgres": "Checking InfluxDB & postgres health",
		"sqlite":   "Checking InfluxDB & sqlite health",
		"":         "Checking InfluxDB & sqlite health",
	} {
		t.Run(want+"/"+dbType, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cfg := mock_config.NewMockInterface(ctrl)
			cfg.EXPECT().GetString("web.database.type").Return(dbType).AnyTimes()
			cfg.EXPECT().GetString("web.src.frontend.path").Return(t.TempDir()).AnyTimes()
			repo := mock_database.NewMockDeviceRepo(ctrl)
			repo.EXPECT().HealthCheck(gomock.Any()).Return(&database.HealthCheckResult{
				Status: "healthy",
				Checks: map[string]database.HealthCheckStatus{},
			}, nil)
			logger, hook := logrustest.NewNullLogger()

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("CONFIG", cfg)
				c.Set("LOGGER", logger.WithField("test", t.Name()))
				c.Set("DEVICE_REPOSITORY", repo)
				c.Next()
			})
			r.GET("/api/health", handler.HealthCheck)
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/health", nil))

			var messages []string
			for _, e := range hook.AllEntries() {
				messages = append(messages, e.Message)
			}
			require.Contains(t, messages, want)
		})
	}
}
