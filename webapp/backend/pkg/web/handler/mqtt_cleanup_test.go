package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMqttSyncUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "disconnected"}[enabled], func(t *testing.T) {
			logger := logrus.NewEntry(logrus.New())
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/health/mqtt-sync", nil)
			c.Set("LOGGER", logger)
			c.Set("DEVICE_REPOSITORY", mock_database.NewMockDeviceRepo(gomock.NewController(t)))
			want := http.StatusBadRequest
			if enabled {
				cfg, err := config.Create()
				require.NoError(t, err)
				c.Set("MQTT_PUBLISHER", mqtt.NewPublisher(cfg, logger))
				want = http.StatusServiceUnavailable
			}
			MqttSync(c)
			require.Equal(t, want, w.Code)
			require.Contains(t, w.Body.String(), `"success":false`)
		})
	}
}
