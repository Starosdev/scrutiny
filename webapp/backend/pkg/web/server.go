package web

import (
	"context"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/analogj/scrutiny/pkg/utils"
	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/errors"
	"github.com/analogj/scrutiny/webapp/backend/pkg/leader"
	"github.com/analogj/scrutiny/webapp/backend/pkg/metrics"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/reports"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/middleware"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const configKeyMetricsEnabled = "web.metrics.enabled"
const configKeyMqttEnabled = "web.mqtt.enabled"
const configKeyTrustedProxies = "web.listen.trusted_proxies"
const indexFile = "index.html"
const apiDocsDirName = "docs"
const swaggerUIFile = "swagger-ui.html"
const openAPIFile = "openapi.yaml"
const apiSummaryPath = "/summary"

type AppEngine struct {
	Config            config.Interface
	Logger            *logrus.Entry
	MetricsCollector  *metrics.Collector
	MqttPublisher     *mqtt.Publisher
	NotificationGate  *notify.NotificationGate
	MissedPingMonitor *MissedPingMonitor
	HeartbeatMonitor  *HeartbeatMonitor
	UptimeKumaMonitor *UptimeKumaMonitor
	ReportScheduler   *reports.Scheduler

	// Leader elects the one replica that runs the monitors and the report scheduler (#880).
	// Nil means this process always runs them.
	Leader     *leader.Elector
	leaderRepo database.DeviceRepo

	// OutboxWorker delivers notifications recorded by any replica, on the leader only.
	OutboxWorker *NotificationOutboxWorker
}

// isLeader reports whether this replica should run background jobs.
func (ae *AppEngine) isLeader() bool {
	return ae.Leader == nil || ae.Leader.Held()
}

// routeGroups holds one gin group per minimum scope. A route is registered on the group that names
// the least privilege it needs; admin callers (master token, JWT, admin tokens) pass every group.
type routeGroups struct {
	read      *gin.RouterGroup // GET endpoints that only read data
	collector *gin.RouterGroup // uploads and registration performed by collectors
	admin     *gin.RouterGroup // settings, destructive actions, anything that changes state from the UI
}

func newRouteGroups(parent *gin.RouterGroup) routeGroups {
	return routeGroups{
		read:      parent.Group("", middleware.RequireScope(models.APITokenScopeRead)),
		collector: parent.Group("", middleware.RequireScope(models.APITokenScopeCollector)),
		admin:     parent.Group("", middleware.RequireScope(models.APITokenScopeAdmin)),
	}
}

// sub returns the groups nested under a path prefix, keeping each group's scope check.
func (g routeGroups) sub(path string) routeGroups {
	return routeGroups{read: g.read.Group(path), collector: g.collector.Group(path), admin: g.admin.Group(path)}
}

func registerZFSPoolRoutes(zfs routeGroups, allowPoolModifications bool) {
	zfs.collector.POST("/pools/register", handler.RegisterZFSPools)         // used by ZFS Collector to register pools
	zfs.read.GET(apiSummaryPath, handler.GetZFSPoolsSummary)                // used by ZFS Dashboard
	zfs.collector.POST("/pool/:guid/metrics", handler.UploadZFSPoolMetrics) // used by ZFS Collector to upload metrics
	zfs.read.GET("/pool/:guid/details", handler.GetZFSPoolDetails)          // used by ZFS Pool Details view

	zfsPoolMutations := zfs.admin.Group("/pool/:guid", middleware.ZFSPoolModificationGuard(allowPoolModifications))
	zfsPoolMutations.POST("/archive", handler.ArchiveZFSPool)
	zfsPoolMutations.POST("/unarchive", handler.UnarchiveZFSPool)
	zfsPoolMutations.POST("/mute", handler.MuteZFSPool)
	zfsPoolMutations.POST("/unmute", handler.UnmuteZFSPool)
	zfsPoolMutations.POST("/label", handler.UpdateZFSPoolLabel)
	zfsPoolMutations.DELETE("", handler.DeleteZFSPool)
}

// registerAPIRoutes registers every /api route. Public routes (auth, health, version) sit directly on
// api; everything else goes through a scope group. metricsEnabled gates /api/metrics.
func registerAPIRoutes(api *gin.RouterGroup, metricsEnabled bool, allowZFSPoolModifications bool) {
	// Public routes. AuthMiddleware lets these through without a token; keep them in step with
	// publicPathSuffixes in middleware/auth.go.
	api.GET("/auth/status", handler.AuthStatus)
	api.POST("/auth/login", handler.Login)
	api.GET("/version", handler.GetVersion)
	api.GET("/health", handler.HealthCheck)
	api.HEAD("/health", handler.HealthCheck)

	g := newRouteGroups(api)

	g.admin.POST("/health/notify", handler.SendTestNotification)          // check if notifications are configured correctly
	g.read.GET("/health/missed-ping-status", handler.GetMissedPingStatus) // get missed ping monitor diagnostic status
	g.admin.POST("/health/uptime-kuma-test", handler.TestUptimeKumaPush)  // test Uptime Kuma push monitor
	g.admin.POST("/health/mqtt-sync", handler.MqttSync)                   // re-sync all MQTT discovery entities with HA

	g.collector.POST("/devices/register", handler.RegisterDevices)    // used by Collector to register new devices and retrieve filtered list
	g.read.GET(apiSummaryPath, handler.GetDevicesSummary)             // used by Dashboard
	g.read.GET("/summary/temp", handler.GetDevicesSummaryTempHistory) // used by Dashboard (Temperature history dropdown)
	g.read.GET("/summary/temp/devices", handler.GetTemperatureDeviceOptions)
	g.read.GET("/summary/workload", handler.GetWorkloadInsights) // used by Workload Insights page
	g.read.GET("/hosts", handler.GetHosts)                       // used by SMART host management
	g.admin.POST("/hosts/archive", handler.ArchiveHosts)
	g.admin.POST("/hosts/unarchive", handler.UnarchiveHosts)
	g.admin.POST("/hosts/purge", handler.PurgeHosts)
	g.read.GET("/filesystems/summary", handler.GetFilesystemSummary)          // used by Dashboard filesystem capacity panel
	g.collector.POST("/filesystems/summary", handler.UploadFilesystemSummary) // used by Filesystem Collector to upload data
	g.admin.POST("/collectors/run", handler.TriggerCollectors)                // used by Dashboard to trigger local collectors in omnibus mode

	// Prometheus metrics endpoint (only registered if enabled)
	if metricsEnabled {
		g.read.GET("/metrics", handler.GetMetrics)
	}

	g.collector.POST("/device/:id/smart", handler.UploadDeviceMetrics) // used by Collector to upload data
	g.collector.POST("/device/:id/selftest", handler.UploadDeviceSelfTests)
	g.read.GET("/device/:id/selftest", handler.GetDeviceSelfTests)                         // used by UI to view self-test history
	g.read.GET("/device/:id/details", handler.GetDeviceDetails)                            // used by Details
	g.admin.POST("/device/:id/archive", handler.ArchiveDevice)                             // used by UI to archive device
	g.admin.POST("/device/:id/unarchive", handler.UnarchiveDevice)                         // used by UI to unarchive device
	g.admin.POST("/device/:id/mute", handler.MuteDevice)                                   // used by UI to mute device
	g.admin.POST("/device/:id/unmute", handler.UnmuteDevice)                               // used by UI to unmute device
	g.admin.POST("/device/:id/reset-status", handler.ResetDeviceStatus)                    // used by UI to reset device failed status
	g.admin.POST("/device/:id/label", handler.UpdateDeviceLabel)                           // used by UI to set device label
	g.admin.POST("/device/:id/max-tbw", handler.UpdateDeviceMaxTBW)                        // used by UI to set per-device rated TBW
	g.admin.POST("/device/:id/smart-display-mode", handler.UpdateDeviceSmartDisplayMode)   // used by UI to set SMART attribute display mode
	g.admin.POST("/device/:id/missed-ping-timeout", handler.UpdateDeviceMissedPingTimeout) // used by UI to set per-device missed ping timeout override
	g.admin.POST("/device/:id/merge_into", handler.MergeDeviceInto)                        // used by API/CLI to merge duplicate devices
	g.admin.DELETE("/device/:id", handler.DeleteDevice)                                    // used by UI to delete device
	g.collector.POST("/device/:id/performance", handler.UploadDevicePerformance)           // used by Collector to upload performance benchmarks
	g.read.GET("/device/:id/performance", handler.GetDevicePerformance)                    // used by UI to view performance history
	g.read.GET("/device/:id/replacement-risk", handler.GetDeviceReplacementRisk)           // used by UI to display drive replacement prediction
	g.read.GET("/device/:id/drive-profile", handler.GetDeviceDriveProfile)                 // debug/inspection surface for consumer drive profile matching
	g.collector.POST("/device/:id/collector-error", handler.UploadCollectorError)          // used by Collector to report smartctl errors
	g.collector.POST("/collector/scan-error", handler.UploadCollectorScanError)            // used by Collector to report scan-level errors (no device context)

	g.read.GET("/settings", handler.GetSettings)    // used to get settings
	g.admin.POST("/settings", handler.SaveSettings) // used to save settings

	// Attribute Override endpoints (UI-configurable SMART overrides)
	g.admin.GET("/settings/overrides", handler.GetAttributeOverrides)
	g.admin.POST("/settings/overrides", handler.SaveAttributeOverride)
	g.admin.DELETE("/settings/overrides/:id", handler.DeleteAttributeOverride)

	// Notification URL endpoints (UI-configurable notification channels)
	g.admin.GET("/settings/notify-urls", handler.GetNotifyUrls)
	g.admin.POST("/settings/notify-urls", handler.SaveNotifyUrl)
	g.admin.DELETE("/settings/notify-urls/:id", handler.DeleteNotifyUrl)
	g.admin.POST("/settings/notify-urls/:id/test", handler.TestNotifyUrl)
	g.admin.PATCH("/settings/notify-urls/:id", handler.UpdateNotifyUrlHeartbeat)

	// Named API token endpoints (admin only)
	g.admin.GET("/settings/tokens", handler.ListAPITokens)
	g.admin.POST("/settings/tokens", handler.CreateAPIToken)
	g.admin.DELETE("/settings/tokens/:id", handler.RevokeAPIToken)

	// Scheduled report endpoints
	g.admin.GET("/reports/generate", handler.GenerateReport) // renders a report on demand, so not a plain read
	g.read.GET("/reports/history", handler.ListReports)

	registerZFSPoolRoutes(g.sub("/zfs"), allowZFSPoolModifications)

	btrfs := g.sub("/btrfs")
	btrfs.collector.POST("/filesystems/register", handler.RegisterBtrfsFilesystems)
	btrfs.read.GET(apiSummaryPath, handler.GetBtrfsFilesystemsSummary)
	btrfs.collector.POST("/filesystem/:uuid/metrics", handler.UploadBtrfsMetrics)
	btrfs.read.GET("/filesystem/:uuid/details", handler.GetBtrfsFilesystemDetails)
	btrfs.admin.POST("/filesystem/:uuid/archive", handler.ArchiveBtrfsFilesystem)
	btrfs.admin.POST("/filesystem/:uuid/unarchive", handler.UnarchiveBtrfsFilesystem)
	btrfs.admin.POST("/filesystem/:uuid/mute", handler.MuteBtrfsFilesystem)
	btrfs.admin.POST("/filesystem/:uuid/unmute", handler.UnmuteBtrfsFilesystem)
	btrfs.admin.POST("/filesystem/:uuid/label", handler.UpdateBtrfsFilesystemLabel)
	btrfs.admin.DELETE("/filesystem/:uuid", handler.DeleteBtrfsFilesystem)

	// MDADM Array API endpoints
	mdadm := g.sub("/mdadm")
	mdadm.collector.POST("/arrays/register", handler.RegisterMdadmArrays)    // used by Collector to register new arrays
	mdadm.read.GET(apiSummaryPath, handler.GetMdadmSummary)                  // used by Dashboard
	mdadm.collector.POST("/array/:uuid/metrics", handler.UploadMdadmMetrics) // used by Collector to upload metrics
	mdadm.read.GET("/array/:uuid/details", handler.GetMdadmArrayDetails)     // used by Array Details view
}

func (ae *AppEngine) registerMiddleware(r *gin.Engine, logger *logrus.Entry) {
	r.Use(middleware.LoggerMiddleware(logger))
	r.Use(middleware.ForwardedForWarningMiddleware(logger))
	r.Use(middleware.BodyLimitMiddleware(ae.Config.GetInt64("web.listen.max_body_bytes")))
	r.Use(middleware.RepositoryMiddleware(ae.Config, logger))
	r.Use(middleware.ConfigMiddleware(ae.Config))
	r.Use(middleware.AuthMiddleware(ae.Config, logger))

	if ae.NotificationGate != nil {
		r.Use(middleware.NotificationGateMiddleware(ae.NotificationGate))
	}
	if ae.MissedPingMonitor != nil {
		r.Use(middleware.MissedPingMonitorMiddleware(ae.MissedPingMonitor))
	}
	if ae.ReportScheduler != nil {
		r.Use(middleware.ReportSchedulerMiddleware(ae.ReportScheduler))
	}

	if ae.Config.GetBool(configKeyMetricsEnabled) {
		if ae.MetricsCollector == nil {
			ae.MetricsCollector = metrics.NewCollector(logger)
		}
		r.Use(middleware.MetricsMiddleware(ae.MetricsCollector))
		logger.Info("Prometheus metrics endpoint enabled")
	} else {
		logger.Info("Prometheus metrics endpoint disabled")
	}

	if ae.Config.GetBool(configKeyMqttEnabled) {
		if ae.MqttPublisher == nil {
			ae.MqttPublisher = mqtt.NewPublisher(ae.Config, logger)
		}
		// Always register the reconnect hook before connecting so the initial
		// connection and every automatic reconnect triggers a discovery re-sync.
		ae.MqttPublisher.SetOnReconnected(ae.loadInitialMqttData)
		if err := ae.MqttPublisher.Connect(); err != nil {
			logger.Warnf("MQTT: initial connect failed: %v (paho keeps retrying in the background)", err)
		} else {
			logger.Info("MQTT Home Assistant integration enabled")
		}
		// Keep the middleware registered even when the initial connect failed:
		// once paho reconnects, device state/discovery publishing resumes.
		r.Use(middleware.MqttPublisherMiddleware(ae.MqttPublisher))
	}

	r.Use(gin.Recovery())
}

func (ae *AppEngine) Setup(logger *logrus.Entry) *gin.Engine {
	// Register additional MIME types for proper file serving
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".mjs", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".ttf", "font/ttf")
	_ = mime.AddExtensionType(".eot", "application/vnd.ms-fontobject")
	_ = mime.AddExtensionType(".otf", "font/otf")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".json", "application/json")

	r := gin.New()
	// Trust X-Forwarded-For only from the configured proxies (default: none), so a client cannot pick
	// the IP that the login rate limiter and the logs see.
	configureTrustedProxies(r, ae.Config.GetStringSlice(configKeyTrustedProxies), logger)
	ae.registerMiddleware(r, logger)

	basePath := ae.Config.GetString("web.listen.basepath")
	logger.Debugf("basepath: %s", basePath)

	base := r.Group(basePath)
	{
		registerAPIRoutes(base.Group("/api"), ae.Config.GetBool(configKeyMetricsEnabled), ae.Config.GetBool(config.WebZFSAllowPoolModificationsKey))
	}

	docsPath := resolveAPIDocsPath(actualDocsBasePath(ae.Config.GetString("web.src.frontend.path")))
	if docsPath != "" {
		swaggerUIPath := filepath.Join(docsPath, swaggerUIFile)
		openAPIPath := filepath.Join(docsPath, openAPIFile)

		base.GET("/docs/api", func(c *gin.Context) {
			c.File(swaggerUIPath)
		})
		base.GET("/docs/api/", func(c *gin.Context) {
			c.File(swaggerUIPath)
		})
		base.GET("/api/docs/openapi.yaml", func(c *gin.Context) {
			c.File(openAPIPath)
		})
		logger.Infof("Serving API docs from %s", docsPath)
	} else {
		logger.Warn("API docs not found on disk; /docs/api and /api/docs/openapi.yaml will not be available")
	}

	//Static request routing
	// Determine the actual frontend path - check if browser/ subdirectory exists
	frontendPath := ae.Config.GetString("web.src.frontend.path")
	browserPath := filepath.Join(frontendPath, "browser")
	indexPath := filepath.Join(browserPath, indexFile)

	// Use browser subdirectory if it exists, otherwise use the configured path directly
	actualFrontendPath := frontendPath
	if utils.FileExists(indexPath) {
		actualFrontendPath = browserPath
		logger.Debugf("Serving frontend from browser subdirectory: %s", actualFrontendPath)
	} else {
		logger.Debugf("Serving frontend from configured path: %s", actualFrontendPath)
	}

	// Create file server - it will automatically use the MIME types registered globally above
	fileServer := http.FileServer(http.Dir(actualFrontendPath))

	// Serve static files with proper MIME types and SPA routing support
	base.GET("/web", func(c *gin.Context) {
		c.File(filepath.Join(actualFrontendPath, indexFile))
	})

	base.GET("/web/*filepath", func(c *gin.Context) {
		file := c.Param("filepath")
		if file == "" || file == "/" {
			c.File(filepath.Join(actualFrontendPath, indexFile))
			return
		}

		// Remove leading slash if present
		if strings.HasPrefix(file, "/") {
			file = file[1:]
		}

		// Check if file exists
		fullPath := filepath.Join(actualFrontendPath, file)
		if !utils.FileExists(fullPath) {
			// For SPA routing, serve index.html for non-existent files
			c.File(filepath.Join(actualFrontendPath, indexFile))
			return
		}

		// Serve the file using the file server
		// MIME type will be automatically set based on registered types above
		c.Request.URL.Path = "/" + file
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	//redirect base url to /web
	base.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, basePath+"/web")
	})

	//catch-all, serve index page for any unmatched routes
	r.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(actualFrontendPath, indexFile))
	})
	return r
}

