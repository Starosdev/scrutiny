package handler

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/analogj/scrutiny/collector/pkg/omnibus"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const collectorRunTimeout = 5 * time.Minute

var (
	// collectorRunning is true for the whole background run and is the only run lock. A second
	// request while it is set gets 409 instead of starting a parallel run. The status endpoint
	// reads it directly.
	collectorRunning atomic.Bool

	// seams for tests
	collectorLookPath = exec.LookPath
	collectorExec     = func(ctx context.Context, path string, args []string) ([]byte, error) {
		return exec.CommandContext(ctx, path, args...).CombinedOutput()
	}
	collectorGap = time.Second
)

// manualCollectors is the order in which a manual run executes the local collectors.
var manualCollectors = []string{
	"scrutiny-collector-metrics",
	"scrutiny-collector-zfs",
	"scrutiny-collector-mdadm",
}

// TriggerCollectors runs enabled local SMART, ZFS, and MDADM collectors sequentially.
func TriggerCollectors(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	// Check if the primary collector binary exists
	if _, execErr := collectorLookPath(manualCollectors[0]); execErr != nil {
		logger.Warnf("Manually triggered collectors, but %s not found in PATH", manualCollectors[0])
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Collector binaries not found on this system"})
		return
	}

	if !collectorRunning.CompareAndSwap(false, true) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A collector run is already in progress"})
		return
	}

	go func() {
		defer collectorRunning.Store(false)
		runCollectorsSequentially(logger)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Collectors triggered successfully",
	})
}

// GetCollectorRunStatus reports whether a manual collector run is in progress, so the dashboard
// can wait for the run to finish before it reloads.
func GetCollectorRunStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "running": collectorRunning.Load()})
}

func runCollectorsSequentially(logger *logrus.Entry) {
	logger.Info("Starting manual sequential collector run")

	ran := false
	for _, bin := range manualCollectors {
		if !manualCollectorEnabled(bin) {
			logger.Infof("Collector %s disabled, skipping manual run", bin)
			continue
		}
		path, err := collectorLookPath(bin)
		if err != nil {
			logger.Debugf("Collector binary %s not found, skipping", bin)
			continue
		}

		// Pause between collectors only. A pause after the last one would hold the run lock for nothing.
		if ran {
			time.Sleep(collectorGap)
		}
		ran = true

		logger.Infof("Executing collector: %s", bin)

		// Timeout prevents hanging processes from holding the run lock forever.
		ctx, cancel := context.WithTimeout(context.Background(), collectorRunTimeout)
		output, err := collectorExec(ctx, path, omnibus.OneShotArgs(bin))
		cancel()
		if err != nil {
			logger.Errorf("Collector %s failed: %v\nOutput: %s", bin, err, string(output))
		} else {
			logger.Infof("Collector %s completed successfully", bin)
		}
	}

	logger.Info("Manual sequential collector run finished")
}

// manualCollectorEnabled follows the all-in-one image's optional collector gate.
// A false startup flag suppresses startup only; a schedule still enables manual runs.
// COLLECTOR_*_ENABLED (spec.EnabledEnv) is deliberately not read: only the standalone
// omnibus manager honors it, and that manager never runs alongside this handler (#1116).
func manualCollectorEnabled(binaryName string) bool {
	for _, spec := range omnibus.CollectorSpecs {
		if spec.BinaryName != binaryName {
			continue
		}
		// Ignore env-file quoting and padding when checking for a schedule.
		schedule := strings.Trim(os.Getenv(spec.ScheduleEnv), "\"' \t\r\n")
		if spec.Name == "metrics" || schedule != "" {
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
