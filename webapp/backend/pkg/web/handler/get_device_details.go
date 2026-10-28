package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetDeviceDetails(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	durationKey, exists := c.GetQuery("duration_key")
	if !exists {
		durationKey = "forever"
	}

	smartResults, err := deviceRepo.GetSmartAttributeHistory(c, device.DeviceID, durationKey, 0, 0, nil)
	if err != nil {
		logger.Errorln("An error occurred while retrieving device smart results", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	mergedOverrides := deviceRepo.GetMergedOverrides(c)
	for i := range smartResults {
		smartResults[i].ApplyOverrides(mergedOverrides, device.DeviceID)
	}

	var deviceMetadata interface{}
	if device.IsAta() {
		// Merge standard ATA SMART attribute metadata with ATA Device Statistics metadata
		// Device statistics (like devstat_7_8 for Percentage Used Endurance Indicator)
		// are critical for enterprise SSD monitoring
		mergedMetadata := make(map[string]interface{})
		for k, v := range thresholds.AtaMetadata {
			mergedMetadata[fmt.Sprintf("%d", k)] = v
		}
		for k, v := range thresholds.AtaDeviceStatsMetadata {
			mergedMetadata[k] = v
		}
		for k, v := range thresholds.FarmMetadata {
			mergedMetadata[k] = v
		}
		deviceMetadata = mergedMetadata
	} else if device.IsNvme() {
		deviceMetadata = thresholds.NmveMetadata
	} else if device.IsScsi() {
		// The per-PHY SAS counters and the per-sensor lifetime temperatures carry numbers the
		// drive reported, so their metadata has to be resolved from the device's own attributes.
		deviceMetadata = thresholds.ScsiMetadataForAttributes(attributeIDsInHistory(smartResults))
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": map[string]interface{}{"device": device, "smart_results": smartResults}, "metadata": deviceMetadata})
}

// attributeIDsInHistory returns every attribute ID appearing anywhere in a device's returned
// SMART history. It covers the whole history rather than only the newest submission, so an
// attribute the drive has stopped reporting keeps its display name on its own history chart.
func attributeIDsInHistory(smartResults []measurements.Smart) []string {
	seen := map[string]struct{}{}
	var attributeIDs []string
	for i := range smartResults {
		for attributeID := range smartResults[i].Attributes {
			if _, ok := seen[attributeID]; ok {
				continue
			}
			seen[attributeID] = struct{}{}
			attributeIDs = append(attributeIDs, attributeID)
		}
	}
	return attributeIDs
}