// configureTrustedProxies sets which peers may supply X-Forwarded-For. An empty list trusts none, and an
// invalid list falls back to trusting none.
func configureTrustedProxies(r *gin.Engine, proxies []string, logger *logrus.Entry) {
	if len(proxies) == 0 {
		proxies = nil
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		logger.Errorf("Invalid %s, trusting no proxies: %v", configKeyTrustedProxies, err)
		_ = r.SetTrustedProxies(nil)
	}
}

// validateTrustedProxies rejects entries that are neither an IP address nor a CIDR range.
func validateTrustedProxies(proxies []string) error {
	for _, p := range proxies {
		if net.ParseIP(p) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(p); err != nil {
			return errors.ConfigValidationError(fmt.Sprintf("%#q entry %q is not an IP address or CIDR range", configKeyTrustedProxies, p))
		}
	}
	return nil
}

func actualDocsBasePath(frontendPath string) string {
	return filepath.Dir(frontendPath)
}

func resolveAPIDocsPath(baseDir string) string {
	candidates := []string{
		filepath.Join(baseDir, apiDocsDirName),
		filepath.Join("/opt", "scrutiny", apiDocsDirName), //nolint:gocritic
		apiDocsDirName,
	}

	for _, candidate := range candidates {
		if utils.FileExists(filepath.Join(candidate, swaggerUIFile)) && utils.FileExists(filepath.Join(candidate, openAPIFile)) {
			return candidate
		}
	}

	return ""
}

