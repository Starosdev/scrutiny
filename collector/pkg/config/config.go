package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/analogj/go-util/utils"
	"github.com/analogj/scrutiny/collector/pkg/errors"
	"github.com/analogj/scrutiny/collector/pkg/models"
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

	deviceOverrides    []models.ScanOverride
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
		args := strings.Split(commandArgString, " ")
		//ensure that the args string contains `--json` or `-j` flag
		containsJsonFlag := false
		containsDeviceFlag := false
		for _, flag := range args {
			if strings.HasPrefix(flag, "--json") || strings.HasPrefix(flag, "-j") {
				containsJsonFlag = true
			}
			if strings.HasPrefix(flag, "--device") || strings.HasPrefix(flag, "-d") {
				containsDeviceFlag = true
			}
		}

		if !containsJsonFlag {
			errorStrings = append(errorStrings, fmt.Sprintf("configuration key '%s' is missing '--json' flag", configKey))
		}

		if containsDeviceFlag {
			errorStrings = append(errorStrings, fmt.Sprintf("configuration key '%s' must not contain '--device' or '-d' flag", configKey))
		}
	}
	//sort(errorStrings)
	sort.Strings(errorStrings)

	if len(errorStrings) == 0 {
		return nil
	} else {
		return errors.ConfigValidationError(strings.Join(errorStrings, ", "))
	}
}

func (c *configuration) GetDeviceOverrides() []models.ScanOverride {
	// we have to support 2 types of device types.
	// - simple device type (device_type: 'sat')
	// and list of device types (type: \n- 3ware,0 \n- 3ware,1 \n- 3ware,2)
	// GetString will return "" if this is a list of device types.

	if c.deviceOverrides == nil {
		overrides := []models.ScanOverride{}
		c.UnmarshalKey("devices", &overrides, func(c *mapstructure.DecoderConfig) { c.WeaklyTypedInput = true })
		c.deviceOverrides = overrides
	}

	return c.deviceOverrides
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
