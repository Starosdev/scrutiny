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
// show a login form. It returns only the release version and booleans. It carries no paths, hosts,
// secrets, or token material. The cost is that an unauthenticated caller can fingerprint the release;
// /api/auth/status already exposes that this is Scrutiny with auth settings, and the release
// version is public on GitHub.
func GetVersion(c *gin.Context) {
	appConfig := c.MustGet("CONFIG").(config.Interface)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"version":     version.VERSION,
		"api_version": APIVersion,
		"capabilities": gin.H{
			"auth_required":                  auth.IsAuthEnabled(appConfig),
			"scoped_tokens":                  true,
			"collector_trigger_enabled":      collectorTriggerEnabled(),
			"zfs_pool_modifications_allowed": zfsPoolModificationsAllowed(c),
			"metrics_enabled":                appConfig.GetBool("web.metrics.enabled"),
			"mqtt_enabled":                   appConfig.GetBool("web.mqtt.enabled"),
		},
	})
}
