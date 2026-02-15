package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/detect"
	"github.com/analogj/scrutiny/collector/pkg/errors"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

// Collector handles performance benchmarking via fio
type Collector struct {
	config      config.Interface
	logger      *logrus.Entry
	apiEndpoint *url.URL
	httpClient  *http.Client
}

// CreateCollector creates a new performance collector
func CreateCollector(appConfig config.Interface, logger *logrus.Entry, apiEndpoint string) (*Collector, error) {
	apiEndpointUrl, err := url.Parse(apiEndpoint)
	if err != nil {
		return nil, err
	}

	timeout := 300 // longer timeout for benchmarks
	if appConfig != nil && appConfig.IsSet("api.timeout") {
		timeout = appConfig.GetAPITimeout()
	}

	collector := &Collector{
		config:      appConfig,
		logger:      logger,
		apiEndpoint: apiEndpointUrl,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}

	return collector, nil
}

// Run executes the performance benchmark collection
func (c *Collector) Run() error {
	c.logger.Infoln("Starting performance benchmark collection")

	if err := c.Validate(); err != nil {
		return err
	}

	// Detect devices using same mechanism as metrics collector
	deviceDetector := detect.Detect{
		Logger: c.logger,
		Config: c.config,
	}
	rawDevices, err := deviceDetector.Start()
	if err != nil {
		return err
	}

	validDevices := lo.Filter[models.Device](rawDevices, func(dev models.Device, _ int) bool {
		return len(dev.WWN) > 0
	})

	if len(validDevices) == 0 {
		c.logger.Infoln("No devices found")
		return nil
	}

	c.logger.Infof("Found %d device(s)", len(validDevices))

	// Register devices with API
	registeredDevices, err := c.RegisterDevices(validDevices)
	if err != nil {
		return err
	}

	profile := c.config.GetString("performance.profile")
	c.logger.Infof("Using benchmark profile: %s", profile)

	// Benchmark each registered device
	for i := range registeredDevices {
		result, err := c.Benchmark(&registeredDevices[i], profile)
		if err != nil {
			c.logger.Errorf("Failed to benchmark device %s (%s): %v", registeredDevices[i].DeviceName, registeredDevices[i].WWN, err)
			continue
		}

		if err := c.Publish(registeredDevices[i].WWN, result); err != nil {
			c.logger.Errorf("Failed to publish results for %s: %v", registeredDevices[i].WWN, err)
		}
	}

	c.logger.Infoln("Performance benchmark collection completed")
	return nil
}

// Validate checks that fio is available
func (c *Collector) Validate() error {
	c.logger.Infoln("Verifying required tools")

	fioBin := c.config.GetString("commands.performance_fio_bin")
	_, err := exec.LookPath(fioBin)
	if err != nil {
		return errors.DependencyMissingError(fmt.Sprintf("%s binary is missing", fioBin))
	}

	return nil
}

// RegisterDevices registers detected devices with the API and returns the filtered list
func (c *Collector) RegisterDevices(devices []models.Device) ([]models.Device, error) {
	c.logger.Infoln("Sending detected devices to API for registration")

	apiEndpoint, _ := url.Parse(c.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse("api/devices/register")

	wrapper := models.DeviceWrapper{Data: devices}
	jsonData, err := json.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal devices: %w", err)
	}

	resp, err := c.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to register devices: %w", err)
	}
	defer resp.Body.Close()

	var responseWrapper models.DeviceWrapper
	if err := json.NewDecoder(resp.Body).Decode(&responseWrapper); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !responseWrapper.Success {
		return nil, errors.ApiServerCommunicationError("An error occurred while registering devices")
	}

	return responseWrapper.Data, nil
}

