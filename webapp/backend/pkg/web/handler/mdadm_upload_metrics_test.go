package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const mdadmUploadTestUUID = "ad005595:61305895:4de01d37:669bf162"

func postMdadmMetrics(t *testing.T, repo *mock_database.MockDeviceRepo, gate *notify.NotificationGate, body string) {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Set("NOTIFICATION_GATE", gate)
		c.Next()
	})
	router.POST("/api/mdadm/array/:uuid/metrics", UploadMdadmMetrics)

	request := httptest.NewRequest(http.MethodPost, "/api/mdadm/array/"+mdadmUploadTestUUID+"/metrics", bytes.NewReader([]byte(body)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
}

// seedAnnounced records the array as already announced in this degraded shape.
func seedAnnounced(gate *notify.NotificationGate, metrics collector.MDADMMetrics) {
	gate.MDADM().Evaluate(mdadmUploadTestUUID, &metrics, false, func() bool { return true })
}

// A muted array sends no notification. The mute route writes the flag and this
// handler is the only place that can honor it, so without this test the flag
// goes back to being inert the moment the early return moves.
func TestUploadMdadmMetricsSkipsNotificationForAMutedArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().SaveMdadmMetrics(gomock.Any(), mdadmUploadTestUUID, gomock.Any()).Return(nil)
	repo.EXPECT().GetMdadmArrayDetails(gomock.Any(), mdadmUploadTestUUID).Return(models.MDADMArray{UUID: mdadmUploadTestUUID, Muted: true}, nil)
	// A muted array must return before the notification settings are read.
	repo.EXPECT().LoadSettings(gomock.Any()).Times(0)

	postMdadmMetrics(t, repo, notify.NewNotificationGate(logrus.New()), `{"state":"degraded","failed_devices":1,"active_devices":1}`)
}

// The other direction of the same guard. Without this, replacing the muted
// check with an unconditional return leaves the suite green and silences every
// MDADM notification. The array was already announced in this shape and repeat
// notifications are off, so the handler stops before it needs any config.
func TestUploadMdadmMetricsEntersTheNotificationPathForAnUnmutedArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().SaveMdadmMetrics(gomock.Any(), mdadmUploadTestUUID, gomock.Any()).Return(nil)
	repo.EXPECT().GetMdadmArrayDetails(gomock.Any(), mdadmUploadTestUUID).Return(models.MDADMArray{UUID: mdadmUploadTestUUID, Muted: false}, nil)
	repo.EXPECT().LoadSettings(gomock.Any()).Times(1).Return(&models.Settings{}, nil)

	gate := notify.NewNotificationGate(logrus.New())
	seedAnnounced(gate, collector.MDADMMetrics{State: "degraded", FailedDevices: 1, ActiveDevices: 1})

	postMdadmMetrics(t, repo, gate, `{"state":"degraded","failed_devices":1,"active_devices":1}`)
}

// A healthy upload clears the record, so the array's next degradation is
// announced even when it has the same shape as the last one.
func TestUploadMdadmMetricsForgetsTheAnnouncementOnceHealthy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().SaveMdadmMetrics(gomock.Any(), mdadmUploadTestUUID, gomock.Any()).Return(nil)
	repo.EXPECT().GetMdadmArrayDetails(gomock.Any(), gomock.Any()).Times(0)

	gate := notify.NewNotificationGate(logrus.New())
	degradedMetrics := collector.MDADMMetrics{State: "degraded", FailedDevices: 1, ActiveDevices: 1}
	seedAnnounced(gate, degradedMetrics)

	postMdadmMetrics(t, repo, gate, `{"state":"clean","failed_devices":0,"active_devices":2}`)

	attempted := false
	gate.MDADM().Evaluate(mdadmUploadTestUUID, &degradedMetrics, false, func() bool { attempted = true; return true })
	require.True(t, attempted)
}

func TestUploadMdadmMetricsRejectsAnInvalidIdentifier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().SaveMdadmMetrics(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	router.POST("/api/mdadm/array/:uuid/metrics", UploadMdadmMetrics)

	request := httptest.NewRequest(http.MethodPost, `/api/mdadm/array/a%22/metrics`, bytes.NewReader([]byte(`{"state":"clean"}`)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

// The identifier reaches a Flux predicate through this route, so it is
// rejected before the repository builds one.
func TestGetMdadmArrayDetailsRejectsAnInvalidIdentifier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetMdadmArrayDetails(gomock.Any(), gomock.Any()).Times(0)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	router.GET("/api/mdadm/array/:uuid/details", GetMdadmArrayDetails)

	request := httptest.NewRequest(http.MethodGet, `/api/mdadm/array/x%22)%20or%20true%20or%20(r%5B%22array_uuid%22%5D%20==%20%22x/details`, nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}
