package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/analogj/scrutiny/collector/pkg/common/shell"
	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/detect"
	"github.com/analogj/scrutiny/collector/pkg/errors"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/sirupsen/logrus"
)

const configKeySmartctlBin = "commands.metrics_smartctl_bin"

type MetricsCollector struct {
	config config.Interface
	BaseCollector
	apiEndpoint *url.URL
	shell       shell.Interface
}

func CreateMetricsCollector(appConfig config.Interface, logger *logrus.Entry, apiEndpoint string) (MetricsCollector, error) {
	apiEndpointUrl, err := url.Parse(apiEndpoint)
	if err != nil {
		return MetricsCollector{}, err
	}

	sc := MetricsCollector{
		config:      appConfig,
		apiEndpoint: apiEndpointUrl,
		BaseCollector: BaseCollector{
			logger:     logger,
			httpClient: NewAuthHTTPClient(appConfig.GetAPITimeout(), appConfig.GetAPIToken()),
		},
		shell: shell.Create(),
	}

	return sc, nil
}

func (mc *MetricsCollector) Run() error {
	err := mc.Validate()
	if err != nil {
		return err
	}

	apiEndpoint, _ := url.Parse(mc.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse("api/devices/register") //this acts like filepath.Join()

	deviceRespWrapper := new(models.DeviceWrapper)

	deviceDetector := detect.Detect{
		Logger: mc.logger,
		Config: mc.config,
	}
	rawDetectedStorageDevices, err := deviceDetector.Start()
	if err != nil {
		mc.ReportScanError("scan", err.Error(), "")
		return err
	}

	mc.logger.Infoln("Sending detected devices to API, for filtering & validation")
	detectedStorageDevices := rawDetectedStorageDevices
	jsonObj, _ := json.Marshal(detectedStorageDevices)
	mc.logger.Debugf("Detected devices: %v", string(jsonObj))
	err = mc.postJson(apiEndpoint.String(), models.DeviceWrapper{
		Data: detectedStorageDevices,
	}, &deviceRespWrapper)
	if err != nil {
		return err
	}

	if !deviceRespWrapper.Success {
		mc.logger.Errorln("An error occurred while retrieving filtered devices")
		mc.logger.Debugln(deviceRespWrapper)
		return errors.ApiServerCommunicationError("An error occurred while retrieving filtered devices")
	} else {
		mc.logger.Debugln(deviceRespWrapper)
		//var wg sync.WaitGroup
		for _, device := range deviceRespWrapper.Data {
			// execute collection in parallel go-routines
			//wg.Add(1)
			//go mc.Collect(&wg, device.WWN, device.DeviceName, device.DeviceType)
			mc.Collect(device.WWN, device.DeviceName, device.DeviceType)

			if mc.config.GetInt("commands.metrics_smartctl_wait") > 0 {
				time.Sleep(time.Duration(mc.config.GetInt("commands.metrics_smartctl_wait")) * time.Second)
			}
		}

		//mc.logger.Infoln("Main: Waiting for workers to finish")
		//wg.Wait()
		mc.logger.Infoln("Main: Completed")
	}

	return nil
}

func (mc *MetricsCollector) Validate() error {
	mc.logger.Infoln("Verifying required tools")
	_, lookErr := exec.LookPath(mc.config.GetString(configKeySmartctlBin))

	if lookErr != nil {
		return errors.DependencyMissingError(fmt.Sprintf("%s binary is missing", mc.config.GetString(configKeySmartctlBin)))
	}

	return nil
}

// func (mc *MetricsCollector) Collect(wg *sync.WaitGroup, deviceWWN string, deviceName string, deviceType string) {
func (mc *MetricsCollector) Collect(deviceWWN string, deviceName string, deviceType string) {
	//defer wg.Done()
	if len(deviceWWN) == 0 {
		mc.logger.Errorf("no device WWN detected for %s. Skipping collection for this device (no data association possible).\n", deviceName)
		mc.ReportScanError("info", fmt.Sprintf("no WWN detected for device %s; smartctl --info may have failed", deviceName), deviceName)
		return
	}
	mc.logger.Infof("Collecting smartctl results for %s\n", deviceName)

	fullDeviceName := detect.DeviceFullPath(deviceName)
	args := strings.Split(mc.config.GetCommandMetricsSmartArgs(fullDeviceName), " ")
	//only include the device type if its a non-standard one. In some cases ata drives are detected as scsi in docker, and metadata is lost.
	if len(deviceType) > 0 && deviceType != "scsi" && deviceType != "ata" {
		args = append(args, "--device", deviceType)
	}
	args = append(args, fullDeviceName)

	timeout := time.Duration(mc.config.GetInt("commands.metrics_smartctl_timeout")) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result, err := mc.shell.CommandContext(ctx, mc.logger, mc.config.GetString(configKeySmartctlBin), args, "", os.Environ())
	resultBytes := []byte(result)
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// smartctl command exited with an error, we should still push the data to the API server
			mc.logger.Errorf("smartctl returned an error code (%d) while processing %s\n", exitError.ExitCode(), deviceName)
			mc.LogSmartctlExitCode(exitError.ExitCode())
			// Bits 0x01 and 0x02 indicate fatal errors where the JSON
			// output should not be trusted:
			//   0x01 = command line parse error
			//   0x02 = device open failed (includes standby)
			// Bit 0x04 (checksum error in response) is intentionally
			// excluded because the JSON data is usually still valid
			// and many drives behind RAID/HBA controllers intermittently
			// return this code.
			exitCode := exitError.ExitCode()
			if exitCode&0x04 != 0 {
				mc.logger.Warnf("smartctl exit code %d for %s has bit 0x04 set (checksum error); data will still be published", exitCode, deviceName)
			}
			if exitCode&0x03 != 0 {
				mc.ReportDeviceError(deviceWWN, "xall", fmt.Sprintf("smartctl exited with fatal code %d while reading %s", exitCode, deviceName))
				return
			}
		} else {
			mc.logger.Errorf("error while attempting to execute smartctl: %s\n", deviceName)
			mc.logger.Errorf("ERROR MESSAGE: %v", err)
			mc.logger.Errorf("IGNORING RESULT: %v", result)
			mc.ReportDeviceError(deviceWWN, "xall", err.Error())
			return
		}
	}

	// Attempt FARM log collection if enabled
	if mc.config.GetBool("commands.metrics_farm_enabled") {
		resultBytes = mc.collectAndMergeFarm(resultBytes, fullDeviceName, deviceType, deviceName)
	}

	if err := mc.Publish(deviceWWN, resultBytes); err != nil {
		mc.logger.Errorf("Failed to publish SMART data for %s: %v", deviceName, err)
	}
}

