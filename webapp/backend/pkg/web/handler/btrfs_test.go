package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func setupBtrfsRouter(t *testing.T, configure func(*mock_database.MockDeviceRepo)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mock_database.NewMockDeviceRepo(ctrl)
	if configure != nil {
		configure(repo)
	}

	logger := logrus.WithField("test", t.Name())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logger)
		c.Next()
	})
	return r
}

func TestRegisterBtrfsFilesystems(t *testing.T) {
	filesystem := models.BtrfsFilesystem{UUID: "11111111-2222-3333-4444-555555555555"}
	router := setupBtrfsRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().RegisterBtrfsFilesystem(gomock.Any(), &filesystem).Return(nil)
	})
	router.POST("/api/btrfs/filesystems/register", handler.RegisterBtrfsFilesystems)

	body, _ := json.Marshal(models.BtrfsFilesystemWrapper{Data: []models.BtrfsFilesystem{filesystem}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/btrfs/filesystems/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetBtrfsFilesystemDetails(t *testing.T) {
	uuid := "11111111-2222-3333-4444-555555555555"
	filesystem := models.BtrfsFilesystem{UUID: uuid}
	router := setupBtrfsRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().GetBtrfsFilesystemDetails(gomock.Any(), uuid).Return(filesystem, nil)
		repo.EXPECT().GetBtrfsMetricsHistory(gomock.Any(), uuid, "week").Return([]measurements.BtrfsMetrics{}, nil)
	})
	router.GET("/api/btrfs/filesystem/:uuid/details", handler.GetBtrfsFilesystemDetails)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/btrfs/filesystem/"+uuid+"/details", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestUploadBtrfsMetricsPreservesDevices(t *testing.T) {
	uuid := "11111111-2222-3333-4444-555555555555"
	filesystem := models.BtrfsFilesystem{
		UUID:        uuid,
		HostID:      "zeus",
		Status:      models.BtrfsFilesystemStatusOnline,
		MountPoint:  "/mnt/cache_ssd",
		DeviceCount: 1,
		Devices: []models.BtrfsDevice{
			{DeviceID: 1, Path: "/dev/sdn1", Size: 4000785960960},
		},
	}

	var captured models.BtrfsFilesystem
	router := setupBtrfsRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().RegisterBtrfsFilesystem(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, fs *models.BtrfsFilesystem) error {
				captured = *fs
				return nil
			},
		)
		repo.EXPECT().SaveBtrfsMetrics(gomock.Any(), gomock.Any()).Return(nil)
	})
	router.POST("/api/btrfs/filesystem/:uuid/metrics", handler.UploadBtrfsMetrics)

	body, _ := json.Marshal(filesystem)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/btrfs/filesystem/"+uuid+"/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, captured.DeviceCount)
	require.Len(t, captured.Devices, 1)
	require.Equal(t, 1, captured.Devices[0].DeviceID)
	require.Equal(t, "/dev/sdn1", captured.Devices[0].Path)
}

func TestUpdateBtrfsFilesystemLabel(t *testing.T) {
	uuid := "11111111-2222-3333-4444-555555555555"
	router := setupBtrfsRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().UpdateBtrfsFilesystemLabel(gomock.Any(), uuid, "tank").Return(nil)
	})
	router.POST("/api/btrfs/filesystem/:uuid/label", handler.UpdateBtrfsFilesystemLabel)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/btrfs/filesystem/"+uuid+"/label", bytes.NewBufferString(`{"label":"tank"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteBtrfsFilesystemRejectsInvalidUUID(t *testing.T) {
	router := setupBtrfsRouter(t, nil)
	router.DELETE("/api/btrfs/filesystem/:uuid", handler.DeleteBtrfsFilesystem)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/btrfs/filesystem/not-a-uuid", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