// MigrateDatabase runs the database migrations. Setup opens repositories without migrating, so Start
// calls this before Setup, and tests that call Setup directly must call it first as well.
func (ae *AppEngine) MigrateDatabase(logger logrus.FieldLogger) error {
	migrationRepo, err := database.NewScrutinyRepository(ae.Config, logger)
	if err != nil {
		return err
	}
	if err := migrationRepo.Close(); err != nil {
		logger.Warnf("Failed to close migration repository: %v", err)
	}
	return nil
}

func (ae *AppEngine) Start() error {
	//set the gin mode
	gin.SetMode(gin.ReleaseMode)
	if strings.ToLower(ae.Config.GetString("log.level")) == "debug" {
		gin.SetMode(gin.DebugMode)
	}

	if err := validateTrustedProxies(ae.Config.GetStringSlice(configKeyTrustedProxies)); err != nil {
		return err
	}

	// Resolve auth secrets first: hub/spoke without a token must fail before anything else starts.
	if err := auth.EnsureStartupAuth(ae.Config, ae.Logger, auth.HasLocalCollector()); err != nil {
		return err
	}

	// check if the SQLite database parent directory exists, fail here rather than in a handler.
	if ae.Config.GetString("web.database.type") != "postgres" && !utils.FileExists(filepath.Dir(ae.Config.GetString("web.database.location"))) {
		return errors.ConfigValidationError(fmt.Sprintf(
			"Database parent directory does not exist. Please check path (%s)",
			filepath.Dir(ae.Config.GetString("web.database.location"))))
	}

	if err := ae.MigrateDatabase(ae.Logger); err != nil {
		return err
	}

	// Elect the replica that runs background jobs before any of them start ticking.
	leaderRepo, err := database.NewScrutinyRepositoryWithoutMigration(ae.Config, ae.Logger)
	if err != nil {
		return err
	}
	ae.leaderRepo = leaderRepo
	ae.Leader = leader.New(leaderRepo, ae.Logger, leader.SchedulerLeaseName, leader.DefaultTTL)
	ae.Leader.Start()

	// Create notification gate and monitors BEFORE Setup() so middleware can register them in gin context
	ae.NotificationGate = notify.NewNotificationGate(ae.Logger)
	ae.NotificationGate.UseOutbox(leaderRepo, ae.isLeader)
	ae.OutboxWorker = NewNotificationOutboxWorker(ae, leaderRepo)
	ae.OutboxWorker.Start()

	missedPingMonitor := NewMissedPingMonitor(ae)
	ae.MissedPingMonitor = missedPingMonitor

	reportScheduler := reports.NewScheduler(ae.Config, ae.Logger, func() (database.DeviceRepo, error) {
		return database.NewScrutinyRepositoryWithoutMigration(ae.Config, ae.Logger)
	})
	reportScheduler.SetLeaderCheck(ae.isLeader)
	ae.ReportScheduler = reportScheduler

	r := ae.Setup(ae.Logger)

	// Start background monitors after router is set up
	missedPingMonitor.Start()
	ae.Logger.Info("Missed ping monitor started")

	heartbeatMonitor := NewHeartbeatMonitor(ae)
	ae.HeartbeatMonitor = heartbeatMonitor
	heartbeatMonitor.Start()
	ae.Logger.Info("Heartbeat monitor started")

	uptimeKumaMonitor := NewUptimeKumaMonitor(ae)
	ae.UptimeKumaMonitor = uptimeKumaMonitor
	uptimeKumaMonitor.Start()
	ae.Logger.Info("Uptime Kuma monitor started")

	reportScheduler.Start()
	ae.Logger.Info("Report scheduler started")

	ae.loadInitialMetrics()

	// Create HTTP server for graceful shutdown support
	addr := fmt.Sprintf("%s:%s", ae.Config.GetString("web.listen.host"), ae.Config.GetString("web.listen.port"))

	readTimeout := ae.Config.GetInt("web.listen.read_timeout_seconds")
	writeTimeout := ae.Config.GetInt("web.listen.write_timeout_seconds")
	idleTimeout := ae.Config.GetInt("web.listen.idle_timeout_seconds")
	ae.Logger.Infof("HTTP server timeouts: read=%ds, write=%ds, idle=%ds", readTimeout, writeTimeout, idleTimeout)

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(readTimeout) * time.Second,
		WriteTimeout: time.Duration(writeTimeout) * time.Second,
		IdleTimeout:  time.Duration(idleTimeout) * time.Second,
	}

	// Channel to receive shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		ae.Logger.Infof("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ae.Logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-quit
	ae.Logger.Info("Shutdown signal received, initiating graceful shutdown...")

	ae.stopBackgroundMonitors()

	// Create a deadline for shutdown (give 30 seconds for graceful shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown the HTTP server gracefully
	if err := srv.Shutdown(ctx); err != nil {
		ae.Logger.Errorf("Server forced to shutdown: %v", err)
		return err
	}

	ae.Logger.Info("Server shutdown complete")
	return nil
}

