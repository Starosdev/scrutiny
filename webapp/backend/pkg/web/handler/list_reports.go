package handler

import (
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/reports"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type reportFileEntry struct {
	Name    string `json:"name"`
	ModTime string `json:"mod_time"`
	Size    int64  `json:"size"`
}

func ListReports(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	appConfig := c.MustGet("CONFIG").(config.Interface)

	// The reports directory is fixed in config (web.reports.path); the API cannot choose it.
	pdfPath := appConfig.GetString("web.reports.path")
	if pdfPath == "" {
		pdfPath = reports.DefaultPDFDir
	}

	entries, err := os.ReadDir(pdfPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "reports": []reportFileEntry{}})
			return
		}
		logger.Errorf("Failed to list reports directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to list reports"})
		return
	}

	var files []reportFileEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pdf") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, reportFileEntry{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime > files[j].ModTime
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"reports": files,
	})
}
