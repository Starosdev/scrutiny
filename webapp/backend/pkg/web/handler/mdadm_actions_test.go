package handler

import (
	"bytes"
	"errors"
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

// mdadmActionRouter binds the handlers under test with the same path parameter
// the server uses. The server's own route table is bound and exercised by
// TestRegisterMdadmRoutesBindsEveryMutation in the web package; this helper
// covers the handler behavior that does not depend on it.
func mdadmActionRouter(t *testing.T, configure func(*mock_database.MockDeviceRepo)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mock_database.NewMockDeviceRepo(ctrl)
	if configure != nil {
		configure(repo)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	mdadm := router.Group("/api/mdadm")
	mdadm.POST("/array/:uuid/archive", ArchiveMdadmArray)
	mdadm.POST("/array/:uuid/unarchive", UnarchiveMdadmArray)
	mdadm.POST("/array/:uuid/mute", MuteMdadmArray)
	mdadm.POST("/array/:uuid/unmute", UnmuteMdadmArray)
	mdadm.POST("/array/:uuid/label", UpdateMdadmArrayLabel)
	mdadm.DELETE("/array/:uuid", DeleteMdadmArray)
	return router
}

func mdadmRequest(t *testing.T, router *gin.Engine, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestMdadmArchiveRoutes(t *testing.T) {
	// The identifier is a synthetic one, which is what an array with no UUID
	// of its own is keyed by, and is not an RFC 4122 UUID.
	const uuid = "synthetic:4b825dc642cb6eb9a060e54bf8d69288fbee4904"

	tests := []struct {
		name     string
		path     string
		archived bool
	}{
		{name: "archive", path: "/api/mdadm/array/" + uuid + "/archive", archived: true},
		{name: "unarchive", path: "/api/mdadm/array/" + uuid + "/unarchive", archived: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := mdadmActionRouter(t, func(repo *mock_database.MockDeviceRepo) {
				repo.EXPECT().UpdateMdadmArrayArchived(gomock.Any(), uuid, test.archived).Return(nil)
			})

			response := mdadmRequest(t, router, http.MethodPost, test.path, "")

			require.Equal(t, http.StatusOK, response.Code)
			require.JSONEq(t, `{"success":true}`, response.Body.String())
		})
	}
}

func TestMdadmMuteRoutes(t *testing.T) {
	const uuid = "9a1b2c3d:4e5f6071:8293a4b5:c6d7e8f9"

	tests := []struct {
		name  string
		path  string
		muted bool
	}{
		{name: "mute", path: "/api/mdadm/array/" + uuid + "/mute", muted: true},
		{name: "unmute", path: "/api/mdadm/array/" + uuid + "/unmute", muted: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := mdadmActionRouter(t, func(repo *mock_database.MockDeviceRepo) {
				repo.EXPECT().UpdateMdadmArrayMuted(gomock.Any(), uuid, test.muted).Return(nil)
			})

			response := mdadmRequest(t, router, http.MethodPost, test.path, "")

			require.Equal(t, http.StatusOK, response.Code)
			require.JSONEq(t, `{"success":true}`, response.Body.String())
		})
	}
}

func TestMdadmLabelRoute(t *testing.T) {
	const uuid = "uuid-1"
	router := mdadmActionRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().UpdateMdadmArrayLabel(gomock.Any(), uuid, "Media array").Return(nil)
	})

	response := mdadmRequest(t, router, http.MethodPost, "/api/mdadm/array/"+uuid+"/label", `{"label":"Media array"}`)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"success":true}`, response.Body.String())
}

func TestMdadmLabelRouteRejectsAMalformedPayload(t *testing.T) {
	router := mdadmActionRouter(t, nil)

	response := mdadmRequest(t, router, http.MethodPost, "/api/mdadm/array/uuid-1/label", `{"label":`)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestMdadmDeleteRoute(t *testing.T) {
	const uuid = "codex-issue-640-1783489348"
	router := mdadmActionRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().DeleteMdadmArray(gomock.Any(), uuid).Return(nil)
	})

	response := mdadmRequest(t, router, http.MethodDelete, "/api/mdadm/array/"+uuid, "")

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"success":true}`, response.Body.String())
}

