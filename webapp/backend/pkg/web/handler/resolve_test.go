package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// fixes #851: a WWN held by several devices cannot identify one of them, so the legacy
// WWN route answers 409 instead of serving whichever device the database returns first.
func TestResolveDeviceRejectsSharedWWN(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const sharedWWN = "0x5000cca264eb01d7"
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetDeviceDetails(gomock.Any(), sharedWWN).Return(models.Device{}, fmt.Errorf("record not found"))
	repo.EXPECT().GetDeviceByWWN(gomock.Any(), sharedWWN).Return(models.Device{}, fmt.Errorf("%w: %s", database.ErrAmbiguousWWN, sharedWWN))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/device/"+sharedWWN+"/details", nil)
	c.Params = gin.Params{{Key: "id", Value: sharedWWN}}

	_, err := ResolveDevice(c, logrus.NewEntry(logrus.New()), repo)

	require.ErrorIs(t, err, database.ErrAmbiguousWWN)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "use the device_id")
}
