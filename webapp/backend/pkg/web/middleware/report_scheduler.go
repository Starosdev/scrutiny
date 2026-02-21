package middleware

import (
	"github.com/gin-gonic/gin"
)

// ReportSchedulerMiddleware injects the report scheduler into the gin context
func ReportSchedulerMiddleware(scheduler interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("REPORT_SCHEDULER", scheduler)
		c.Next()
	}
}
