package web

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/analogj/go-util/utils"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/errors"
	"github.com/analogj/scrutiny/webapp/backend/pkg/metrics"
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
const indexFile = "index.html"

type AppEngine struct {
	Config             config.Interface
	Logger             *logrus.Entry
	MetricsCollector   *metrics.Collector
	MqttPublisher      *mqtt.Publisher
	NotificationGate   *notify.NotificationGate
	MissedPingMonitor  *MissedPingMonitor
	HeartbeatMonitor   *HeartbeatMonitor
	UptimeKumaMonitor  *UptimeKumaMonitor
	ReportScheduler    *reports.Scheduler
}

func (ae *AppEngine) registerMiddleware(r *gin.Engine, logger *logrus.Entry) {
	r.Use(middleware.LoggerMiddleware(logger))
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
		if err := ae.MqttPublisher.Connect(); err != nil {
			logger.Errorf("Failed to connect MQTT: %v (MQTT integration disabled)", err)
			ae.MqttPublisher = nil
		} else {
			r.Use(middleware.MqttPublisherMiddleware(ae.MqttPublisher))
			logger.Info("MQTT Home Assistant integration enabled")
		}
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
	ae.registerMiddleware(r, logger)

	basePath := ae.Config.GetString("web.listen.basepath")
	logger.Debugf("basepath: %s", basePath)

	base := r.Group(basePath)
	{
		api := base.Group("/api")
		{
			// Auth endpoints (always public, checked by middleware)
			api.GET("/auth/status", handler.AuthStatus)
			api.POST("/auth/login", handler.Login)

			api.GET("/health", handler.HealthCheck)
			api.HEAD("/health", handler.HealthCheck)
			api.POST("/health/notify", handler.SendTestNotification)        //check if notifications are configured correctly
			api.GET("/health/missed-ping-status", handler.GetMissedPingStatus) //get missed ping monitor diagnostic status
			api.POST("/health/uptime-kuma-test", handler.TestUptimeKumaPush) // test Uptime Kuma push monitor
			api.POST("/health/mqtt-sync", handler.MqttSync)                 // re-sync all MQTT discovery entities with HA

			api.POST("/devices/register", handler.RegisterDevices)         //used by Collector to register new devices and retrieve filtered list
			api.GET("/summary", handler.GetDevicesSummary)                 //used by Dashboard
			api.GET("/summary/temp", handler.GetDevicesSummaryTempHistory)       // used by Dashboard (Temperature history dropdown)
			api.GET("/summary/workload", handler.GetWorkloadInsights)           // used by Workload Insights page

			// Prometheus metrics endpoint (only registered if enabled)
			if ae.Config.GetBool(configKeyMetricsEnabled) {
				api.GET("/metrics", handler.GetMetrics)
			}

			api.POST("/device/:id/smart", handler.UploadDeviceMetrics) // used by Collector to upload data
			api.POST("/device/:id/selftest", handler.UploadDeviceSelfTests)
			api.GET("/device/:id/details", handler.GetDeviceDetails)   // used by Details
			api.POST("/device/:id/archive", handler.ArchiveDevice)     // used by UI to archive device
			api.POST("/device/:id/unarchive", handler.UnarchiveDevice) // used by UI to unarchive device
			api.POST("/device/:id/mute", handler.MuteDevice)           // used by UI to mute device
			api.POST("/device/:id/unmute", handler.UnmuteDevice)       // used by UI to unmute device
			api.POST("/device/:id/reset-status", handler.ResetDeviceStatus) // used by UI to reset device failed status
			api.POST("/device/:id/label", handler.UpdateDeviceLabel)                         // used by UI to set device label
			api.POST("/device/:id/smart-display-mode", handler.UpdateDeviceSmartDisplayMode)       // used by UI to set SMART attribute display mode
			api.POST("/device/:id/missed-ping-timeout", handler.UpdateDeviceMissedPingTimeout) // used by UI to set per-device missed ping timeout override
			api.DELETE("/device/:id", handler.DeleteDevice)                                  // used by UI to delete device
			api.POST("/device/:id/performance", handler.UploadDevicePerformance)            // used by Collector to upload performance benchmarks
			api.GET("/device/:id/performance", handler.GetDevicePerformance)                // used by UI to view performance history
			api.POST("/device/:id/collector-error", handler.UploadCollectorError)           // used by Collector to report smartctl errors
			api.POST("/collector/scan-error", handler.UploadCollectorScanError)             // used by Collector to report scan-level errors (no device context)

			api.GET("/settings", handler.GetSettings)   //used to get settings
			api.POST("/settings", handler.SaveSettings) //used to save settings

			// Attribute Override endpoints (UI-configurable SMART overrides)
			api.GET("/settings/overrides", handler.GetAttributeOverrides)
			api.POST("/settings/overrides", handler.SaveAttributeOverride)
			api.DELETE("/settings/overrides/:id", handler.DeleteAttributeOverride)

			// Notification URL endpoints (UI-configurable notification channels)
			api.GET("/settings/notify-urls", handler.GetNotifyUrls)
			api.POST("/settings/notify-urls", handler.SaveNotifyUrl)
			api.DELETE("/settings/notify-urls/:id", handler.DeleteNotifyUrl)
			api.POST("/settings/notify-urls/:id/test", handler.TestNotifyUrl)

			// Scheduled report endpoints
			api.GET("/reports/generate", handler.GenerateReport)
			api.GET("/reports/history", handler.ListReports)

			// ZFS Pool API endpoints
			zfs := api.Group("/zfs")
			{
				zfs.POST("/pools/register", handler.RegisterZFSPools)        //used by ZFS Collector to register pools
				zfs.GET("/summary", handler.GetZFSPoolsSummary)              //used by ZFS Dashboard
				zfs.POST("/pool/:guid/metrics", handler.UploadZFSPoolMetrics) //used by ZFS Collector to upload metrics
				zfs.GET("/pool/:guid/details", handler.GetZFSPoolDetails)    //used by ZFS Pool Details view
				zfs.POST("/pool/:guid/archive", handler.ArchiveZFSPool)      //used by UI to archive pool
				zfs.POST("/pool/:guid/unarchive", handler.UnarchiveZFSPool)  //used by UI to unarchive pool
				zfs.POST("/pool/:guid/mute", handler.MuteZFSPool)            //used by UI to mute pool
				zfs.POST("/pool/:guid/unmute", handler.UnmuteZFSPool)        //used by UI to unmute pool
				zfs.POST("/pool/:guid/label", handler.UpdateZFSPoolLabel)    //used by UI to set pool label
				zfs.DELETE("/pool/:guid", handler.DeleteZFSPool)             //used by UI to delete pool
			}
		}
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

func (ae *AppEngine) Start() error {
	//set the gin mode
	gin.SetMode(gin.ReleaseMode)
	if strings.ToLower(ae.Config.GetString("log.level")) == "debug" {
		gin.SetMode(gin.DebugMode)
	}

	//check if the database parent directory exists, fail here rather than in a handler.
	if !utils.FileExists(filepath.Dir(ae.Config.GetString("web.database.location"))) {
		return errors.ConfigValidationError(fmt.Sprintf(
			"Database parent directory does not exist. Please check path (%s)",
			filepath.Dir(ae.Config.GetString("web.database.location"))))
	}

	// Create notification gate and monitors BEFORE Setup() so middleware can register them in gin context
	ae.NotificationGate = notify.NewNotificationGate(ae.Logger)

	missedPingMonitor := NewMissedPingMonitor(ae)
	ae.MissedPingMonitor = missedPingMonitor

	reportScheduler := reports.NewScheduler(ae.Config, ae.Logger, func() (database.DeviceRepo, error) {
		return database.NewScrutinyRepository(ae.Config, ae.Logger)
	})
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
	ae.loadInitialMqttData()

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
		deviceRepo, err := database.NewScrutinyRepository(ae.Config, ae.Logger)
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
		deviceRepo, err := database.NewScrutinyRepository(ae.Config, ae.Logger)
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
}
