package detect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/analogj/scrutiny/collector/pkg/common/shell"
	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/sirupsen/logrus"
)

// Config key for the host identifier (S1192: deduplicated string literal)
const configKeyHostId = "host.id"

type Detect struct {
	Logger *logrus.Entry
	Config config.Interface
	Shell  shell.Interface
}

// stripDevicePrefix removes the platform-specific device prefix from a device path.
// On platforms where DevicePrefix() is empty (e.g., Windows), it falls back to
// stripping the common "/dev/" prefix to avoid storing paths like "/dev/sda" as
// the device name, which would cause doubling in the UI (e.g., "/dev//dev/sda").
// IOService/IODeviceTree paths are returned unchanged since they have no prefix.
func stripDevicePrefix(devicePath string) string {
	if isIOPath(devicePath) {
		return devicePath
	}
	prefix := DevicePrefix()
	if prefix != "" {
		return strings.TrimPrefix(devicePath, prefix)
	}
	// Fallback: strip "/dev/" if present (handles Windows where smartctl
	// outputs /dev/sda but DevicePrefix() is empty)
	return strings.TrimPrefix(devicePath, "/dev/")
}

// isIOPath reports whether name is a macOS IOService or IODeviceTree path.
// These paths must be passed verbatim to smartctl without a /dev/ prefix
// and without case modification.
func isIOPath(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "ioservice:") || strings.HasPrefix(lower, "iodevicetree:")
}

// DeviceFullPath returns the full path used when invoking smartctl for a device.
// For standard devices it prepends DevicePrefix() (e.g. "/dev/"); IOService and
// IODeviceTree paths are returned verbatim because they are self-contained
// identifiers that must not be prefixed.
func DeviceFullPath(deviceName string) string {
	if isIOPath(deviceName) {
		return deviceName
	}
	return fmt.Sprintf("%s%s", DevicePrefix(), deviceName)
}

func isStandardDeviceType(deviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "", "ata", "scsi", "nvme":
		return true
	default:
		return false
	}
}

func normalizeDeviceName(deviceName string) string {
	return strings.TrimSpace(stripDevicePrefix(deviceName))
}

//private/common functions

// This function calls smartctl --scan which can be used to detect storage devices.
// It has a couple of issues however:
// - --scan does not return any results on mac
//
// To handle these issues, we have OS specific wrapper functions that update/modify these detected devices.
// models.Device returned from this function only contain the minimum data for smartctl to execute: device type and device name (device file).
func (d *Detect) SmartctlScan() ([]models.Device, error) {
	//we use smartctl to detect all the drives available.
	args := strings.Split(d.Config.GetString("commands.metrics_scan_args"), " ")
	timeout := time.Duration(d.Config.GetInt("commands.metrics_smartctl_timeout")) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	detectedDeviceConnJson, err := d.Shell.CommandContext(ctx, d.Logger, d.Config.GetString("commands.metrics_smartctl_bin"), args, "", os.Environ())
	if err != nil {
		d.Logger.Errorf("Error scanning for devices: %v", err)
		return nil, err
	}

	var detectedDeviceConns models.Scan
	err = json.Unmarshal([]byte(detectedDeviceConnJson), &detectedDeviceConns)
	if err != nil {
		d.Logger.Errorf("Error decoding detected devices: %v", err)
		return nil, err
	}

	detectedDevices := d.TransformDetectedDevices(detectedDeviceConns)

	return detectedDevices, nil
}