// Deleting an array that is not registered is not an error. The repository
// deletes by identifier and removes nothing, and a caller clearing a stale
// array should not have to care whether it was already gone.
func TestMdadmDeleteRouteIsIdempotentForAnUnknownArray(t *testing.T) {
	const uuid = "never-registered"
	router := mdadmActionRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().DeleteMdadmArray(gomock.Any(), uuid).Return(nil)
	})

	response := mdadmRequest(t, router, http.MethodDelete, "/api/mdadm/array/"+uuid, "")

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"success":true}`, response.Body.String())
}

func TestMdadmDeleteRouteReportsARepositoryFailure(t *testing.T) {
	router := mdadmActionRouter(t, func(repo *mock_database.MockDeviceRepo) {
		repo.EXPECT().DeleteMdadmArray(gomock.Any(), "uuid-1").Return(errors.New("influx unreachable"))
	})

	response := mdadmRequest(t, router, http.MethodDelete, "/api/mdadm/array/uuid-1", "")

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"success":false}`, response.Body.String())
}

func TestMdadmRoutesRejectAnOversizedIdentifier(t *testing.T) {
	router := mdadmActionRouter(t, nil)

	response := mdadmRequest(t, router, http.MethodDelete, "/api/mdadm/array/"+strings.Repeat("a", 257), "")

	require.Equal(t, http.StatusBadRequest, response.Code)
}

// An identifier carrying a quote would end the InfluxDB delete predicate the
// repository builds, so it never reaches it.
func TestMdadmRoutesRejectAnIdentifierOutsideTheAllowedCharacters(t *testing.T) {
	for _, identifier := range []string{`a%22`, `a%5C`, `a%2Fb`, `a%0Ab`} {
		t.Run(identifier, func(t *testing.T) {
			router := mdadmActionRouter(t, nil)

			response := mdadmRequest(t, router, http.MethodDelete, "/api/mdadm/array/"+identifier, "")

			require.NotEqual(t, http.StatusOK, response.Code)
		})
	}
}

func TestMdadmStateRoutesReportARepositoryFailure(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		configure func(*mock_database.MockDeviceRepo)
	}{
		{
			name: "archive",
			path: "/api/mdadm/array/md-1/archive",
			configure: func(repo *mock_database.MockDeviceRepo) {
				repo.EXPECT().UpdateMdadmArrayArchived(gomock.Any(), "md-1", true).Return(errors.New("database is locked"))
			},
		},
		{
			name: "mute",
			path: "/api/mdadm/array/md-1/mute",
			configure: func(repo *mock_database.MockDeviceRepo) {
				repo.EXPECT().UpdateMdadmArrayMuted(gomock.Any(), "md-1", true).Return(errors.New("database is locked"))
			},
		},
		{
			name: "label",
			path: "/api/mdadm/array/md-1/label",
			configure: func(repo *mock_database.MockDeviceRepo) {
				repo.EXPECT().UpdateMdadmArrayLabel(gomock.Any(), "md-1", "x").Return(errors.New("database is locked"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := mdadmActionRouter(t, test.configure)
			body := ""
			if test.name == "label" {
				body = `{"label":"x"}`
			}

			response := mdadmRequest(t, router, http.MethodPost, test.path, body)

			require.Equal(t, http.StatusInternalServerError, response.Code)
			require.JSONEq(t, `{"success":false}`, response.Body.String())
		})
	}
}

// A whitespace-only identifier reaches the handler as an empty one, and must
// not become a delete with an empty filter.
func TestMdadmRoutesRejectABlankIdentifier(t *testing.T) {
	router := mdadmActionRouter(t, nil)

	response := mdadmRequest(t, router, http.MethodDelete, "/api/mdadm/array/%20", "")

	require.Equal(t, http.StatusBadRequest, response.Code)
}
