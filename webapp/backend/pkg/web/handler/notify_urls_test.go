package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func setupNotifyUrlsRouter(t *testing.T, mockRepo *mock_database.MockDeviceRepo, mockCfg *mock_config.MockInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("LOGGER", logger)
		c.Set("DEVICE_REPOSITORY", mockRepo)
		c.Set("CONFIG", mockCfg)
		c.Next()
	})
	r.GET("/api/settings/notify-urls", handler.GetNotifyUrls)
	r.POST("/api/settings/notify-urls", handler.SaveNotifyUrl)
	r.PATCH("/api/settings/notify-urls/:id", handler.UpdateNotifyUrlHeartbeat)
	return r
}

func TestGetNotifyUrls_IncludesHeartbeatEnabled(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockCfg := mock_config.NewMockInterface(mockCtrl)

	mockCfg.EXPECT().GetStringSlice("notify.urls").Return([]string{})
	mockRepo.EXPECT().GetNotifyUrls(gomock.Any()).Return([]models.NotifyUrl{
		{ID: 1, URL: "slack://token", Label: "Slack", Source: "ui", HeartbeatEnabled: false},
	}, nil)

	router := setupNotifyUrlsRouter(t, mockRepo, mockCfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings/notify-urls", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			HeartbeatEnabled bool `json:"heartbeat_enabled"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	require.False(t, resp.Data[0].HeartbeatEnabled)
}

func TestSaveNotifyUrl_PersistsHeartbeatEnabled(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockCfg := mock_config.NewMockInterface(mockCtrl)

	mockRepo.EXPECT().SaveNotifyUrl(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ interface{}, u *models.NotifyUrl) error {
			require.True(t, u.HeartbeatEnabled)
			u.ID = 42
			return nil
		},
	)

	router := setupNotifyUrlsRouter(t, mockRepo, mockCfg)

	body, _ := json.Marshal(map[string]interface{}{
		"url":               "generic://healthchecks.io/ping/abc",
		"label":             "HC",
		"heartbeat_enabled": true,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/notify-urls", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateNotifyUrlHeartbeat_Success(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockCfg := mock_config.NewMockInterface(mockCtrl)

	mockRepo.EXPECT().UpdateNotifyUrlHeartbeat(gomock.Any(), uint(1), false).Return(nil)

	router := setupNotifyUrlsRouter(t, mockRepo, mockCfg)

	body, _ := json.Marshal(map[string]bool{"enabled": false})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/settings/notify-urls/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, true, resp["success"])
}
