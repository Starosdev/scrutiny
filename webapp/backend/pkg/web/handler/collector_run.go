package handler

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/analogj/scrutiny/collector/pkg/omnibus"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const collectorRunTimeout = 5 * time.Minute

var (
	// collectorRunMu is held for the whole background run. A second request while it is held
	// gets 409 instead of joining: runs take minutes, and a rejected caller can simply retry.
	collectorRunMu sync.Mutex

	// seams for tests
	collectorLookPath = exec.LookPath
	collectorExec     = func(ctx context.Context, path string, args []string) ([]byte, error) {
		return exec.CommandContext(ctx, path, args...).CombinedOutput()
	}
	collectorGap = time.Second
)

// TriggerCollectors runs the enabled local collector binaries sequentially in the background.
// The binary list is shared with the omnibus manager (omnibus.CollectorSpecs); SMART always runs,
// and the optional collectors run only when the all-in-one image would enable them.
func TriggerCollectors(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	// Check if the primary collector binary exists
	if _, execErr := collectorLookPath(omnibus.CollectorSpecs[0].BinaryName); execErr != nil {
		logger.Warnf("Manually triggered collectors, but %s not found in PATH", omnibus.CollectorSpecs[0].BinaryName)
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Collector binaries not found on this system"})
		return
	}

	if !collectorRunMu.TryLock() {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A collector run is already in progress"})
		return
	}

	go func() {
		defer collectorRunMu.Unlock()
		runCollectorsSequentially(logger)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Collectors triggered successfully",
	})
}

func runCollectorsSequentially(logger *logrus.Entry) {
	logger.Info("Starting manual sequential collector run")

	for _, spec := range omnibus.CollectorSpecs {
		path, err := collectorLookPath(spec.BinaryName)
		if err != nil {
			logger.Debugf("Collector binary %s not found, skipping", spec.BinaryName)
			continue
		}
		if !manualCollectorEnabled(spec.BinaryName) {
			logger.Infof("Collector %s disabled, skipping manual run", spec.BinaryName)
			continue
		}

		logger.Infof("Executing collector: %s", spec.BinaryName)

		// Timeout prevents hanging processes from holding the run lock forever.
		ctx, cancel := context.WithTimeout(context.Background(), collectorRunTimeout)
		output, err := collectorExec(ctx, path, omnibus.OneShotArgs(spec.BinaryName))
		cancel()
		if err != nil {
			logger.Errorf("Collector %s failed: %v\nOutput: %s", spec.BinaryName, err, string(output))
		} else {
			logger.Infof("Collector %s completed successfully", spec.BinaryName)
		}

		time.Sleep(collectorGap)
	}

	logger.Info("Manual sequential collector run finished")
}

// manualCollectorEnabled follows the all-in-one image's optional collector gate.
// A false startup flag suppresses startup only; a schedule still enables manual runs.
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
