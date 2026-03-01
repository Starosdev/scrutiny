package middleware

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
)

// NotificationGateMiddleware injects the notification gate into the gin context
func NotificationGateMiddleware(gate *notify.NotificationGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("NOTIFICATION_GATE", gate)
		c.Next()
	}
}
