package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupSummaryRouter(t *testing.T, repo *mock_database.MockDeviceRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.WithField("test", t.Name())
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("LOGGER", logger)
		c.Set("DEVICE_REPOSITORY", repo)
		c.Next()
	})
	router.GET("/api/summary", handler.GetDevicesSummary)
	return router
}

func TestGetDevicesSummaryPreservesLegacyUnpaginatedRequest(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	summary := map[string]*models.DeviceSummary{
		"device-1": {Device: models.Device{DeviceID: "device-1"}},
	}
	repo.EXPECT().GetSummary(gomock.Any()).Return(summary, nil)
	repo.EXPECT().LoadSettings(gomock.Any()).Return(&models.Settings{}, nil).AnyTimes()
	repo.EXPECT().GetDevicesLastSeenTimes(gomock.Any()).Return(map[string]time.Time{}, nil).AnyTimes()

	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/api/summary", nil)
	setupSummaryRouter(t, repo).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), `"pagination"`)
}

func TestGetDevicesSummaryReturnsPaginatedResponse(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	options := models.DeviceSummaryPageOptions{
		Page:        2,
		PageSize:    10,
		Archived:    true,
		Sort:        "title_asc",
		Display:     "label",
		HostSearch:  "alpha",
		GroupByHost: true,
	}
	page := &models.DeviceSummaryPage{
		Summary: map[string]*models.DeviceSummary{
			"device-1": {Device: models.Device{DeviceID: "device-1"}},
		},
		Pagination: models.PaginationMetadata{
			Page:           2,
			PageSize:       10,
			TotalItems:     75,
			TotalPages:     2,
			AttentionCount: 3,
		},
	}
	repo.EXPECT().GetSummaryPage(gomock.Any(), options).Return(page, nil)
	repo.EXPECT().LoadSettings(gomock.Any()).Return(&models.Settings{}, nil).AnyTimes()
	repo.EXPECT().GetDevicesLastSeenTimes(gomock.Any()).Return(map[string]time.Time{}, nil).AnyTimes()

	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/api/summary?page=2&page_size=10&group_by=host&archived=true&sort=title_asc&display=label&host=alpha", nil)
	setupSummaryRouter(t, repo).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var payload models.DeviceSummaryWrapper
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.NotNil(t, payload.Data.Pagination)
	require.Equal(t, 75, payload.Data.Pagination.TotalItems)
	require.Equal(t, 3, payload.Data.Pagination.AttentionCount)
}

func TestGetDevicesSummaryRejectsInvalidPagination(t *testing.T) {
	testCases := []string{
		"/api/summary?page=0",
		"/api/summary?page=one",
		"/api/summary?page=1&page_size=10",
		"/api/summary?page=1&page_size=100&group_by=host",
		"/api/summary?page=1&group_by=unknown",
		"/api/summary?page=1&archived=maybe",
		"/api/summary?page=1&sort=unknown",
		"/api/summary?page=1&display=unknown",
	}

	for _, path := range testCases {
		t.Run(path, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)
			repo := mock_database.NewMockDeviceRepo(mockCtrl)

			response := httptest.NewRecorder()
			request, _ := http.NewRequest(http.MethodGet, path, nil)
			setupSummaryRouter(t, repo).ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Contains(t, response.Body.String(), `"success":false`)
		})
	}
}

func TestGetDevicesSummaryFlagsStaleDevices(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)

	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-5 * time.Minute)
	summary := map[string]*models.DeviceSummary{
		"silent":    {Device: models.Device{DeviceID: "silent"}},
		"fresh":     {Device: models.Device{DeviceID: "fresh"}},
		"override":  {Device: models.Device{DeviceID: "override", MissedPingTimeoutOverride: 4320}},
		"archived":  {Device: models.Device{DeviceID: "archived", Archived: true}},
		"neverseen": {Device: models.Device{DeviceID: "neverseen"}},
	}
	repo.EXPECT().GetSummary(gomock.Any()).Return(summary, nil)
	settings := &models.Settings{}
	settings.Metrics.MissedPingTimeoutMinutes = 60
	repo.EXPECT().LoadSettings(gomock.Any()).Return(settings, nil)
	repo.EXPECT().GetDevicesLastSeenTimes(gomock.Any()).Return(map[string]time.Time{
		"silent": old, "fresh": recent, "override": old, "archived": old,
	}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/summary", nil)
	setupSummaryRouter(t, repo).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data struct {
			Summary map[string]struct {
				LastSeen *time.Time `json:"last_seen"`
				Stale    bool       `json:"stale"`
			} `json:"summary"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	got := body.Data.Summary
	require.True(t, got["silent"].Stale)
	require.NotNil(t, got["silent"].LastSeen)
	require.False(t, got["fresh"].Stale)
	require.False(t, got["override"].Stale, "per-device 3 day override must win over the 60 minute global")
	require.False(t, got["archived"].Stale)
	require.False(t, got["neverseen"].Stale)
	require.Nil(t, got["neverseen"].LastSeen)
}
