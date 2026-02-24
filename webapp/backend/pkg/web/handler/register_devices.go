package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

// register devices that are detected by various collectors.
// This function is run everytime a collector is about to start a run. It can be used to update device metadata.
func RegisterDevices(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	var collectorDeviceWrapper models.DeviceWrapper
	err := c.BindJSON(&collectorDeviceWrapper)
	if err != nil {
		logger.Errorln("Cannot parse detected devices", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	//filter any device with empty wwn (they are invalid)
	detectedStorageDevices := lo.Filter[models.Device](collectorDeviceWrapper.Data, func(dev models.Device, _ int) bool {
		return len(dev.WWN) > 0
	})

	errs := []error{}
	for _, dev := range detectedStorageDevices {
		//insert devices into DB (and update specified columns if device is already registered)
		// update device fields that may change: (DeviceType, HostID)
		if err := deviceRepo.RegisterDevice(c, dev); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		logger.Errorln("An error occurred while registering devices", errs)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
		})
		return
	}

	// Publish MQTT discovery for registered devices (if enabled)
	publishMqttDiscovery(c, deviceRepo, detectedStorageDevices)

	c.JSON(http.StatusOK, models.DeviceWrapper{
		Success: true,
		Data:    detectedStorageDevices,
	})
}

func publishMqttDiscovery(c *gin.Context, deviceRepo database.DeviceRepo, devices []models.Device) {
	pubVal, exists := c.Get("MQTT_PUBLISHER")
	if !exists {
		return
	}
	pub, ok := pubVal.(*mqtt.Publisher)
	if !ok || pub == nil {
		return
	}
	for _, dev := range devices {
		// Fetch device from DB to get the actual archived status
		// (collector-sent devices don't have this field set correctly)
		if dbDevice, err := deviceRepo.GetDeviceDetails(c, dev.WWN); err == nil && !dbDevice.Archived {
			pub.PublishDiscovery(dbDevice)
		}
	}
}
