package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/analogj/scrutiny/collector/pkg/errors"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/analogj/scrutiny/pkg/utils"
	"github.com/go-viper/mapstructure/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config key constants for smartctl command arguments (S1192: deduplicated string literals)
const configKeyMetricsScanArgs = "commands.metrics_scan_args"
const configKeyMetricsInfoArgs = "commands.metrics_info_args"
const configKeyMetricsSmartArgs = "commands.metrics_smart_args"

// When initializing this class the following methods must be called:
// Config.New
// Config.Init
// This is done automatically when created via the Factory.
type configuration struct {
	*viper.Viper

	deviceOverrides []models.ScanOverride
}

//Viper uses the following precedence order. Each item takes precedence over the item below it:
// explicit call to Set
// flag
// env
// config
// key/value store
// default

func (c *configuration) Init() error {
	c.Viper = viper.New()
	//set defaults
	c.SetDefault("host.id", "")

	c.SetDefault("devices", []string{})

	c.SetDefault("log.level", "INFO")
	c.SetDefault("log.file", "")

	c.SetDefault("api.endpoint", "http://localhost:8080")
	c.SetDefault("api.timeout", 60)
	c.SetDefault("api.token", "")

	c.SetDefault("commands.metrics_smartctl_bin", "smartctl")
	c.SetDefault(configKeyMetricsScanArgs, "--scan --json")
	c.SetDefault(configKeyMetricsInfoArgs, "--info --json")
	c.SetDefault(configKeyMetricsSmartArgs, "--xall --json")
	c.SetDefault("commands.metrics_smartctl_wait", 0)
	c.SetDefault("commands.metrics_api_retry_count", 2)
	c.SetDefault("commands.metrics_api_retry_delay", 2)
	c.SetDefault("commands.metrics_farm_enabled", false)
	c.SetDefault("commands.metrics_farm_args", "-l farm --json")
	c.SetDefault("commands.metrics_smartctl_timeout", 120)
	c.SetDefault("commands.performance_fio_timeout", 300)

	//configure env variable parsing.
	c.SetEnvPrefix("COLLECTOR")
	c.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	c.AutomaticEnv()

	//c.SetDefault("collect.short.command", "-a -o on -S on")

	c.SetDefault("commands.performance_fio_bin", "fio")
	c.SetDefault("performance.enabled", false)
	c.SetDefault("performance.profile", "quick")
	c.SetDefault("performance.allow_direct_device_io", false)
	c.SetDefault("performance.temp_file_size", "256M")
	c.SetDefault("performance.mount_points", map[string]string{})

	c.SetDefault("allow_listed_devices", []string{})

	c.SetDefault("cron.schedule", "")
	c.SetDefault("cron.run_on_startup", false)
	c.SetDefault("cron.startup_sleep_secs", 0)

	//if you want to load a non-standard location system config file (~/drawbridge.yml), use ReadConfig
	c.SetConfigType("yaml")
	//c.SetConfigName("drawbridge")
	//c.AddConfigPath("$HOME/")

	//CLI options will be added via the `Set()` function
	return nil
}

func (c *configuration) ReadConfig(configFilePath string, logger *logrus.Entry) error {
	configFilePath, err := utils.ExpandPath(configFilePath)
	if err != nil {
		return err
	}

	if !utils.FileExists(configFilePath) {
		logger.Infof("No configuration file found at %v. Using Defaults.", configFilePath)
		return errors.ConfigFileMissingError("The configuration file could not be found.")
	}

	//validate config file contents
	//err = c.ValidateConfigFile(configFilePath)
	//if err != nil {
	//	logger.Errorf("Config file at `%v` is invalid: %s", configFilePath, err)
	//	return err
	//}

	logger.Infof("Loading configuration file: %s", configFilePath)

	config_data, err := os.Open(configFilePath)
	if err != nil {
		logger.Errorf("Error reading configuration file: %s", err)
		return err
	}

	err = c.MergeConfig(config_data)
	if err != nil {
		return err
	}

	return c.ValidateConfig()
}

// This function ensures that the merged config works correctly.
func (c *configuration) ValidateConfig() error {

	//TODO:
	// check that device prefix matches OS
	// check that schema of config file is valid

	// check that the collector commands are valid
	commandArgStrings := map[string]string{
		configKeyMetricsScanArgs:  c.GetString(configKeyMetricsScanArgs),
		configKeyMetricsInfoArgs:  c.GetString(configKeyMetricsInfoArgs),
		configKeyMetricsSmartArgs: c.GetString(configKeyMetricsSmartArgs),
	}

	errorStrings := []string{}
	for configKey, commandArgString := range commandArgStrings {
		errorStrings = append(errorStrings, validateCollectorCommandArgs(configKey, commandArgString)...)
	}
	//sort(errorStrings)
	sort.Strings(errorStrings)

	if len(errorStrings) == 0 {
		return nil
	}
	return errors.ConfigValidationError(strings.Join(errorStrings, ", "))
}