func (ae *AppEngine) loadInitialMetrics() {
	if !ae.Config.GetBool(configKeyMetricsEnabled) || ae.MetricsCollector == nil {
		return
	}
	go func() {
		deviceRepo, err := database.NewScrutinyRepositoryWithoutMigration(ae.Config, ae.Logger)
		if err != nil {
			ae.Logger.Errorln("Failed to create repository for loading metrics:", err)
			return
		}
		defer deviceRepo.Close()

		if err := ae.MetricsCollector.LoadInitialData(deviceRepo, context.Background()); err != nil {
			ae.Logger.Errorln("Failed to load initial metrics data:", err)
		}
	}()
}

func (ae *AppEngine) loadInitialMqttData() {
	if !ae.Config.GetBool(configKeyMqttEnabled) || ae.MqttPublisher == nil {
		return
	}
	go func() {
		deviceRepo, err := database.NewScrutinyRepositoryWithoutMigration(ae.Config, ae.Logger)
		if err != nil {
			ae.Logger.Errorln("Failed to create repository for loading MQTT data:", err)
			return
		}
		defer deviceRepo.Close()

		if err := ae.MqttPublisher.LoadInitialData(deviceRepo, context.Background()); err != nil {
			ae.Logger.Errorln("Failed to load initial MQTT data:", err)
		}
	}()
}

func (ae *AppEngine) stopBackgroundMonitors() {
	if ae.MqttPublisher != nil {
		ae.MqttPublisher.Disconnect()
	}
	if ae.MissedPingMonitor != nil {
		ae.MissedPingMonitor.Stop()
	}
	if ae.HeartbeatMonitor != nil {
		ae.HeartbeatMonitor.Stop()
	}
	if ae.UptimeKumaMonitor != nil {
		ae.UptimeKumaMonitor.Stop()
	}
	if ae.ReportScheduler != nil {
		ae.ReportScheduler.Stop()
	}
	if ae.OutboxWorker != nil {
		ae.OutboxWorker.Stop()
	}
	// Release the lease only after every job has stopped, so no two replicas run jobs at once.
	if ae.Leader != nil {
		ae.Leader.Stop()
	}
	if ae.leaderRepo != nil {
		_ = ae.leaderRepo.Close()
	}
}
