package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestRegisterMdadmRoutesBindsEveryMutation binds the server's own route table,
// so a route deleted from it fails here. A handler test that declares its own
// routes cannot catch that, and an unbound route is the defect these routes
// were added for: the request fell through to the SPA handler and returned the
// frontend HTML with a 200.
func TestRegisterMdadmRoutesBindsEveryMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().UpdateMdadmArrayArchived(gomock.Any(), "md-1", true).Return(nil)
	repo.EXPECT().UpdateMdadmArrayArchived(gomock.Any(), "md-1", false).Return(nil)
	repo.EXPECT().UpdateMdadmArrayMuted(gomock.Any(), "md-1", true).Return(nil)
	repo.EXPECT().UpdateMdadmArrayMuted(gomock.Any(), "md-1", false).Return(nil)
	repo.EXPECT().UpdateMdadmArrayLabel(gomock.Any(), "md-1", "Media array").Return(nil)
	repo.EXPECT().DeleteMdadmArray(gomock.Any(), "md-1").Return(nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	registerMdadmRoutes(router.Group("/api/mdadm"))

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "archive", method: http.MethodPost, path: "/api/mdadm/array/md-1/archive"},
		{name: "unarchive", method: http.MethodPost, path: "/api/mdadm/array/md-1/unarchive"},
		{name: "mute", method: http.MethodPost, path: "/api/mdadm/array/md-1/mute"},
		{name: "unmute", method: http.MethodPost, path: "/api/mdadm/array/md-1/unmute"},
		{name: "label", method: http.MethodPost, path: "/api/mdadm/array/md-1/label", body: `{"label":"Media array"}`},
		{name: "delete", method: http.MethodDelete, path: "/api/mdadm/array/md-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request *http.Request
			if test.body == "" {
				request = httptest.NewRequest(test.method, test.path, nil)
			} else {
				request = httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				request.Header.Set("Content-Type", "application/json")
			}

			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code)
			require.JSONEq(t, `{"success":true}`, response.Body.String())
		})
	}
}