// Benchmark runs fio tests against a device and returns aggregated results
func (c *Collector) Benchmark(device *models.Device, profile string) (*models.PerformanceResult, error) {
	c.logger.Infof("Benchmarking device %s (%s)", device.DeviceName, device.WWN)

	fioBin := c.config.GetString("commands.performance_fio_bin")
	targetPath, cleanup, err := c.resolveTargetPath(device)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve benchmark target: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	result := &models.PerformanceResult{
		Date:           time.Now().Unix(),
		Profile:        profile,
		DeviceProtocol: device.DeviceProtocol,
	}

	// Get fio version
	versionOut, err := exec.Command(fioBin, "--version").Output()
	if err == nil {
		result.FioVersion = strings.TrimSpace(string(versionOut))
	}

	startTime := time.Now()

	// Sequential read test
	c.logger.Debugf("Running sequential read test on %s", device.DeviceName)
	seqReadOut, err := c.runFio(fioBin, c.buildFioArgs("read", "1M", profile, targetPath))
	if err != nil {
		c.logger.Warnf("Sequential read test failed for %s: %v", device.DeviceName, err)
	} else {
		if fioResult, parseErr := models.ParseFioOutput(seqReadOut); parseErr == nil && len(fioResult.Jobs) > 0 {
			result.SeqReadBwBytes, _, _, _, _, _ = models.ExtractReadStats(&fioResult.Jobs[0])
		}
	}

	// Sequential write test
	c.logger.Debugf("Running sequential write test on %s", device.DeviceName)
	seqWriteOut, err := c.runFio(fioBin, c.buildFioArgs("write", "1M", profile, targetPath))
	if err != nil {
		c.logger.Warnf("Sequential write test failed for %s: %v", device.DeviceName, err)
	} else {
		if fioResult, parseErr := models.ParseFioOutput(seqWriteOut); parseErr == nil && len(fioResult.Jobs) > 0 {
			result.SeqWriteBwBytes, _, _, _, _, _ = models.ExtractWriteStats(&fioResult.Jobs[0])
		}
	}

	// Random read test (IOPS + latency)
	c.logger.Debugf("Running random read test on %s", device.DeviceName)
	randReadOut, err := c.runFio(fioBin, c.buildFioArgs("randread", "4K", profile, targetPath))
	if err != nil {
		c.logger.Warnf("Random read test failed for %s: %v", device.DeviceName, err)
	} else {
		if fioResult, parseErr := models.ParseFioOutput(randReadOut); parseErr == nil && len(fioResult.Jobs) > 0 {
			_, result.RandReadIOPS, result.RandReadLatAvgNs, result.RandReadLatP50Ns, result.RandReadLatP95Ns, result.RandReadLatP99Ns = models.ExtractReadStats(&fioResult.Jobs[0])
		}
	}

	// Random write test (IOPS + latency)
	c.logger.Debugf("Running random write test on %s", device.DeviceName)
	randWriteOut, err := c.runFio(fioBin, c.buildFioArgs("randwrite", "4K", profile, targetPath))
	if err != nil {
		c.logger.Warnf("Random write test failed for %s: %v", device.DeviceName, err)
	} else {
		if fioResult, parseErr := models.ParseFioOutput(randWriteOut); parseErr == nil && len(fioResult.Jobs) > 0 {
			_, result.RandWriteIOPS, result.RandWriteLatAvgNs, result.RandWriteLatP50Ns, result.RandWriteLatP95Ns, result.RandWriteLatP99Ns = models.ExtractWriteStats(&fioResult.Jobs[0])
		}
	}

	// Mixed random R/W test (comprehensive profile only)
	if profile == "comprehensive" {
		c.logger.Debugf("Running mixed random R/W test on %s", device.DeviceName)
		mixedArgs := c.buildFioArgs("randrw", "4K", profile, targetPath)
		mixedArgs = append(mixedArgs, "--rwmixread=70")
		mixedOut, err := c.runFio(fioBin, mixedArgs)
		if err != nil {
			c.logger.Warnf("Mixed R/W test failed for %s: %v", device.DeviceName, err)
		} else if fioResult, err := models.ParseFioOutput(mixedOut); err == nil && len(fioResult.Jobs) > 0 {
			result.MixedRwIOPS = fioResult.Jobs[0].Read.IOPS + fioResult.Jobs[0].Write.IOPS
		}
	}

	result.TestDurationSec = time.Since(startTime).Seconds()
	c.logger.Infof("Benchmarking complete for %s (%.1fs)", device.DeviceName, result.TestDurationSec)

	return result, nil
}

