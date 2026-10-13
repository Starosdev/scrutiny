package middleware

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
)

// scopeSatisfies reports whether a caller holding scope have may use a route that needs scope need.
// admin satisfies everything; read and collector satisfy only themselves, so a collector token
// cannot read settings and a read token cannot upload metrics.
func scopeSatisfies(have, need string) bool {
	return have == models.APITokenScopeAdmin || (have != "" && have == need)
}

// RequireScope rejects the request with 403 unless the authenticated caller holds the scope (or admin).
// It runs after AuthMiddleware. A missing AUTH_SCOPE is denied, so a route group that is registered
// without AuthMiddleware in front fails closed.
func RequireScope(need string) gin.HandlerFunc {
	return func(c *gin.Context) {
		have, _ := c.Get(authScopeKey)
		haveScope, _ := have.(string)
		if !scopeSatisfies(haveScope, need) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "This token does not have the required scope: " + need,
			})
			return
		}
		c.Next()
	}
}
