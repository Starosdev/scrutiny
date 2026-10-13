package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupHostsRouter(t *testing.T, repo *mock_database.MockDeviceRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Set("DEVICE_REPOSITORY", repo)
		c.Next()
	})
	router.GET("/hosts", GetHosts)
	router.POST("/hosts/archive", ArchiveHosts)
	router.POST("/hosts/unarchive", UnarchiveHosts)
	router.POST("/hosts/purge", PurgeHosts)
	return router
}

func TestGetHostsReturnsHostInventory(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetHosts(gomock.Any()).Return([]models.HostSummary{
		{HostID: "alpha", ActiveDevices: 2, ArchivedDevices: 1, TotalDevices: 3},
	}, nil)
	router := setupHostsRouter(t, repo)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/hosts", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"success":true,"data":[{"host_id":"alpha","active_devices":2,"archived_devices":1,"total_devices":3}]}`, response.Body.String())
}

func TestArchiveHostsDeduplicatesHostIDsAndReturnsPerHostResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{
		{DeviceID: "a", HostId: "alpha"},
	}, nil)
	repo.EXPECT().UpdateHostArchived(gomock.Any(), "alpha", true).Return(int64(1), nil)
	repo.EXPECT().UpdateHostArchived(gomock.Any(), "missing", true).Return(int64(0), nil)
	router := setupHostsRouter(t, repo)

	response := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"host_ids":["alpha","alpha","missing"]}`)
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/hosts/archive", body))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
		"success":false,
		"data":[
			{"host_id":"alpha","success":true,"device_count":1},
			{"host_id":"missing","success":false,"device_count":0,"error":"host not found"}
		]
	}`, response.Body.String())
}

func TestPurgeHostsRequiresTypedConfirmation(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	router := setupHostsRouter(t, repo)

	response := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"host_ids":["alpha"],"confirmation":"purge"}`)
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/hosts/purge", body))

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.JSONEq(t, `{"success":false,"error":"confirmation must equal PURGE"}`, response.Body.String())
}

func TestPurgeHostsReturnsRetryablePartialResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{
		{DeviceID: "a", HostId: "alpha"},
		{DeviceID: "b", HostId: "beta"},
	}, nil)
	repo.EXPECT().PurgeHosts(gomock.Any(), []string{"alpha", "beta"}).Return([]models.HostActionResult{
		{HostID: "alpha", Success: true, DeviceCount: 1},
		{HostID: "beta", DeviceCount: 1, Error: "storage unavailable"},
	}, nil)
	router := setupHostsRouter(t, repo)

	response := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"host_ids":["alpha","beta"],"confirmation":"PURGE"}`)
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/hosts/purge", body))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
		"success":false,
		"data":[
			{"host_id":"alpha","success":true,"device_count":1},
			{"host_id":"beta","success":false,"device_count":1,"error":"storage unavailable"}
		]
	}`, response.Body.String())
}

func TestNormalizeHostIDsRejectsEmptyValues(t *testing.T) {
	_, err := normalizeHostIDs([]string{"alpha", " "})
	require.EqualError(t, err, "host_ids cannot contain empty values")
}

func TestNormalizeHostIDsPreservesExactStoredIdentity(t *testing.T) {
	hostIDs, err := normalizeHostIDs([]string{" alpha "})
	require.NoError(t, err)
	require.Equal(t, []string{" alpha "}, hostIDs)
}
