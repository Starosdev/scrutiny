package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/gin-gonic/gin"
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

	errs := []error{}
	detectedStorageDevices := collectorDeviceWrapper.Data
	for i := range detectedStorageDevices {
		// Compute DeviceID before registration so it is present in the response.
		// RegisterDevice performs the same computation internally; doing it here
		// ensures the response payload carries the device_id the collector should
		// use for subsequent API calls (e.g. SMART submission).
		if detectedStorageDevices[i].DeviceID == "" {
			detectedStorageDevices[i].DeviceID = deviceid.Generate(
				detectedStorageDevices[i].ModelName,
				detectedStorageDevices[i].SerialNumber,
				detectedStorageDevices[i].WWN,
			)
		}
		//insert devices into DB (and update specified columns if device is already registered)
		// update device fields that may change: (DeviceType, HostID)
		if err := deviceRepo.RegisterDevice(c, detectedStorageDevices[i]); err != nil {
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
	for i := range devices {
		// Compute DeviceID if not already set (collector may not populate it)
		devID := devices[i].DeviceID
		if devID == "" {
			devID = deviceid.Generate(devices[i].ModelName, devices[i].SerialNumber, devices[i].WWN)
		}
		// Fetch device from DB to get the actual archived status
		// (collector-sent devices don't have this field set correctly)
		if dbDevice, err := deviceRepo.GetDeviceDetails(c, devID); err == nil && !dbDevice.Archived {
			pub.PublishDiscovery(&dbDevice)
		}
	}
}
