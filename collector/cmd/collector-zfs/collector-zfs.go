package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	_ "go.uber.org/automaxprocs"

	utils "github.com/analogj/go-util/utils"
	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/errors"
	"github.com/analogj/scrutiny/collector/pkg/zfs"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// CLI flag and config key constants (S1192: deduplicated string literals)
const flagHostId = "host-id"
const flagApiToken = "api-token"
const flagLogFile = "log-file"
const flagApiEndpoint = "api-endpoint"
const configKeyLogFile = "log.file"

var goos string
var goarch string

func main() {
	cfg, createErr := config.Create()
	if createErr != nil {
		fmt.Printf("FATAL: %+v\n", createErr)
		os.Exit(1)
	}

	// Create a bootstrap logger for config loading
	bootstrapLogger := logrus.WithFields(logrus.Fields{"type": "zfs"})
	bootstrapLogger.Logger.SetLevel(logrus.InfoLevel)

	if err := readOptionalCollectorConfig(cfg, resolveCollectorConfigPath("zfs"), bootstrapLogger); err != nil {
		os.Exit(1)
	}

	cli.CommandHelpTemplate = `NAME:
   {{.HelpName}} - {{.Usage}}
USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Category}}
CATEGORY:
   {{.Category}}{{end}}{{if .Description}}
DESCRIPTION:
   {{.Description}}{{end}}{{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`

	app := &cli.App{
		Name:     "scrutiny-collector-zfs",
		Usage:    "ZFS pool data collector for scrutiny",
		Version:  version.VERSION,
		Compiled: time.Now(),
		Authors: []*cli.Author{
			{
				Name:  "Scrutiny Contributors",
				Email: "https://github.com/Starosdev/scrutiny",
			},
		},
		Before: func(c *cli.Context) error {
			color.New(color.FgGreen).Fprintf(c.App.Writer, "%s", collectorBanner("Starosdev/scrutiny/zfs"))
			return nil
		},

		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Run the scrutiny ZFS pool collector",
				Action: func(c *cli.Context) error {
					if c.IsSet("config") {
						if err := cfg.ReadConfig(c.String("config"), bootstrapLogger); err != nil {
							fmt.Printf("Could not find config file at specified path: %s", c.String("config"))
							return err
						}
					}

					applyCollectorOverrides(c, cfg)

					collectorLogger, logFile, err := CreateLogger(cfg)
					if logFile != nil {
						defer logFile.Close()
					}
					if err != nil {
						return err
					}

					settingsData, settingsErr := redactCollectorSettings(cfg)
					if settingsErr != nil {
						collectorLogger.Warnf("Failed to marshal settings for debug logging: %v", settingsErr)
					} else {
						collectorLogger.Debug(string(settingsData))
					}

					zfsCollector, err := zfs.CreateCollector(
						cfg,
						collectorLogger,
						cfg.GetString("api.endpoint"),
					)
					if err != nil {
						return err
					}

					return zfsCollector.Run()
				},

				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "Specify the path to the config file",
					},
					&cli.StringFlag{
						Name:    flagApiEndpoint,
						Usage:   "The api server endpoint",
						EnvVars: []string{"COLLECTOR_ZFS_API_ENDPOINT", "COLLECTOR_API_ENDPOINT"},
					},
					&cli.StringFlag{
						Name:    flagLogFile,
						Usage:   "Path to file for logging. Leave empty to use STDOUT",
						EnvVars: []string{"COLLECTOR_ZFS_LOG_FILE", "COLLECTOR_LOG_FILE"},
					},
					&cli.BoolFlag{
						Name:    "debug",
						Usage:   "Enable debug logging",
						EnvVars: []string{"COLLECTOR_ZFS_DEBUG", "COLLECTOR_DEBUG", "DEBUG"},
					},
					&cli.StringFlag{
						Name:    flagHostId,
						Usage:   "Host identifier/label, used for grouping pools",
						Value:   "",
						EnvVars: []string{"COLLECTOR_ZFS_HOST_ID", "COLLECTOR_HOST_ID"},
					},
					&cli.StringFlag{
						Name:    flagApiToken,
						Usage:   "API token for authenticating with the Scrutiny server",
						EnvVars: []string{"COLLECTOR_ZFS_API_TOKEN", "COLLECTOR_API_TOKEN"},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(color.HiRedString("ERROR: %v", err))
	}
}

