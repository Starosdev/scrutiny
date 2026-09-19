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
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