func validateCollectorCommandArgs(configKey string, commandArgString string) []string {
	args := strings.Split(commandArgString, " ")
	containsJSONFlag, containsDeviceFlag := collectorCommandFlags(args)
	validationErrors := []string{}
	if !containsJSONFlag {
		validationErrors = append(validationErrors, fmt.Sprintf("configuration key '%s' is missing '--json' flag", configKey))
	}
	if containsDeviceFlag {
		validationErrors = append(validationErrors, fmt.Sprintf("configuration key '%s' must not contain '--device' or '-d' flag", configKey))
	}
	return validationErrors
}

func collectorCommandFlags(args []string) (containsJSONFlag bool, containsDeviceFlag bool) {
	for _, flag := range args {
		if strings.HasPrefix(flag, "--json") || strings.HasPrefix(flag, "-j") {
			containsJSONFlag = true
		}
		if strings.HasPrefix(flag, "--device") || strings.HasPrefix(flag, "-d") {
			containsDeviceFlag = true
		}
	}
	return containsJSONFlag, containsDeviceFlag
}

func (c *configuration) GetDeviceOverrides() []models.ScanOverride {
	// We support 2 shapes for a device's type:
	// - a single device type   (type: 'sat')
	// - a list of device types (type: \n- 3ware,0 \n- 3ware,1 \n- 3ware,2)
	//
	// A comma is part of the smartctl device-type grammar ('megaraid,14',
	// 'sat,auto', 'jmb39x-q,0'), not a separator between types. Viper's default
	// decode hook splits a scalar string on ',' when the destination is a slice,
	// which turns 'jmb39x-q,0' into the two bogus types 'jmb39x-q' and '0'.
	// Override the hook so WeaklyTypedInput simply lifts the scalar into a
	// one-element slice verbatim; a YAML list still yields one entry per item.
	// Fixes #418, #663.
	if c.deviceOverrides == nil {
		overrides := []models.ScanOverride{}
		err := c.UnmarshalKey("devices", &overrides, func(decoderConfig *mapstructure.DecoderConfig) {
			decoderConfig.WeaklyTypedInput = true
			decoderConfig.DecodeHook = mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
			)
		})
		if err != nil {
			logrus.Errorf("Could not parse the 'devices' section of the collector config; no device overrides will be applied: %v", err)
			c.deviceOverrides = []models.ScanOverride{}
			return c.deviceOverrides
		}
		c.deviceOverrides = normalizeDeviceOverrides(overrides)
	}

	return c.deviceOverrides
}

// normalizeDeviceOverrides trims surrounding whitespace from every configured
// device type and drops the empty ones. Whitespace matters because the type is
// handed verbatim to smartctl's --device flag. Empty entries matter because the
// decode hook removed above used to map an explicitly empty type to an empty
// slice, whereas WeaklyTypedInput lifting maps it to a slice holding one empty
// string.
//
// An all-empty type deliberately becomes an empty (but non-nil) slice, matching
// the old behavior so that buildOverrideDeviceGroup keeps dropping the device
// rather than falling back to the scanned type.
func normalizeDeviceOverrides(overrides []models.ScanOverride) []models.ScanOverride {
	for i := range overrides {
		if overrides[i].DeviceType == nil {
			continue
		}
		cleaned := make([]string, 0, len(overrides[i].DeviceType))
		for _, deviceType := range overrides[i].DeviceType {
			if trimmed := strings.TrimSpace(deviceType); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		overrides[i].DeviceType = cleaned
	}
	return overrides
}

func (c *configuration) GetCommandMetricsInfoArgs(deviceName string) string {
	overrides := c.GetDeviceOverrides()

	for _, deviceOverrides := range overrides {
		if strings.ToLower(deviceName) == strings.ToLower(deviceOverrides.Device) {
			//found matching device
			if len(deviceOverrides.Commands.MetricsInfoArgs) > 0 {
				return deviceOverrides.Commands.MetricsInfoArgs
			} else {
				return c.GetString(configKeyMetricsInfoArgs)
			}
		}
	}
	return c.GetString(configKeyMetricsInfoArgs)
}

func (c *configuration) GetCommandMetricsSmartArgs(deviceName string) string {
	overrides := c.GetDeviceOverrides()

	for _, deviceOverrides := range overrides {
		if strings.ToLower(deviceName) == strings.ToLower(deviceOverrides.Device) {
			//found matching device
			if len(deviceOverrides.Commands.MetricsSmartArgs) > 0 {
				return deviceOverrides.Commands.MetricsSmartArgs
			} else {
				return c.GetString(configKeyMetricsSmartArgs)
			}
		}
	}
	return c.GetString(configKeyMetricsSmartArgs)
}

func (c *configuration) IsAllowlistedDevice(deviceName string) bool {
	allowList := c.GetStringSlice("allow_listed_devices")
	if len(allowList) == 0 {
		return true
	}

	for _, item := range allowList {
		if item == deviceName {
			return true
		}
	}

	return false
}

func (c *configuration) GetAPITimeout() int {
	return c.GetInt("api.timeout")
}

func (c *configuration) GetAPIToken() string {
	return c.GetString("api.token")
}
