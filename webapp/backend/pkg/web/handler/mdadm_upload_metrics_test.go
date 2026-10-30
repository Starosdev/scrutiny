package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// A muted array sends no notification. The mute route writes the flag and this
// handler is the only place that can honor it, so without this test the flag
// goes back to being inert the moment the early return moves.
func TestUploadMdadmMetricsSkipsNotificationForAMutedArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const uuid = "ad005595:61305895:4de01d37:669bf162"
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().SaveMdadmMetrics(gomock.Any(), uuid, gomock.Any()).Return(nil)
	repo.EXPECT().GetMdadmArrayDetails(gomock.Any(), uuid).Return(models.MDADMArray{UUID: uuid, Muted: true}, nil)
	// The notification path reads metrics history to decide whether the state
	// changed. A muted array must return before that.
	repo.EXPECT().GetMdadmMetricsHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	router.POST("/api/mdadm/array/:uuid/metrics", UploadMdadmMetrics)

	// A degraded array with a failed device is what would otherwise notify.
	body := `{"state":"degraded","failed_devices":1,"active_devices":1,"total_devices":2}`
	request := httptest.NewRequest(http.MethodPost, "/api/mdadm/array/"+uuid+"/metrics", bytes.NewReader([]byte(body)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
}

// The other direction of the same guard. Without this, replacing the muted
// check with an unconditional return leaves the suite green and silences every
// MDADM notification.
func TestUploadMdadmMetricsEntersTheNotificationPathForAnUnmutedArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const uuid = "ad005595:61305895:4de01d37:669bf162"
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().SaveMdadmMetrics(gomock.Any(), uuid, gomock.Any()).Return(nil)
	repo.EXPECT().GetMdadmArrayDetails(gomock.Any(), uuid).Return(models.MDADMArray{UUID: uuid, Muted: false}, nil)
	// Reached only past the mute check. The history says the array was already
	// degraded at the previous collection, so the notification is suppressed as
	// a repeat and the handler returns before it needs any config.
	repo.EXPECT().GetMdadmMetricsHistory(gomock.Any(), uuid, "day").Times(1).Return([]measurements.MDADMMetrics{
		{ArrayUUID: uuid, State: "degraded", FailedDevices: 1},
		{ArrayUUID: uuid, State: "degraded", FailedDevices: 1},
	}, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	router.POST("/api/mdadm/array/:uuid/metrics", UploadMdadmMetrics)

	body := `{"state":"degraded","failed_devices":1,"active_devices":1}`
	request := httptest.NewRequest(http.MethodPost, "/api/mdadm/array/"+uuid+"/metrics", bytes.NewReader([]byte(body)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
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
