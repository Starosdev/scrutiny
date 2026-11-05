package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRegisterZFSPoolsRecordsCompleteInventoryIncludingEmptyReports(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().RegisterZFSPoolInventory(gomock.Any(), "host-a", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, pools []models.ZFSPool) error {
			require.Empty(t, pools)
			return nil
		},
	)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	router.POST("/api/zfs/pools/register", handler.RegisterZFSPools)

	body, err := json.Marshal(models.ZFSPoolWrapper{HostID: "host-a", Complete: true, Data: []models.ZFSPool{}})
	require.NoError(t, err)
	response := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/api/zfs/pools/register", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
}

func setupZFSUploadRouter(t *testing.T, configure func(*mock_database.MockDeviceRepo)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	configure(repo)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	return router
}

func postZFSMetrics(t *testing.T, router *gin.Engine, pool models.ZFSPool) int {
	t.Helper()
	body, _ := json.Marshal(pool)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/zfs/pool/"+pool.GUID+"/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w.Code
}

// A healthy upload with a healthy previous state never reaches the settings
// load, so the notification path costs nothing on the common case.
func TestUploadZFSPoolMetricsSkipsNotificationWhenHealthy(t *testing.T) {
	const guid = "1234567890"
	router := setupZFSUploadRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().GetZFSPoolDetails(gomock.Any(), guid).Return(models.ZFSPool{GUID: guid, Status: models.ZFSPoolStatusOnline}, nil)
		repo.EXPECT().RegisterZFSPool(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().SaveZFSPoolMetrics(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().LoadSettings(gomock.Any()).Times(0)
	})
	router.POST("/api/zfs/pool/:guid/metrics", handler.UploadZFSPoolMetrics)

	require.Equal(t, http.StatusOK, postZFSMetrics(t, router, models.ZFSPool{GUID: guid, Status: models.ZFSPoolStatusOnline}))
}

// The mute route writes a flag the upload payload never carries, so the
// stored row is the only place the handler can honor it.
func TestUploadZFSPoolMetricsSkipsNotificationForAMutedPool(t *testing.T) {
	const guid = "1234567890"
	router := setupZFSUploadRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().GetZFSPoolDetails(gomock.Any(), guid).Return(models.ZFSPool{GUID: guid, Status: models.ZFSPoolStatusOnline, Muted: true}, nil)
		repo.EXPECT().RegisterZFSPool(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().SaveZFSPoolMetrics(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().LoadSettings(gomock.Any()).Times(0)
	})
	router.POST("/api/zfs/pool/:guid/metrics", handler.UploadZFSPoolMetrics)

	require.Equal(t, http.StatusOK, postZFSMetrics(t, router, models.ZFSPool{GUID: guid, Status: models.ZFSPoolStatusDegraded}))
}

// A vdev query that fails returns the row with an error. Treating that as "no
// previous state" would count every existing error total as new, so the
// upload skips the check instead.
func TestUploadZFSPoolMetricsSkipsNotificationWhenPreviousStateIsUnreadable(t *testing.T) {
	const guid = "1234567890"
	router := setupZFSUploadRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().GetZFSPoolDetails(gomock.Any(), guid).Return(models.ZFSPool{GUID: guid, Status: models.ZFSPoolStatusOnline}, errors.New("vdev query failed"))
		repo.EXPECT().RegisterZFSPool(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().SaveZFSPoolMetrics(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().LoadSettings(gomock.Any()).Times(0)
	})
	router.POST("/api/zfs/pool/:guid/metrics", handler.UploadZFSPoolMetrics)

	require.Equal(t, http.StatusOK, postZFSMetrics(t, router, models.ZFSPool{GUID: guid, Status: models.ZFSPoolStatusOnline, TotalChecksumErrors: 4}))
}

// A first upload of an already degraded pool notifies: not-found means no
// previous state, not an unreadable one.
func TestUploadZFSPoolMetricsNotifiesOnFirstUploadOfADegradedPool(t *testing.T) {
	const guid = "1234567890"
	var deliveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deliveries.Add(1)
		var payload notify.Payload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, notify.NotifyFailureTypeZFSPoolDegraded, payload.FailureType)
		require.Equal(t, guid, payload.DeviceSerial)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg, err := config.Create()
	require.NoError(t, err)
	cfg.Set("notify.urls", []string{server.URL})
	settings := &models.Settings{}
	settings.ApplyDefaults()

	var calls []string
	router := setupZFSUploadRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().GetZFSPoolDetails(gomock.Any(), guid).DoAndReturn(func(context.Context, string) (models.ZFSPool, error) {
			calls = append(calls, "details")
			return models.ZFSPool{}, gorm.ErrRecordNotFound
		})
		repo.EXPECT().RegisterZFSPool(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, models.ZFSPool) error {
			calls = append(calls, "register")
			return nil
		})
		repo.EXPECT().SaveZFSPoolMetrics(gomock.Any(), gomock.Any()).Return(nil)
		repo.EXPECT().LoadSettings(gomock.Any()).Return(settings, nil)
		repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(nil, nil)
	})
	router.Use(func(c *gin.Context) {
		c.Set("CONFIG", cfg)
		c.Set("NOTIFICATION_GATE", notify.NewNotificationGate(logrus.New()))
		c.Next()
	})
	router.POST("/api/zfs/pool/:guid/metrics", handler.UploadZFSPoolMetrics)

	require.Equal(t, http.StatusOK, postZFSMetrics(t, router, models.ZFSPool{GUID: guid, Status: models.ZFSPoolStatusDegraded}))
	require.Equal(t, []string{"details", "register"}, calls)
	require.Equal(t, int32(1), deliveries.Load())
}
