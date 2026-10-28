package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
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
		deviceMetadata = thresholds.ScsiMetadata
	}

	filesystems, filesystemStatus := deviceFilesystems(c, logger, deviceRepo, device.HostId, device.DeviceName)

	data := map[string]interface{}{"device": device, "smart_results": smartResults, "filesystems": filesystems}
	if filesystemStatus != nil {
		data["filesystem_status"] = filesystemStatus
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data, "metadata": deviceMetadata})
}

// deviceFilesystems returns the filesystems backed by one drive, and the
// collector status for its host.
//
// The status travels with the filesystems because an empty list is ambiguous on
// its own: the filesystem collector may never have run, it may have been unable
// to see the host's mounts, or the drive may genuinely hold no mounted
// filesystem. The UI needs to tell a user which of those it is looking at
// rather than reporting zero free space for all three.
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
) ([]models.FilesystemCapacity, *models.FilesystemHostStatus) {
	filesystemsByHost, hostStatuses, err := deviceRepo.GetFilesystemSummary(c)
	if err != nil {
		logger.Errorln("An error occurred while retrieving filesystem capacity", err)
		return nil, nil
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

	return filesystems, hostStatuses[hostID]
}
