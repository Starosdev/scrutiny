package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
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

	filesystems, filesystemStatus, collectorReporting := deviceFilesystems(c, logger, deviceRepo, device.HostId, device.DeviceName)

	data := map[string]interface{}{
		"device":                         device,
		"smart_results":                  smartResults,
		"filesystems":                    filesystems,
		"filesystem_collector_reporting": collectorReporting,
	}
	if filesystemStatus != nil {
		data["filesystem_status"] = filesystemStatus
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data, "metadata": deviceMetadata})
}

// deviceFilesystems returns the filesystems backed by one drive, the collector
// status for its host, and whether the filesystem collector is reporting for
// any host at all.
//
// All three travel together because an empty list is ambiguous on its own: the
// filesystem collector may never have run anywhere, it may be running but under
// a host id this drive's host does not share, it may have been unable to see the
// host's mounts, or the drive may genuinely hold no mounted filesystem. The UI
// needs to tell a user which of those it is looking at rather than reporting
// zero free space for all four.
//
// The fleet-wide flag is what separates the first two. A host status exists only
// for a host that has reported, so without the flag a host id mismatch between
// the filesystem and SMART collectors is indistinguishable from a deployment
// that never enabled the collector, and the page would stay silent in the one
// case where it has something specific to say.
//
// A read failure degrades to no filesystems: the device detail page is
// primarily a SMART view, and failing the whole response over an optional
// collector's table would hide the data the page exists to show.
func deviceFilesystems(
	c *gin.Context,
	logger *logrus.Entry,
	deviceRepo database.DeviceRepo,
	hostID string,
	deviceName string,
) ([]models.FilesystemCapacity, *models.FilesystemHostStatus, bool) {
	filesystemsByHost, hostStatuses, err := deviceRepo.GetFilesystemSummary(c)
	if err != nil {
		logger.Errorln("An error occurred while retrieving filesystem capacity", err)
		return nil, nil, false
	}

	hostFilesystems := filesystemsByHost[hostID]
	filesystems := make([]models.FilesystemCapacity, 0)
	for i := range hostFilesystems {
		for _, backingDevice := range hostFilesystems[i].BackingDevices {
			if backingDevice == deviceName {
				filesystems = append(filesystems, hostFilesystems[i])
				break
			}
		}
	}

	return database.DedupeFilesystemsBySource(filesystems), hostStatuses[hostID], len(hostStatuses) > 0
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