// Publish sends benchmark results to the API
func (c *Collector) Publish(wwn string, result *models.PerformanceResult) error {
	c.logger.Infof("Publishing performance results for %s", wwn)

	apiEndpoint, _ := url.Parse(c.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse(fmt.Sprintf("api/device/%s/performance", strings.ToLower(wwn)))

	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	resp, err := c.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to publish results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	c.logger.Infof("Successfully published performance results for %s", wwn)
	return nil
}

// resolveTargetPath determines the fio target file path for benchmarking.
// Returns the path, a cleanup function (if a temp file was created), and any error.
func (c *Collector) resolveTargetPath(device *models.Device) (string, func(), error) {
	fullDeviceName := fmt.Sprintf("%s%s", detect.DevicePrefix(), device.DeviceName)

	if c.config.GetBool("performance.allow_direct_device_io") {
		c.logger.Warnf("Direct device I/O enabled -- benchmarking raw device %s (writes are destructive!)", fullDeviceName)
		return fullDeviceName, nil, nil
	}

	// Try to find mount point for the device
	mountPoint, err := findMountPoint(fullDeviceName)
	if err != nil {
		return "", nil, fmt.Errorf("could not find mount point for %s: %w", fullDeviceName, err)
	}

	tempFile := fmt.Sprintf("%s/.scrutiny_perf_bench", mountPoint)
	cleanup := func() {
		os.Remove(tempFile)
	}

	return tempFile, cleanup, nil
}

// buildFioArgs constructs fio command arguments for a given test
func (c *Collector) buildFioArgs(rwMode string, blockSize string, profile string, targetPath string) []string {
	var size, testRuntime, numjobs string

	switch profile {
	case "comprehensive":
		size = "1G"
		testRuntime = "30"
		numjobs = "4"
	default: // "quick"
		size = c.config.GetString("performance.temp_file_size")
		if size == "" {
			size = "256M"
		}
		testRuntime = "10"
		numjobs = "1"
	}

	ioengine := "libaio"
	if runtime.GOOS == "darwin" {
		ioengine = "posixaio"
	}

	args := []string{
		"--name=scrutiny_bench",
		fmt.Sprintf("--rw=%s", rwMode),
		fmt.Sprintf("--bs=%s", blockSize),
		fmt.Sprintf("--size=%s", size),
		fmt.Sprintf("--numjobs=%s", numjobs),
		fmt.Sprintf("--runtime=%s", testRuntime),
		"--time_based",
		"--direct=1",
		fmt.Sprintf("--ioengine=%s", ioengine),
		"--output-format=json",
		"--group_reporting",
		fmt.Sprintf("--filename=%s", targetPath),
	}

	return args
}

// runFio executes a fio command and returns the JSON output
func (c *Collector) runFio(fioBin string, args []string) ([]byte, error) {
	c.logger.Debugf("Executing: %s %s", fioBin, strings.Join(args, " "))

	cmd := exec.Command(fioBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("fio command failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// findMountPoint finds the mount point for a given device path
func findMountPoint(devicePath string) (string, error) {
	// Use df to find the mount point
	cmd := exec.Command("df", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("df command failed for %s: %w", devicePath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected df output for %s", devicePath)
	}

	// The mount point is the last field in the second line
	fields := strings.Fields(lines[1])
	if len(fields) == 0 {
		return "", fmt.Errorf("could not parse df output for %s", devicePath)
	}

	return fields[len(fields)-1], nil
}
