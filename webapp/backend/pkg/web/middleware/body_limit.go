package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// authBodyLimitBytes caps request bodies on /api/auth/* routes (64 KiB). Login bodies are a few hundred
// bytes and the routes are reachable without credentials.
const authBodyLimitBytes int64 = 64 << 10

// BodyLimitMiddleware caps request bodies at maxBytes (web.listen.max_body_bytes). A request that
// declares a larger Content-Length is rejected with 413 before any handler runs. Bodies without a
// declared length (chunked) are wrapped in http.MaxBytesReader, so reading past the limit fails and the
// handler's bind step reports an error. A maxBytes of zero or less disables the limit. /api/auth/* routes
// get the smaller authBodyLimitBytes cap in addition.
func BodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := maxBytes
		if limit > authBodyLimitBytes && strings.Contains(c.Request.URL.Path, "/api/auth/") {
			limit = authBodyLimitBytes
		}
		if limit <= 0 || c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"error":   "Request body too large.",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