// updates a device model with information from smartctl --scan
// It has a couple of issues however:
// - WWN is provided as component data, rather than a "string". We'll have to generate the WWN value ourselves
// - WWN from smartctl only provided for ATA protocol drives, NVMe and SCSI drives do not include WWN.
func (d *Detect) SmartCtlInfo(device *models.Device) error {
	if strings.TrimSpace(device.DeviceName) == "" {
		return fmt.Errorf("device name is empty; skipping smartctl info call")
	}
	fullDeviceName := DeviceFullPath(device.DeviceName)
	args := strings.Split(d.Config.GetCommandMetricsInfoArgs(fullDeviceName), " ")
	//only include the device type if its a non-standard one. In some cases ata drives are detected as scsi in docker, and metadata is lost.
	if len(device.DeviceType) > 0 && device.DeviceType != "scsi" && device.DeviceType != "ata" {
		args = append(args, "--device", device.DeviceType)
	}
	args = append(args, fullDeviceName)

	timeout := time.Duration(d.Config.GetInt("commands.metrics_smartctl_timeout")) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	availableDeviceInfoJson, err := d.Shell.CommandContext(ctx, d.Logger, d.Config.GetString("commands.metrics_smartctl_bin"), args, "", os.Environ())
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode := exitErr.ExitCode()
		if exitCode&0xBF == 0 {
			d.Logger.Warnf("Successfully retrieved device information for %s, but received exit code %d, which is a non-fatal exit code. Continuing.", device.DeviceName, exitCode)
		} else {
			d.Logger.Errorf("Could not retrieve device information for %s: %v", device.DeviceName, err)
			return err
		}
	}

	if err != nil {
		d.Logger.Errorf("Could not retrieve device information for %s: %v", device.DeviceName, err)
		return err
	}

	var availableDeviceInfo collector.SmartInfo
	err = json.Unmarshal([]byte(availableDeviceInfoJson), &availableDeviceInfo)
	if err != nil {
		d.Logger.Errorf("Could not decode device information for %s: %v", device.DeviceName, err)
		return err
	}

	//WWN: this is a serial number/world-wide number that will not change.
	//DeviceType and DeviceName are already populated, however may change between collector runs (eg. config/host restart)
	//InterfaceType:
	device.ModelName = availableDeviceInfo.ModelName
	device.InterfaceSpeed = availableDeviceInfo.InterfaceSpeed.Current.String
	device.SerialNumber = availableDeviceInfo.SerialNumber
	device.Firmware = availableDeviceInfo.FirmwareVersion
	device.RotationSpeed = availableDeviceInfo.RotationRate
	device.Capacity = availableDeviceInfo.Capacity()
	device.FormFactor = availableDeviceInfo.FormFactor.Name
	device.DeviceProtocol = availableDeviceInfo.Device.Protocol
	device.SmartSupport = availableDeviceInfo.SmartSupport
	device.ResolvedDeviceName = normalizeDeviceName(availableDeviceInfo.Device.Name)
	// Only set DeviceType if not already populated (e.g., from user override or scan)
	if len(device.DeviceType) == 0 {
		device.DeviceType = availableDeviceInfo.Device.Type
	}
	if len(availableDeviceInfo.Vendor) > 0 {
		device.Manufacturer = availableDeviceInfo.Vendor
	}

	//populate WWN is possible if present
	if availableDeviceInfo.Wwn.Naa != 0 { //valid values are 1-6 (5 is what we handle correctly)
		d.Logger.Info("Generating WWN")
		wwn := Wwn{
			Naa: availableDeviceInfo.Wwn.Naa,
			Oui: availableDeviceInfo.Wwn.Oui,
			Id:  availableDeviceInfo.Wwn.ID,
		}
		device.WWN = strings.ToLower(wwn.ToString())
		d.Logger.Debugf("NAA: %d OUI: %d Id: %d => WWN: %s", wwn.Naa, wwn.Oui, wwn.Id, device.WWN)
	} else {
		d.Logger.Info("Using WWN Fallback")
		d.wwnFallback(device)
	}
	if len(device.WWN) == 0 {
		d.Logger.Warnf("no WWN populated for device: %s. Device will be registered using model+serial as identifier.", device.DeviceName)
	}

	return nil
}

// function will remove devices that are marked for "ignore" in config file
// will also add devices that are specified in config file, but "missing" from smartctl --scan
// this function will also update the deviceType to the option specified in config.
func (d *Detect) TransformDetectedDevices(detectedDeviceConns models.Scan) []models.Device {
	groupedDevices := d.buildScannedDeviceGroups(&detectedDeviceConns)

	// now that we've "grouped" all the devices, lets override any groups specified in the config file.
	d.applyDeviceOverrides(groupedDevices)

	// flatten map
	detectedDevices := []models.Device{}
	for _, group := range groupedDevices {
		detectedDevices = append(detectedDevices, group...)
	}

	return detectedDevices
}

// buildScannedDeviceGroups builds the initial device groups from a smartctl --scan result,
// keyed by the (case-preserved) device path and filtered by the configured allow list.
func (d *Detect) buildScannedDeviceGroups(scan *models.Scan) map[string][]models.Device {
	groupedDevices := map[string][]models.Device{}
	for _, scannedDevice := range scan.Devices {
		if strings.TrimSpace(scannedDevice.Name) == "" {
			d.Logger.Warnf("smartctl --scan returned a device entry with an empty name; skipping to avoid invoking smartctl with path %q", DevicePrefix())
			continue
		}

		// Preserve case for all device paths — filesystem paths are case-sensitive.
		// /dev/disk/by-id/ paths may contain uppercase characters from manufacturer
		// names and serial numbers that must be passed verbatim to smartctl.
		deviceFile := scannedDevice.Name

		// If the user has defined a device allow list, and this device isnt there, then ignore it
		if !d.Config.IsAllowlistedDevice(deviceFile) {
			continue
		}

		groupedDevices[deviceFile] = append(groupedDevices[deviceFile], models.Device{
			HostId:           d.Config.GetString(configKeyHostId),
			CollectorVersion: version.VERSION,
			DeviceType:       scannedDevice.Type,
			DeviceName:       stripDevicePrefix(deviceFile),
		})
	}
	return groupedDevices
}

