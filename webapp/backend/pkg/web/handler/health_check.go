package handler

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/analogj/go-util/utils"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func HealthCheck(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	appConfig := c.MustGet("CONFIG").(config.Interface)
	logger.Infof("Checking Influxdb & Sqlite health")

	// Check sqlite and influxdb health with detailed status
	healthResult, err := deviceRepo.HealthCheck(c)

	// Check if the /web folder is populated with expected frontend files
	frontendPath := appConfig.GetString("web.src.frontend.path")
	indexPath := filepath.Join(frontendPath, "index.html")
	frontendOk := utils.FileExists(indexPath)

	// Add frontend check to the health result
	if healthResult != nil {
		if frontendOk {
			healthResult.Checks["frontend"] = database.HealthCheckStatus{
				Status:    "ok",
				LatencyMs: 0,
			}
		} else {
			healthResult.Status = "unhealthy"
			healthResult.Checks["frontend"] = database.HealthCheckStatus{
				Status:    "error",
				LatencyMs: 0,
				Error:     fmt.Sprintf("Frontend files not found. Expected index.html at: %s", indexPath),
			}
		}
	}

	if err != nil || !frontendOk {
		logger.Errorln("An error occurred during healthcheck", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"status":  healthResult.Status,
			"checks":  healthResult.Checks,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  healthResult.Status,
		"checks":  healthResult.Checks,
	})
}
