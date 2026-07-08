package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestRegisterMdadmArraysReturnsPartialSuccessWithErrorDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().RegisterMdadmArray(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().RegisterMdadmArray(gomock.Any(), gomock.Any()).Return(errors.New("duplicate key"))

	body := map[string]any{
		"data": []map[string]any{
			{"uuid": "uuid-1", "name": "md0", "level": "raid1", "devices": []string{"/dev/sda", "/dev/sdb"}},
			{"uuid": "uuid-2", "name": "md1", "level": "raid1", "devices": []string{"/dev/sdc", "/dev/sdd"}},
		},
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/mdadm/arrays/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("DEVICE_REPOSITORY", repo)
	c.Set("LOGGER", logrus.NewEntry(logrus.New()))

	RegisterMdadmArrays(c)

	require.Equal(t, http.StatusOK, w.Code)
	var response models.MDADMArrayWrapper
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Len(t, response.Data, 1)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], "uuid-2")
}

func TestRegisterMdadmArraysPassesHostIDThroughRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().RegisterMdadmArray(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, array models.MDADMArray) error {
		assert.Equal(t, "uuid-1", array.UUID)
		assert.Equal(t, "host-a", array.HostID)
		return nil
	})

	body := `{"data":[{"uuid":"uuid-1","name":"md0","level":"raid1","devices":["/dev/sda"],"host_id":"host-a"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mdadm/arrays/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("DEVICE_REPOSITORY", repo)
	c.Set("LOGGER", logrus.NewEntry(logrus.New()))

	RegisterMdadmArrays(c)

	require.Equal(t, http.StatusOK, w.Code)
	var response models.MDADMArrayWrapper
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.True(t, response.Success)
	require.Len(t, response.Data, 1)
	assert.Equal(t, "host-a", response.Data[0].HostID)
}

func TestRegisterMdadmArraysRejectsMissingUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)

	body := `{"data":[{"uuid":"","name":"md0","level":"raid1","devices":["/dev/sda"]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mdadm/arrays/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("DEVICE_REPOSITORY", repo)
	c.Set("LOGGER", logrus.NewEntry(logrus.New()))

	RegisterMdadmArrays(c)

	require.Equal(t, http.StatusOK, w.Code)
	var response models.MDADMArrayWrapper
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Empty(t, response.Data)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], "missing UUID")
}
