package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimitMiddleware caps request bodies at maxBytes (web.listen.max_body_bytes). A request that
// declares a larger Content-Length is rejected with 413 before any handler runs. Bodies without a
// declared length (chunked) are wrapped in http.MaxBytesReader, so reading past the limit fails and the
// handler's bind step reports an error. A maxBytes of zero or less disables the limit.
func BodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 || c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"error":   "Request body too large.",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
