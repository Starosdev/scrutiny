package middleware

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/gin-gonic/gin"
)

// MqttPublisherMiddleware injects MQTT publisher into gin context
func MqttPublisherMiddleware(publisher *mqtt.Publisher) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("MQTT_PUBLISHER", publisher)
		c.Next()
	}
}
