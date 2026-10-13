package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListReports_ReturnsNamesWithoutAbsolutePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scrutiny-daily.pdf"), []byte("pdf"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600))

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	cfg := mock_config.NewMockInterface(mockCtrl)
	cfg.EXPECT().GetString("web.reports.path").Return(dir).AnyTimes()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("LOGGER", logrus.NewEntry(logrus.New()))
		c.Set("CONFIG", cfg)
	})
	r.GET("/api/reports/history", handler.ListReports)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/reports/history", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), dir, "the server filesystem path must not be returned")
	var resp struct {
		Reports []map[string]interface{} `json:"reports"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Reports, 1)
	require.Equal(t, "scrutiny-daily.pdf", resp.Reports[0]["name"])
	require.NotContains(t, resp.Reports[0], "path")
}
