package middleware

import (
	"net"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ForwardedForWarningMiddleware logs one warning (per middleware instance, race-safe) the first time a
// request carries X-Forwarded-For from a peer that is not in web.listen.trusted_proxies. The header is
// never trusted here; it only triggers the warning, and only the peer IP is logged. Behind a reverse
// proxy without trusted_proxies every user shares the proxy IP, so failed logins from one user can
// lock out everyone.
func ForwardedForWarningMiddleware(logger logrus.FieldLogger) gin.HandlerFunc {
	var once sync.Once
	return func(c *gin.Context) {
		if c.GetHeader("X-Forwarded-For") != "" {
			peer, _, err := net.SplitHostPort(c.Request.RemoteAddr)
			// ClientIP equals the peer only when the peer is not a trusted proxy.
			if err == nil && c.ClientIP() == peer {
				once.Do(func() {
					logger.Warnf("Received X-Forwarded-For from %s, which is not in web.listen.trusted_proxies, so the header is ignored. If this is your reverse proxy, add it to web.listen.trusted_proxies; otherwise all users share one IP for login rate limiting.", peer)
				})
			}
		}
		c.Next()
	}
}
