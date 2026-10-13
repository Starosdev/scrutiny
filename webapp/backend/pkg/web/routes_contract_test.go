package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

const openAPIPathForTest = "../../../../docs/openapi.yaml"

var ginParam = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// routeScopes is the intended minimum scope for every authenticated /api route. A route missing here
// fails the contract test, so adding a route forces a decision about who may call it.
var routeScopes = map[string]string{
	"GET /api/health/missed-ping-status":         models.APITokenScopeRead,
	"POST /api/health/notify":                    models.APITokenScopeAdmin,
	"POST /api/health/uptime-kuma-test":          models.APITokenScopeAdmin,
	"POST /api/health/mqtt-sync":                 models.APITokenScopeAdmin,
	"POST /api/devices/register":                 models.APITokenScopeCollector,
	"GET /api/summary":                           models.APITokenScopeRead,
	"GET /api/summary/temp":                      models.APITokenScopeRead,
	"GET /api/summary/temp/devices":              models.APITokenScopeRead,
	"GET /api/summary/workload":                  models.APITokenScopeRead,
	"GET /api/hosts":                             models.APITokenScopeRead,
	"POST /api/hosts/archive":                    models.APITokenScopeAdmin,
	"POST /api/hosts/unarchive":                  models.APITokenScopeAdmin,
	"POST /api/hosts/purge":                      models.APITokenScopeAdmin,
	"GET /api/filesystems/summary":               models.APITokenScopeRead,
	"POST /api/filesystems/summary":              models.APITokenScopeCollector,
	"POST /api/collectors/run":                   models.APITokenScopeAdmin,
	"GET /api/metrics":                           models.APITokenScopeRead,
	"POST /api/device/:id/smart":                 models.APITokenScopeCollector,
	"POST /api/device/:id/selftest":              models.APITokenScopeCollector,
	"GET /api/device/:id/selftest":               models.APITokenScopeRead,
	"GET /api/device/:id/details":                models.APITokenScopeRead,
	"POST /api/device/:id/archive":               models.APITokenScopeAdmin,
	"POST /api/device/:id/unarchive":             models.APITokenScopeAdmin,
	"POST /api/device/:id/mute":                  models.APITokenScopeAdmin,
	"POST /api/device/:id/unmute":                models.APITokenScopeAdmin,
	"POST /api/device/:id/reset-status":          models.APITokenScopeAdmin,
	"POST /api/device/:id/label":                 models.APITokenScopeAdmin,
	"POST /api/device/:id/max-tbw":               models.APITokenScopeAdmin,
	"POST /api/device/:id/smart-display-mode":    models.APITokenScopeAdmin,
	"POST /api/device/:id/missed-ping-timeout":   models.APITokenScopeAdmin,
	"POST /api/device/:id/merge_into":            models.APITokenScopeAdmin,
	"DELETE /api/device/:id":                     models.APITokenScopeAdmin,
	"POST /api/device/:id/performance":           models.APITokenScopeCollector,
	"GET /api/device/:id/performance":            models.APITokenScopeRead,
	"GET /api/device/:id/replacement-risk":       models.APITokenScopeRead,
	"GET /api/device/:id/drive-profile":          models.APITokenScopeRead,
	"POST /api/device/:id/collector-error":       models.APITokenScopeCollector,
	"POST /api/collector/scan-error":             models.APITokenScopeCollector,
	"GET /api/settings":                          models.APITokenScopeRead,
	"POST /api/settings":                         models.APITokenScopeAdmin,
	"GET /api/settings/overrides":                models.APITokenScopeAdmin,
	"POST /api/settings/overrides":               models.APITokenScopeAdmin,
	"DELETE /api/settings/overrides/:id":         models.APITokenScopeAdmin,
	"GET /api/settings/notify-urls":              models.APITokenScopeAdmin,
	"POST /api/settings/notify-urls":             models.APITokenScopeAdmin,
	"DELETE /api/settings/notify-urls/:id":       models.APITokenScopeAdmin,
	"POST /api/settings/notify-urls/:id/test":    models.APITokenScopeAdmin,
	"PATCH /api/settings/notify-urls/:id":        models.APITokenScopeAdmin,
	"GET /api/settings/tokens":                   models.APITokenScopeAdmin,
	"POST /api/settings/tokens":                  models.APITokenScopeAdmin,
	"DELETE /api/settings/tokens/:id":            models.APITokenScopeAdmin,
	"GET /api/reports/generate":                  models.APITokenScopeAdmin,
	"GET /api/reports/history":                   models.APITokenScopeRead,
	"POST /api/zfs/pools/register":               models.APITokenScopeCollector,
	"GET /api/zfs/summary":                       models.APITokenScopeRead,
	"POST /api/zfs/pool/:guid/metrics":           models.APITokenScopeCollector,
	"GET /api/zfs/pool/:guid/details":            models.APITokenScopeRead,
	"POST /api/zfs/pool/:guid/archive":           models.APITokenScopeAdmin,
	"POST /api/zfs/pool/:guid/unarchive":         models.APITokenScopeAdmin,
	"POST /api/zfs/pool/:guid/mute":              models.APITokenScopeAdmin,
	"POST /api/zfs/pool/:guid/unmute":            models.APITokenScopeAdmin,
	"POST /api/zfs/pool/:guid/label":             models.APITokenScopeAdmin,
	"DELETE /api/zfs/pool/:guid":                 models.APITokenScopeAdmin,
	"POST /api/btrfs/filesystems/register":       models.APITokenScopeCollector,
	"GET /api/btrfs/summary":                     models.APITokenScopeRead,
	"POST /api/btrfs/filesystem/:uuid/metrics":   models.APITokenScopeCollector,
	"GET /api/btrfs/filesystem/:uuid/details":    models.APITokenScopeRead,
	"POST /api/btrfs/filesystem/:uuid/archive":   models.APITokenScopeAdmin,
	"POST /api/btrfs/filesystem/:uuid/unarchive": models.APITokenScopeAdmin,
	"POST /api/btrfs/filesystem/:uuid/mute":      models.APITokenScopeAdmin,
	"POST /api/btrfs/filesystem/:uuid/unmute":    models.APITokenScopeAdmin,
	"POST /api/btrfs/filesystem/:uuid/label":     models.APITokenScopeAdmin,
	"DELETE /api/btrfs/filesystem/:uuid":         models.APITokenScopeAdmin,
	"POST /api/mdadm/arrays/register":            models.APITokenScopeCollector,
	"GET /api/mdadm/summary":                     models.APITokenScopeRead,
	"POST /api/mdadm/array/:uuid/metrics":        models.APITokenScopeCollector,
	"GET /api/mdadm/array/:uuid/details":         models.APITokenScopeRead,
}

