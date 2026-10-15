package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetDevicesSummary(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	if _, paginated := c.GetQuery("page"); paginated {
		getPaginatedDevicesSummary(c, logger, deviceRepo)
		return
	}

	summary, err := deviceRepo.GetSummary(c)
	if err != nil {
		logger.Errorln("An error occurred while retrieving device summary", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	annotateStaleness(c, logger, deviceRepo, summary)

	//this must match DeviceSummaryWrapper (webapp/backend/pkg/models/device_summary.go)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"summary": summary,
			//"temperature": tem
		},
	})
}

func getPaginatedDevicesSummary(c *gin.Context, logger *logrus.Entry, deviceRepo database.DeviceRepo) {
	options, err := parseSummaryPageOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	result, err := deviceRepo.GetSummaryPage(c, options)
	if err != nil {
		logger.Errorln("An error occurred while retrieving paginated device summary", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	annotateStaleness(c, logger, deviceRepo, result.Summary)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"summary":    result.Summary,
			"pagination": result.Pagination,
		},
	})
}

// LastSeenCacheTTL bounds how often /api/summary runs the multi-bucket last-seen query.
// Set to 0 to disable caching.
var LastSeenCacheTTL = 30 * time.Second

var lastSeenCache struct {
	fetched time.Time
	times   map[string]time.Time
	mu      sync.Mutex
}

func cachedLastSeenTimes(c *gin.Context, deviceRepo database.DeviceRepo) (map[string]time.Time, error) {
	lastSeenCache.mu.Lock()
	defer lastSeenCache.mu.Unlock()
	if LastSeenCacheTTL > 0 && lastSeenCache.times != nil && time.Since(lastSeenCache.fetched) < LastSeenCacheTTL {
		return lastSeenCache.times, nil
	}
	times, err := deviceRepo.GetDevicesLastSeenTimes(c)
	if err != nil {
		return nil, err
	}
	lastSeenCache.times, lastSeenCache.fetched = times, time.Now()
	return times, nil
}

// annotateStaleness sets LastSeen and Stale on each summary using the same last-seen data and
// timeout rules as the missed-ping monitor. A failure here is logged and leaves the fields unset
// (Stale nil, distinct from false): staleness is advisory and must not break the dashboard.
// Muted devices are annotated like any other; mute only suppresses notifications.
func annotateStaleness(c *gin.Context, logger *logrus.Entry, deviceRepo database.DeviceRepo, summaries map[string]*models.DeviceSummary) {
	settings, err := deviceRepo.LoadSettings(c)
	if err != nil || settings == nil {
		logger.Warnln("Could not load settings for stale device detection", err)
		return
	}
	lastSeenTimes, err := cachedLastSeenTimes(c, deviceRepo)
	if err != nil {
		logger.Warnln("Could not load last seen times for stale device detection", err)
		return
	}

	// Stale is only meaningful once the operator has enabled missed ping monitoring and set a
	// timeout that matches the collector schedule. The collector runs daily by default, so the
	// 60 minute default timeout would mark every device stale for most of the day.
	reportStale := settings.Metrics.NotifyOnMissedPing
	now := time.Now()
	for id, summary := range summaries {
		lastSeen, ok := lastSeenTimes[id]
		if !ok {
			continue
		}
		summary.LastSeen = &lastSeen
		if reportStale {
			stale := summary.Device.IsStale(lastSeen, now, settings.Metrics.MissedPingTimeoutMinutes)
			summary.Stale = &stale
		}
	}
}

func parseSummaryPageOptions(c *gin.Context) (models.DeviceSummaryPageOptions, error) {
	options := models.DeviceSummaryPageOptions{
		Archived:   false,
		Sort:       c.Query("sort"),
		Display:    c.Query("display"),
		HostSearch: c.Query("host"),
	}
	if groupBy := c.Query("group_by"); groupBy != "" {
		if groupBy != "host" {
			return options, fmt.Errorf("group_by must be host")
		}
		options.GroupByHost = true
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 1 {
		return options, fmt.Errorf("page must be a positive integer")
	}
	options.Page = page

	if pageSizeValue, exists := c.GetQuery("page_size"); exists {
		pageSize, parseErr := strconv.Atoi(pageSizeValue)
		if parseErr != nil || (!options.GroupByHost && !validSummaryPageSize(pageSize)) {
			return options, fmt.Errorf("page_size must be one of 25, 50, 100, or 250")
		}
		if options.GroupByHost && !validSummaryHostPageSize(pageSize) {
			return options, fmt.Errorf("page_size must be one of 5, 10, 25, or 50 when group_by=host")
		}
		options.PageSize = pageSize
	}

	if archivedValue, exists := c.GetQuery("archived"); exists {
		archived, parseErr := strconv.ParseBool(archivedValue)
		if parseErr != nil {
			return options, fmt.Errorf("archived must be true or false")
		}
		options.Archived = archived
	}

	if options.Sort != "" && !validSummarySort(options.Sort) {
		return options, fmt.Errorf("unsupported dashboard sort")
	}
	if options.Display != "" && !validSummaryDisplay(options.Display) {
		return options, fmt.Errorf("unsupported dashboard display")
	}

	return options, nil
}

func validSummaryPageSize(pageSize int) bool {
	return pageSize == 25 || pageSize == 50 || pageSize == 100 || pageSize == 250
}

func validSummaryHostPageSize(pageSize int) bool {
	return pageSize == 5 || pageSize == 10 || pageSize == 25 || pageSize == 50
}

func validSummarySort(sort string) bool {
	switch sort {
	case "status", "status_asc", "status_desc",
		"title", "title_asc", "title_desc",
		"age", "age_asc", "age_desc",
		"capacity_asc", "capacity_desc",
		"temperature_asc", "temperature_desc":
		return true
	default:
		return false
	}
}

func validSummaryDisplay(display string) bool {
	return display == "name" || display == "serial_id" || display == "uuid" || display == "label"
}
