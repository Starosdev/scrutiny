package handler

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/gin-gonic/gin"
)

// removeMqttDevice removes a device from Home Assistant via MQTT discovery.
func removeMqttDevice(c *gin.Context, device *models.Device) {
	pubVal, exists := c.Get("MQTT_PUBLISHER")
	if !exists {
		return
	}
	pub, ok := pubVal.(*mqtt.Publisher)
	if !ok || pub == nil {
		return
	}
	pub.RemoveDevice(device)
}

// publishMqttDeviceDiscovery publishes MQTT discovery for a single device.
func publishMqttDeviceDiscovery(c *gin.Context, device *models.Device) {
	pubVal, exists := c.Get("MQTT_PUBLISHER")
	if !exists {
		return
	}
	pub, ok := pubVal.(*mqtt.Publisher)
	if !ok || pub == nil {
		return
	}
	pub.PublishDiscovery(device)
}
