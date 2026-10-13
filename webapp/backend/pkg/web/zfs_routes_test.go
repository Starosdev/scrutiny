package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterZFSPoolRoutesDisablesEveryMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("AUTH_SCOPE", "admin") })
	registerZFSPoolRoutes(newRouteGroups(router.Group("")).sub("/api/zfs"), false)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "archive", method: http.MethodPost, path: "/api/zfs/pool/1/archive"},
		{name: "unarchive", method: http.MethodPost, path: "/api/zfs/pool/1/unarchive"},
		{name: "mute", method: http.MethodPost, path: "/api/zfs/pool/1/mute"},
		{name: "unmute", method: http.MethodPost, path: "/api/zfs/pool/1/unmute"},
		{name: "label", method: http.MethodPost, path: "/api/zfs/pool/1/label"},
		{name: "delete", method: http.MethodDelete, path: "/api/zfs/pool/1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request, err := http.NewRequest(test.method, test.path, nil)
			require.NoError(t, err)

			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusForbidden, response.Code)
		})
	}
}
