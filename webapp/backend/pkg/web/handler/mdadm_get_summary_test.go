package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMdadmSummaryNormalizesNullDevices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetMdadmArrays(gomock.Any()).Return([]models.MDADMArray{{
		UUID:  "uuid-1",
		Name:  "md0",
		Level: "raid1",
	}}, nil)
	repo.EXPECT().GetLatestMdadmMetrics(gomock.Any(), "uuid-1").Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mdadm/summary", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("DEVICE_REPOSITORY", repo)
	c.Set("LOGGER", logrus.NewEntry(logrus.New()))

	GetMdadmSummary(c)

	require.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Data []struct {
			Devices []string `json:"devices"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	assert.NotNil(t, response.Data[0].Devices)
	assert.Empty(t, response.Data[0].Devices)
}