// publicRoutes need no token (see publicPathSuffixes in middleware/auth.go).
var publicRoutes = map[string]bool{
	"GET /api/auth/status": true,
	"POST /api/auth/login": true,
	"GET /api/version":     true,
	"GET /api/health":      true,
	"HEAD /api/health":     true,
}

// specOnlyAllowlist lists documented operations that Setup registers outside registerAPIRoutes
// (served docs). Everything else in the spec must be a registered route.
var specOnlyAllowlist = map[string]bool{
	"GET /api/docs/openapi.yaml": true,
}

func newContractRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerAPIRoutes(r.Group("/api"), true, true)
	return r
}

func registeredRoutes(r *gin.Engine) map[string]string {
	routes := map[string]string{}
	for _, ri := range r.Routes() {
		routes[ri.Method+" "+ri.Path] = ri.Path
	}
	return routes
}

func loadOpenAPI(t *testing.T) (info map[string]any, ops map[string]bool) {
	t.Helper()
	raw, err := os.ReadFile(openAPIPathForTest)
	require.NoError(t, err)
	var doc struct {
		Info  map[string]any            `yaml:"info"`
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true, "head": true}
	ops = map[string]bool{}
	for path, item := range doc.Paths {
		for m := range item {
			if methods[m] {
				ops[strings.ToUpper(m)+" "+path] = true
			}
		}
	}
	return doc.Info, ops
}

// TestRoutesMatchOpenAPI fails when a route is added to or removed from server.go without the same
// change in docs/openapi.yaml, so the published contract cannot drift silently.
func TestRoutesMatchOpenAPI(t *testing.T) {
	_, spec := loadOpenAPI(t)

	var code []string
	for key := range registeredRoutes(newContractRouter()) {
		method, path, _ := strings.Cut(key, " ")
		code = append(code, method+" "+ginParam.ReplaceAllString(path, "{$1}"))
	}

	var missingFromSpec, missingFromCode []string
	codeSet := map[string]bool{}
	for _, k := range code {
		codeSet[k] = true
		if !spec[k] {
			missingFromSpec = append(missingFromSpec, k)
		}
	}
	for k := range spec {
		if !codeSet[k] && !specOnlyAllowlist[k] {
			missingFromCode = append(missingFromCode, k)
		}
	}
	sort.Strings(missingFromSpec)
	sort.Strings(missingFromCode)
	require.Empty(t, missingFromSpec, "routes registered in server.go but absent from docs/openapi.yaml")
	require.Empty(t, missingFromCode, "operations in docs/openapi.yaml with no registered route")
}

func TestOpenAPIVersionMatchesAPIVersion(t *testing.T) {
	info, _ := loadOpenAPI(t)
	require.Equal(t, handler.APIVersion, info["version"])
}

// TestEveryRouteDeclaresAScope checks the routeScopes table covers the route table exactly.
func TestEveryRouteDeclaresAScope(t *testing.T) {
	for key := range registeredRoutes(newContractRouter()) {
		if publicRoutes[key] {
			continue
		}
		_, ok := routeScopes[key]
		require.True(t, ok, "route %q has no entry in routeScopes (decide its minimum scope)", key)
	}
	registered := registeredRoutes(newContractRouter())
	for key := range routeScopes {
		_, ok := registered[key]
		require.True(t, ok, "routeScopes lists %q but no such route is registered", key)
	}
}

// TestRouteScopeEnforcement sends every authenticated route a request per scope. Handlers are not
// wired to a repository here, so a request that passes the scope check panics inside the handler and
// gin.Recovery turns it into a 500; a request that fails the check is a 403 and never reaches it.
func TestRouteScopeEnforcement(t *testing.T) {
	scopes := []string{models.APITokenScopeRead, models.APITokenScopeCollector, models.APITokenScopeAdmin}
	for key, need := range routeScopes {
		for _, have := range scopes {
			r := gin.New()
			r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
				c.AbortWithStatus(http.StatusInternalServerError)
			}))
			scope := have
			r.Use(func(c *gin.Context) { c.Set("AUTH_SCOPE", scope) })
			registerAPIRoutes(r.Group("/api"), true, true)

			method, path, _ := strings.Cut(key, " ")
			path = ginParam.ReplaceAllString(path, "1")
			w := httptest.NewRecorder()
			req, err := http.NewRequest(method, path, nil)
			require.NoError(t, err)
			r.ServeHTTP(w, req)

			allowed := have == models.APITokenScopeAdmin || have == need
			if allowed {
				require.NotEqual(t, http.StatusForbidden, w.Code, "%s with scope %s should pass the scope check", key, have)
			} else {
				require.Equal(t, http.StatusForbidden, w.Code, "%s with scope %s should be forbidden", key, have)
			}
		}
	}
}
