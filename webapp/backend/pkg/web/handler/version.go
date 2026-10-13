package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/gin-gonic/gin"
)

// APIVersion is the major version of the HTTP API contract in docs/openapi.yaml. Paths documented in
// the spec are stable within a major version and fields are only added. Bump it only for a break.
const APIVersion = "2"

// GetVersion reports the server version, the API version, and capability flags so clients (the web
// UI, mobile apps) can adapt without parsing settings responses.
//
// It is public, like /api/auth/status, because a client must read it before it can log in: it needs
// api_version to decide whether it can talk to this server and auth_required to decide whether to
// show a login form. Anonymous callers get only the release version, api_version and auth_required; the full capabilities object needs any valid credential. It carries no paths, hosts,
// secrets, or token material. The cost is that an unauthenticated caller can fingerprint the release;
// /api/auth/status already exposes that this is Scrutiny with auth settings, and the release
// version is public on GitHub.
func GetVersion(c *gin.Context) {
	appConfig := c.MustGet("CONFIG").(config.Interface)

	resp := gin.H{
		"success":       true,
		"version":       version.VERSION,
		"api_version":   APIVersion,
		"auth_required": auth.IsAuthEnabled(appConfig),
	}
	// AuthMiddleware sets AUTH_SCOPE when the request carries a valid credential (or auth is off).
	// Anonymous callers get only the three fields above.
	if scope, _ := c.Get("AUTH_SCOPE"); scope != nil {
		resp["capabilities"] = gin.H{
			"auth_required":                  auth.IsAuthEnabled(appConfig),
			"scoped_tokens":                  true,
			"collector_trigger_enabled":      collectorTriggerEnabled(),
			"zfs_pool_modifications_allowed": zfsPoolModificationsAllowed(c),
			"metrics_enabled":                appConfig.GetBool("web.metrics.enabled"),
			"mqtt_enabled":                   appConfig.GetBool("web.mqtt.enabled"),
		}
	}
	c.JSON(http.StatusOK, resp)
}
