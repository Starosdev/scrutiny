package handler

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/analogj/scrutiny/collector/pkg/omnibus"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// TriggerCollectors runs enabled local SMART, ZFS, and MDADM collectors sequentially.
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
		}

		logger.Info("Starting manual sequential collector run")

		for _, bin := range collectors {
			if !manualCollectorEnabled(bin) {
				logger.Infof("Collector %s disabled, skipping manual run", bin)
				continue
			}
			// Check if binary exists before trying to run it
			path, err := exec.LookPath(bin)
			if err != nil {
				logger.Debugf("Collector binary %s not found, skipping", bin)
				continue
			}

			logger.Infof("Executing collector: %s", bin)

			// Create command with a timeout to prevent hanging processes
			ctx, cancel := context.WithTimeout(bgCtx, 5*time.Minute)
			cmd := exec.CommandContext(ctx, path, omnibus.OneShotArgs(bin)...)

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

// manualCollectorEnabled follows the all-in-one image's optional collector gate.
// A false startup flag suppresses startup only; a schedule still enables manual runs.
func manualCollectorEnabled(binaryName string) bool {
	for _, spec := range omnibus.CollectorSpecs {
		if spec.BinaryName != binaryName {
			continue
		}
		if spec.Name == "metrics" || os.Getenv(spec.ScheduleEnv) != "" {
			return true
		}
		// Match /etc/scrutiny/startup-gate.sh, including quoted env-file values.
		startup := strings.Map(func(r rune) rune {
			if r == '\'' || r == '"' || unicode.IsSpace(r) {
				return -1
			}
			return unicode.ToLower(r)
		}, os.Getenv(spec.RunStartupEnv))
		enabled, _ := strconv.ParseBool(startup)
		return enabled
	}
	return false
}