// collectAndMergeFarm runs a second smartctl call to collect the Seagate FARM log
// and merges it into the main SMART JSON payload. Returns the original payload
// unmodified if FARM collection fails or the drive does not support FARM.
func (mc *MetricsCollector) collectAndMergeFarm(smartJson []byte, fullDeviceName string, deviceType string, deviceName string) []byte {
	farmArgs := strings.Split(mc.config.GetString("commands.metrics_farm_args"), " ")
	if len(deviceType) > 0 && deviceType != "scsi" && deviceType != "ata" {
		farmArgs = append(farmArgs, "--device", deviceType)
	}
	farmArgs = append(farmArgs, fullDeviceName)

	farmTimeout := time.Duration(mc.config.GetInt("commands.metrics_smartctl_timeout")) * time.Second
	farmCtx, farmCancel := context.WithTimeout(context.Background(), farmTimeout)
	defer farmCancel()
	farmResult, farmErr := mc.shell.CommandContext(farmCtx, mc.logger, mc.config.GetString(configKeySmartctlBin), farmArgs, "", os.Environ())
	if farmErr != nil {
		mc.logger.Debugf("FARM log collection failed for %s (drive may not support FARM): %v", deviceName, farmErr)
		return smartJson
	}

	// Parse FARM JSON and check if supported
	var farmMap map[string]interface{}
	if unmarshalErr := json.Unmarshal([]byte(farmResult), &farmMap); unmarshalErr != nil {
		mc.logger.Debugf("Failed to parse FARM JSON for %s: %v", deviceName, unmarshalErr)
		return smartJson
	}

	farmLog, ok := farmMap["seagate_farm_log"]
	if !ok {
		mc.logger.Debugf("No seagate_farm_log key in FARM output for %s", deviceName)
		return smartJson
	}

	// Check the supported field
	if farmLogMap, ok := farmLog.(map[string]interface{}); ok {
		if supported, ok := farmLogMap["supported"].(bool); ok && !supported {
			mc.logger.Debugf("FARM log not supported for %s", deviceName)
			return smartJson
		}
	}

	// Merge FARM data into SMART JSON
	var smartMap map[string]interface{}
	if unmarshalErr := json.Unmarshal(smartJson, &smartMap); unmarshalErr != nil {
		mc.logger.Debugf("Failed to parse SMART JSON for FARM merge on %s: %v", deviceName, unmarshalErr)
		return smartJson
	}

	smartMap["seagate_farm_log"] = farmLog
	merged, mergeErr := json.Marshal(smartMap)
	if mergeErr != nil {
		mc.logger.Debugf("Failed to marshal merged SMART+FARM JSON for %s: %v", deviceName, mergeErr)
		return smartJson
	}

	mc.logger.Infof("Successfully collected and merged FARM log for %s", deviceName)
	return merged
}

