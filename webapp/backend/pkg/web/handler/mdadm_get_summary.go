package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetMdadmSummary returns a summary of all MDADM arrays with their latest metrics
func GetMdadmSummary(c *gin.Context) {
	dbRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	arrays, err := dbRepo.GetMdadmArrays(c.Request.Context())
	if err != nil {
		logger.Errorf("Failed to get MDADM arrays summary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "errors": []string{err.Error()}})
		return
	}

	// Build summary with latest metrics for each array
	type ArraySummary struct {
		UUID     string   `json:"uuid"`
		Name     string   `json:"name"`
		Level    string   `json:"level"`
		Devices  []string `json:"devices"`
		Label    string   `json:"label,omitempty"`
		Archived bool     `json:"archived"`
		Muted    bool     `json:"muted"`

		// Latest metrics (populated from InfluxDB)
		State        string  `json:"state,omitempty"`
		SyncProgress float64 `json:"sync_progress,omitempty"`
		ArraySize    int64   `json:"array_size,omitempty"`
		UsedBytes    int64   `json:"used_bytes,omitempty"`
	}

	summaries := make([]ArraySummary, 0, len(arrays))
	for _, array := range arrays {
		summary := ArraySummary{
			UUID:     array.UUID,
			Name:     array.Name,
			Level:    array.Level,
			Devices:  array.Devices,
			Label:    array.Label,
			Archived: array.Archived,
			Muted:    array.Muted,
		}

		// Fetch latest metrics for this array
		latest, err := dbRepo.GetLatestMdadmMetrics(c.Request.Context(), array.UUID)
		if err != nil {
			logger.Warnf("Failed to get latest metrics for array %s: %v", array.UUID, err)
		} else if latest != nil {
			summary.State = latest.State
			summary.SyncProgress = latest.SyncProgress
			summary.ArraySize = latest.ArraySize
			summary.UsedBytes = latest.UsedBytes
		}

		summaries = append(summaries, summary)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summaries,
	})
}
