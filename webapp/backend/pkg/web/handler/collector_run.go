package handler

import (
	"context"
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// TriggerCollectors triggers all local collector binaries sequentially in the background
func TriggerCollectors(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	// Check if the primary collector binary exists
	_, execErr := exec.LookPath("scrutiny-collector-metrics")
	if execErr != nil {
		logger.Warn("Manually triggered collectors, but scrutiny-collector-metrics not found in PATH")
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Collector binaries not found on this system"})
		return
	}

	// Run collectors in the background sequentially
	go func() {
		// Use a detached context for background execution
		bgCtx := context.Background()
		collectors := []string{
			"scrutiny-collector-metrics",
			"scrutiny-collector-zfs",
			"scrutiny-collector-mdadm",
			"scrutiny-collector-performance",
		}

		logger.Info("Starting manual sequential collector run")

		for _, bin := range collectors {
			// Check if binary exists before trying to run it
			path, err := exec.LookPath(bin)
			if err != nil {
				logger.Debugf("Collector binary %s not found, skipping", bin)
				continue
			}

			logger.Infof("Executing collector: %s", bin)
			
			// Create command with a timeout to prevent hanging processes
			ctx, cancel := context.WithTimeout(bgCtx, 5*time.Minute)
			cmd := exec.CommandContext(ctx, path, "run")
			
			// We don't capture stdout/stderr here to keep it simple, 
			// but we log the result. Collectors log to their own destinations usually.
			output, err := cmd.CombinedOutput()
			if err != nil {
				logger.Errorf("Collector %s failed: %v\nOutput: %s", bin, err, string(output))
			} else {
				logger.Infof("Collector %s completed successfully", bin)
			}
			cancel()
			
			// Small buffer between collectors
			time.Sleep(1 * time.Second)
		}
		
		logger.Info("Manual sequential collector run finished")
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Collectors triggered successfully",
	})
}