func (mc *MetricsCollector) Publish(deviceWWN string, payload []byte) error {
	mc.logger.Infof("Publishing smartctl results for %s\n", deviceWWN)

	apiEndpoint, _ := url.Parse(mc.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse(fmt.Sprintf("api/device/%s/smart", strings.ToLower(deviceWWN)))

	resp, err := mc.httpClient.Post(apiEndpoint.String(), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		mc.logger.Errorf("An error occurred while publishing SMART data for device (%s): %v", deviceWWN, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		mc.logger.Errorln("Authentication failed (HTTP 401). Check that api.token in collector.yaml matches web.auth.token in scrutiny.yaml.")
	}

	return nil
}

// collectorErrorPayload is the JSON body sent to the backend collector-error endpoints.
type collectorErrorPayload struct {
	ErrorType    string `json:"error_type"`
	ErrorMessage string `json:"error_message"`
	// DeviceName is an optional hint used when no WWN is available (scan/info errors).
	DeviceName string `json:"device_name,omitempty"`
}

// ReportDeviceError posts a collector error to /api/device/:wwn/collector-error.
// Errors from this call are logged but do not abort the collection run.
func (mc *MetricsCollector) ReportDeviceError(deviceWWN string, errorType string, errorMessage string) {
	if deviceWWN == "" {
		mc.logger.Debugf("Cannot report device error without WWN; skipping")
		return
	}
	apiEndpoint, _ := url.Parse(mc.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse(fmt.Sprintf("api/device/%s/collector-error", strings.ToLower(deviceWWN)))

	body := collectorErrorPayload{ErrorType: errorType, ErrorMessage: errorMessage}
	var result map[string]interface{}
	if err := mc.postJson(apiEndpoint.String(), body, &result); err != nil {
		mc.logger.Warnf("Failed to report collector device error for %s: %v", deviceWWN, err)
	}
}

// ReportScanError posts a collector scan-level error to /api/collector/scan-error.
// deviceName is an optional hint included in the payload so the backend can produce
// a more informative notification subject when no WWN is available.
// Errors from this call are logged but do not abort the collection run.
func (mc *MetricsCollector) ReportScanError(errorType string, errorMessage string, deviceName string) {
	apiEndpoint, _ := url.Parse(mc.apiEndpoint.String())
	apiEndpoint, _ = apiEndpoint.Parse("api/collector/scan-error")

	body := collectorErrorPayload{ErrorType: errorType, ErrorMessage: errorMessage, DeviceName: deviceName}
	var result map[string]interface{}
	if err := mc.postJson(apiEndpoint.String(), body, &result); err != nil {
		mc.logger.Warnf("Failed to report collector scan error: %v", err)
	}
}
