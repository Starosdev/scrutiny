package middleware

import (
	"context"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func RepositoryMiddleware(appConfig config.Interface, globalLogger logrus.FieldLogger) gin.HandlerFunc {

	maxRetries := 30
	retryInterval := 10 * time.Second

	var deviceRepo database.DeviceRepo
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		deviceRepo, err = database.NewScrutinyRepository(appConfig, globalLogger)
		if err == nil {
			break
		}
		if attempt < maxRetries {
			globalLogger.Warnf("Database initialization failed (attempt %d/%d): %v. Retrying in %s...",
				attempt, maxRetries, err, retryInterval)
			time.Sleep(retryInterval)
		}
	}
	if err != nil {
		globalLogger.Fatalf("Failed to initialize database after %d attempts (%s): %v",
			maxRetries, time.Duration(maxRetries)*retryInterval, err)
	}

	// ensure the settings have been loaded into the app config during startup.
	_, err = deviceRepo.LoadSettings(context.Background())
	if err != nil {
		globalLogger.Fatalf("Failed to load settings from database: %v", err)
	}

	//TODO: determine where we can call defer deviceRepo.Close()
	return func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", deviceRepo)
		c.Next()
	}
}
