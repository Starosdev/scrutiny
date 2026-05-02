package handler

import (
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// UploadMdadmMetrics handles MDADM metrics uploaded by the collector
func UploadMdadmMetrics(c *gin.Context) {
	dbRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	uuid := c.Param("uuid")
		if uuid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []string{"UUID is required"}})
			return
		}

		var metrics collector.MDADMMetrics
		if err := c.ShouldBindJSON(&metrics); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []error{err}})
			return
		}

		if err := dbRepo.SaveMdadmMetrics(c.Request.Context(), uuid, metrics); err != nil {
			logger.Errorf("Failed to save MDADM metrics for array %s: %v", uuid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "errors": []string{err.Error()}})
			return
		}

		// Trigger notifications if the array is degraded or has failed devices
		if metrics.FailedDevices > 0 || strings.Contains(strings.ToLower(metrics.State), "degraded") {
			// Get array details to construct the notification
			array, err := dbRepo.GetMdadmArrayDetails(c.Request.Context(), uuid)
			if err == nil {
				// Check if we should notify (avoid repeating alerts if it was already degraded)
				shouldNotify := true
				history, err := dbRepo.GetMdadmMetricsHistory(c.Request.Context(), uuid, "day")
				if err == nil && len(history) > 1 {
					lastMetric := history[len(history)-2] // -1 is the current metric we just saved
					wasDegraded := lastMetric.FailedDevices > 0 || strings.Contains(strings.ToLower(lastMetric.State), "degraded")
					if wasDegraded {
						shouldNotify = false
					}
				}

				if shouldNotify {
					appConfig := c.MustGet("CONFIG").(config.Interface)
					notification := notify.NewMDADMNotify(logger, appConfig, array, metrics)
					notification.LoadDatabaseUrls(c.Request.Context(), dbRepo)
					if err := notification.Send(); err != nil {
						logger.Errorf("Failed to send MDADM notification: %v", err)
					}
				}
			} else {
				logger.Errorf("Failed to retrieve details for MDADM array %s during notification process: %v", uuid, err)
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
