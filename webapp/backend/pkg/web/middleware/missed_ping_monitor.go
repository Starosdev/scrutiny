package middleware

import (
	"github.com/gin-gonic/gin"
)

// MissedPingMonitorMiddleware injects the missed ping monitor into the gin context
func MissedPingMonitorMiddleware(monitor interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("MISSED_PING_MONITOR", monitor)
		c.Next()
	}
}
