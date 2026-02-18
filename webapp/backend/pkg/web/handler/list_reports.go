package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type reportFileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func ListReports(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	// Load settings from DB to get PDF path (consistent with scheduler)
	pdfPath := "/opt/scrutiny/reports"
	settings, err := deviceRepo.LoadSettings(c)
	if err == nil && settings != nil && settings.Metrics.ReportPDFPath != "" {
		pdfPath = settings.Metrics.ReportPDFPath
	}

	entries, err := os.ReadDir(pdfPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "reports": []reportFileEntry{}})
			return
		}
		logger.Errorf("Failed to list reports directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
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
			Path:    filepath.Join(pdfPath, entry.Name()),
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
