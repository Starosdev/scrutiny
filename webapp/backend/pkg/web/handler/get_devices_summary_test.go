package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