func resolveCollectorConfigPath(collectorName string) string {
	configFilePath := fmt.Sprintf("/opt/scrutiny/config/collector-%s.yaml", collectorName)
	configFilePathAlternative := fmt.Sprintf("/opt/scrutiny/config/collector-%s.yml", collectorName)
	configFilePathFallback := "/opt/scrutiny/config/collector.yaml"
	configFilePathFallbackAlt := "/opt/scrutiny/config/collector.yml"
	if !utils.FileExists(configFilePath) && utils.FileExists(configFilePathAlternative) {
		return configFilePathAlternative
	}
	if !utils.FileExists(configFilePath) && !utils.FileExists(configFilePathAlternative) {
		if utils.FileExists(configFilePathFallback) {
			return configFilePathFallback
		}
		if utils.FileExists(configFilePathFallbackAlt) {
			return configFilePathFallbackAlt
		}
	}
	return configFilePath
}

func readOptionalCollectorConfig(cfg config.Interface, configFilePath string, bootstrapLogger *logrus.Entry) error {
	err := cfg.ReadConfig(configFilePath, bootstrapLogger)
	if _, ok := err.(errors.ConfigFileMissingError); ok {
		return nil
	}
	return err
}

func applyCollectorOverrides(c *cli.Context, cfg config.Interface) {
	if c.IsSet(flagHostId) {
		cfg.Set("host.id", c.String(flagHostId))
	}
	if c.Bool("debug") {
		cfg.Set("log.level", "DEBUG")
	}
	if c.IsSet(flagLogFile) {
		cfg.Set(configKeyLogFile, c.String(flagLogFile))
	}
	if c.IsSet(flagApiEndpoint) {
		apiEndpoint := strings.TrimSuffix(c.String(flagApiEndpoint), "/") + "/"
		cfg.Set("api.endpoint", apiEndpoint)
	}
	if c.IsSet(flagApiToken) {
		cfg.Set("api.token", c.String(flagApiToken))
	}
}

func redactCollectorSettings(cfg config.Interface) ([]byte, error) {
	settingsMap := cfg.AllSettings()
	if apiMap, ok := settingsMap["api"].(map[string]interface{}); ok {
		if _, hasToken := apiMap["token"]; hasToken && apiMap["token"] != "" {
			apiMap["token"] = "[REDACTED]"
		}
	}
	return json.MarshalIndent(settingsMap, "", "\t")
}

func collectorBanner(name string) string {
	versionInfo := fmt.Sprintf("dev-%s", version.VERSION)
	if len(goos) > 0 && len(goarch) > 0 {
		versionInfo = fmt.Sprintf("%s.%s-%s", goos, goarch, version.VERSION)
	}
	subtitle := name + utils.LeftPad2Len(versionInfo, " ", 65-len(name))
	return fmt.Sprintf(utils.StripIndent(
		`
		 ___   ___  ____  __  __  ____  ____  _  _  _  _
		/ __) / __)(  _ \(  )(  )(_  _)(_  _)( \( )( \/ )
		\__ \( (__  )   / )(__)(   )(   _)(_  )  (  \  /
		(___/ \___)(_)\_)(______) (__) (____)(_)\_) (__)
		%s

		`), subtitle)
}

// CreateLogger creates a logger for the ZFS collector
func CreateLogger(appConfig config.Interface) (*logrus.Entry, *os.File, error) {
	logger := logrus.WithFields(logrus.Fields{
		"type": "zfs",
	})

	if level, err := logrus.ParseLevel(appConfig.GetString("log.level")); err == nil {
		logger.Logger.SetLevel(level)
	} else {
		logger.Logger.SetLevel(logrus.InfoLevel)
	}

	var logFile *os.File
	var err error
	if appConfig.IsSet(configKeyLogFile) && len(appConfig.GetString(configKeyLogFile)) > 0 {
		logFile, err = os.OpenFile(appConfig.GetString(configKeyLogFile), os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Logger.Errorf("Failed to open log file %s for output: %s", appConfig.GetString(configKeyLogFile), err)
			return nil, logFile, err
		}
		logger.Logger.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}
	return logger, logFile, nil
}