// applyDeviceOverrides mutates groupedDevices according to the config device overrides, either
// removing ignored devices or replacing the scanned group with the configured one.
func (d *Detect) applyDeviceOverrides(groupedDevices map[string][]models.Device) {
	for _, overrideDevice := range d.Config.GetDeviceOverrides() {
		// Preserve case for the override device path — filesystem paths are case-sensitive.
		// Map lookups use case-insensitive comparison to match scanned devices without mutating paths.
		overrideDeviceFile := overrideDevice.Device

		if overrideDevice.Ignore {
			deleteGroupedDeviceFold(groupedDevices, overrideDeviceFile, true)
			continue
		}

		// create a new device group, and replace the one generated by smartctl --scan
		overrideDeviceGroup := d.buildOverrideDeviceGroup(&overrideDevice, groupedDevices)

		// Remove any scanned entry stored under a different case to prevent duplicates.
		deleteGroupedDeviceFold(groupedDevices, overrideDeviceFile, false)
		groupedDevices[overrideDeviceFile] = overrideDeviceGroup
	}
}

// buildOverrideDeviceGroup constructs the replacement device group for a non-ignored override,
// expanding explicit device types or defaulting to the scanned (or "ata") type.
func (d *Detect) buildOverrideDeviceGroup(overrideDevice *models.ScanOverride, groupedDevices map[string][]models.Device) []models.Device {
	overrideDeviceGroup := []models.Device{}
	if overrideDevice.DeviceType != nil {
		for _, overrideDeviceType := range overrideDevice.DeviceType {
			overrideDeviceGroup = append(overrideDeviceGroup, models.Device{
				HostId:           d.Config.GetString(configKeyHostId),
				CollectorVersion: version.VERSION,
				DeviceType:       overrideDeviceType,
				DeviceName:       stripDevicePrefix(overrideDevice.Device),
				Label:            overrideDevice.Label,
			})
		}
		return overrideDeviceGroup
	}

	// user may have specified device in config file without device type (default to scanned device type)
	overrideDeviceGroup = append(overrideDeviceGroup, models.Device{
		HostId:           d.Config.GetString(configKeyHostId),
		CollectorVersion: version.VERSION,
		DeviceType:       scannedDeviceType(groupedDevices, overrideDevice.Device),
		DeviceName:       stripDevicePrefix(overrideDevice.Device),
		Label:            overrideDevice.Label,
	})
	return overrideDeviceGroup
}

// scannedDeviceType returns the device type of the scanned group matching deviceFile
// (case-insensitive), defaulting to "ata" when no scanned device is present.
func scannedDeviceType(groupedDevices map[string][]models.Device, deviceFile string) string {
	if scannedKey := groupedDeviceKey(groupedDevices, deviceFile); scannedKey != "" {
		if scanned := groupedDevices[scannedKey]; len(scanned) > 0 {
			return scanned[0].DeviceType
		}
	}
	return "ata"
}

// deleteGroupedDeviceFold deletes the first map entry whose key case-insensitively matches
// deviceFile. When onlyDifferentCase is true, an exact-case match is preserved.
func deleteGroupedDeviceFold(groupedDevices map[string][]models.Device, deviceFile string, includeExactCase bool) {
	for key := range groupedDevices {
		if !strings.EqualFold(key, deviceFile) {
			continue
		}
		if !includeExactCase && key == deviceFile {
			continue
		}
		delete(groupedDevices, key)
		return
	}
}

func FilterRedundantDevices(devices []models.Device) []models.Device {
	controllerResolvedNames := map[string]struct{}{}
	for i := range devices {
		device := devices[i]
		if isStandardDeviceType(device.DeviceType) {
			continue
		}
		if normalizeDeviceName(device.DeviceName) == normalizeDeviceName(device.ResolvedDeviceName) {
			continue
		}
		if resolved := normalizeDeviceName(device.ResolvedDeviceName); resolved != "" {
			controllerResolvedNames[resolved] = struct{}{}
		}
	}

	if len(controllerResolvedNames) == 0 {
		return devices
	}

	filtered := make([]models.Device, 0, len(devices))
	for i := range devices {
		device := devices[i]
		if isStandardDeviceType(device.DeviceType) {
			if _, redundant := controllerResolvedNames[normalizeDeviceName(device.DeviceName)]; redundant {
				continue
			}
		}
		filtered = append(filtered, device)
	}
	return filtered
}

// groupedDeviceKey returns the key in groupedDevices whose lowercased form matches the
// lowercased target, enabling case-insensitive lookups without mutating stored paths.
// Returns an empty string if no match is found.
func groupedDeviceKey(groupedDevices map[string][]models.Device, target string) string {
	for key := range groupedDevices {
		if strings.EqualFold(key, target) {
			return key
		}
	}
	return ""
}
