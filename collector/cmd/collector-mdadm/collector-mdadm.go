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
	"github.com/analogj/scrutiny/collector/pkg/mdadm"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// CLI flag and config key constants
const flagApiToken = "api-token"
const flagLogFile = "log-file"
const flagApiEndpoint = "api-endpoint"
const configKeyLogFile = "log.file"

var goos string
var goarch string

func main() {
	config, err := config.Create()
	if err != nil {
		fmt.Printf("FATAL: %+v\n", err)
		os.Exit(1)
	}

	// Use separate config file for MDADM collector
	configFilePath := "/opt/scrutiny/config/collector-mdadm.yaml"
	configFilePathAlternative := "/opt/scrutiny/config/collector-mdadm.yml"
	// Fall back to main collector config if MDADM-specific config doesn't exist
	configFilePathFallback := "/opt/scrutiny/config/collector.yaml"
	configFilePathFallbackAlt := "/opt/scrutiny/config/collector.yml"

	if !utils.FileExists(configFilePath) && utils.FileExists(configFilePathAlternative) {
		configFilePath = configFilePathAlternative
	} else if !utils.FileExists(configFilePath) && !utils.FileExists(configFilePathAlternative) {
		if utils.FileExists(configFilePathFallback) {
			configFilePath = configFilePathFallback
		} else if utils.FileExists(configFilePathFallbackAlt) {
			configFilePath = configFilePathFallbackAlt
		}
	}

	// Create a bootstrap logger for config loading
	bootstrapLogger := logrus.WithFields(logrus.Fields{"type": "mdadm"})
	bootstrapLogger.Logger.SetLevel(logrus.InfoLevel)

	// Load the config file
	err = config.ReadConfig(configFilePath, bootstrapLogger)
	if _, ok := err.(errors.ConfigFileMissingError); ok {
		// Ignore "could not find config file"
	} else if err != nil {
		os.Exit(1)
	}

	app := &cli.App{
		Name:     "scrutiny-collector-mdadm",
		Usage:    "MDADM RAID array data collector for scrutiny",
		Version:  version.VERSION,
		Compiled: time.Now(),
		Authors: []*cli.Author{
			{
				Name:  "Scrutiny Contributors",
				Email: "https://github.com/Starosdev/scrutiny",
			},
		},
		Before: func(c *cli.Context) error {
			collectorMdadm := "Starosdev/scrutiny/mdadm"

			var versionInfo string
			if len(goos) > 0 && len(goarch) > 0 {
				versionInfo = fmt.Sprintf("%s.%s-%s", goos, goarch, version.VERSION)
			} else {
				versionInfo = fmt.Sprintf("dev-%s", version.VERSION)
			}

			subtitle := collectorMdadm + utils.LeftPad2Len(versionInfo, " ", 65-len(collectorMdadm))

			banner := fmt.Sprintf(utils.StripIndent(
				`
			 ___   ___  ____  __  __  ____  ____  _  _  _  _
			/ __) / __)(  _ \(  )(  )(_  _)(_  _)( \( )( \/ )
			\__ \( (__  )   / )(__)(   )(   _)(_  )  (  \  /
			(___/ \___)(_)\_)(______) (__) (____)(_)\_) (__)
			%s
 
			`), subtitle)
			color.New(color.FgGreen).Fprintf(c.App.Writer, "%s", banner)

			return nil
		},

		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Run the scrutiny MDADM RAID array collector",
				Action: func(c *cli.Context) error {
					if c.IsSet("config") {
						err = config.ReadConfig(c.String("config"), bootstrapLogger)
						if err != nil {
							fmt.Printf("Could not find config file at specified path: %s", c.String("config"))
							return err
						}
					}

					if c.Bool("debug") {
						config.Set("log.level", "DEBUG")
					}

					if c.IsSet(flagLogFile) {
						config.Set(configKeyLogFile, c.String(flagLogFile))
					}

					if c.IsSet(flagApiEndpoint) {
						apiEndpoint := strings.TrimSuffix(c.String(flagApiEndpoint), "/") + "/"
						config.Set("api.endpoint", apiEndpoint)
					}

					if c.IsSet(flagApiToken) {
						config.Set("api.token", c.String(flagApiToken))
					}

					collectorLogger, logFile, err := CreateLogger(config)
					if logFile != nil {
						defer logFile.Close()
					}
					if err != nil {
						return err
					}

					settingsMap := config.AllSettings()
					if apiMap, ok := settingsMap["api"].(map[string]interface{}); ok {
						if _, hasToken := apiMap["token"]; hasToken && apiMap["token"] != "" {
							apiMap["token"] = "[REDACTED]"
						}
					}
					settingsData, settingsErr := json.MarshalIndent(settingsMap, "", "\t")
					if settingsErr != nil {
						collectorLogger.Warnf("Failed to marshal settings for debug logging: %v", settingsErr)
					} else {
						collectorLogger.Debug(string(settingsData))
					}

					mdadmCollector, err := mdadm.CreateCollector(
						config,
						collectorLogger,
						config.GetString("api.endpoint"),
					)
					if err != nil {
						return err
					}

					return mdadmCollector.Run()
				},

				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "Specify the path to the config file",
					},
					&cli.StringFlag{
						Name:    flagApiEndpoint,
						Usage:   "The api server endpoint",
						EnvVars: []string{"COLLECTOR_MDADM_API_ENDPOINT", "COLLECTOR_API_ENDPOINT"},
					},
					&cli.StringFlag{
						Name:    flagLogFile,
						Usage:   "Path to file for logging. Leave empty to use STDOUT",
						EnvVars: []string{"COLLECTOR_MDADM_LOG_FILE", "COLLECTOR_LOG_FILE"},
					},
					&cli.BoolFlag{
						Name:    "debug",
						Usage:   "Enable debug logging",
						EnvVars: []string{"COLLECTOR_MDADM_DEBUG", "COLLECTOR_DEBUG", "DEBUG"},
					},
					&cli.StringFlag{
						Name:    flagApiToken,
						Usage:   "API token for authenticating with the Scrutiny server",
						EnvVars: []string{"COLLECTOR_MDADM_API_TOKEN", "COLLECTOR_API_TOKEN"},
					},
				},
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(color.HiRedString("ERROR: %v", err))
	}
}

// CreateLogger creates a logger for the MDADM collector
func CreateLogger(appConfig config.Interface) (*logrus.Entry, *os.File, error) {
	logger := logrus.WithFields(logrus.Fields{
		"type": "mdadm",
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
