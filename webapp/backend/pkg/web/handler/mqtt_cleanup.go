package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// MqttSync re-syncs all MQTT discovery entities with Home Assistant.
// It cleans up legacy WWN-based topics, removes archived devices, and re-publishes
// discovery + state for all active devices.
func MqttSync(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	pubVal, exists := c.Get("MQTT_PUBLISHER")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "MQTT integration is not enabled"})
		return
	}
	pub, ok := pubVal.(*mqtt.Publisher)
	if !ok || pub == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "MQTT publisher is not connected"})
		return
	}

	published, cleaned, err := pub.SyncAllDevices(deviceRepo, c)
	if err != nil {
		logger.Errorf("MQTT sync failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	logger.Infof("MQTT sync completed: %d devices published, %d legacy topics cleaned", published, cleaned)
	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"devices_published": published,
		"topics_cleaned":    cleaned,
	})
}
