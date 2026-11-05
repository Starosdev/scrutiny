package handler

import (
	"errors"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/metrics"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// UploadZFSPoolMetrics receives ZFS pool metrics from the collector and saves them
func UploadZFSPoolMetrics(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	guid := c.Param("guid")
	if err := validation.ValidateGUID(guid); err != nil {
		logger.Warnf(fmtInvalidGUID, guid)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var pool models.ZFSPool
	err := c.BindJSON(&pool)
	if err != nil {
		logger.Errorln("Cannot parse ZFS pool metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Ensure the GUID matches the URL parameter
	pool.GUID = guid

	// The previous state has to be read before the register call replaces it.
	// A read error other than not-found leaves the previous state unknown, and
	// comparing against an unknown state would report every existing error
	// total as new, so that upload skips the notification check.
	var previous *models.ZFSPool
	previousKnown := true
	if stored, err := deviceRepo.GetZFSPoolDetails(c, guid); err == nil {
		previous = &stored
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Warnf("Could not read previous ZFS state for pool %s, skipping notification check: %v", guid, err)
		previousKnown = false
	}

	// Update the pool in the database
	if err := deviceRepo.RegisterZFSPool(c, pool); err != nil {
		logger.Errorln("An error occurred while updating ZFS pool", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Save metrics to InfluxDB
	if err := deviceRepo.SaveZFSPoolMetrics(c, pool); err != nil {
		logger.Errorln("An error occurred while saving ZFS pool metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	if previousKnown {
		maybeNotifyZFS(c, deviceRepo, logger, previous, &pool)
	}

	if collectorVal, exists := c.Get("METRICS_COLLECTOR"); exists {
		if collector, ok := collectorVal.(*metrics.Collector); ok && collector != nil {
			if err := collector.RefreshZFSPoolMetrics(deviceRepo, c); err != nil {
				logger.Warnf("Failed to refresh Prometheus ZFS metrics for pool %s: %v", guid, err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func maybeNotifyZFS(c *gin.Context, deviceRepo database.DeviceRepo, logger *logrus.Entry, previous *models.ZFSPool, current *models.ZFSPool) {
	notifyStateChange(c, deviceRepo, logger, stateNotification{
		kind:       "ZFS pool",
		identifier: current.GUID,
		silenced:   previous != nil && (previous.Muted || previous.Archived),
		failureTypes: func(repeat bool, usageThreshold int) []string {
			return notify.ZFSFailureTypes(previous, current, repeat, usageThreshold)
		},
		build: func(appConfig config.Interface, failureTypes []string) notify.Notify {
			return notify.NewZFSNotify(logger, appConfig, current, failureTypes)
		},
	})
}
