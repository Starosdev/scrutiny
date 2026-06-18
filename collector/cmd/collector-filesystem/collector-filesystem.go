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
	"github.com/analogj/scrutiny/collector/pkg/filesystem"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const flagHostID = "host-id"
const flagAPIToken = "api-token"
const flagLogFile = "log-file"
const flagAPIEndpoint = "api-endpoint"
const configKeyLogFile = "log.file"

var goos string
var goarch string

func main() {
	appConfig, err := config.Create()
	if err != nil {
		fmt.Printf("FATAL: %+v\n", err)
		os.Exit(1)
	}

	configFilePath := resolveConfigPath()

	bootstrapLogger := logrus.WithFields(logrus.Fields{"type": "filesystem"})
	bootstrapLogger.Logger.SetLevel(logrus.InfoLevel)

	err = appConfig.ReadConfig(configFilePath, bootstrapLogger)
	if _, missing := err.(errors.ConfigFileMissingError); err != nil && !missing {
		os.Exit(1)
	}

	app := &cli.App{
		Name:     "scrutiny-collector-filesystem",
		Usage:    "filesystem capacity collector for scrutiny",
		Version:  version.VERSION,
		Compiled: time.Now(),
		Before: func(c *cli.Context) error {
			collectorName := "Starosdev/scrutiny/filesystem"

			var versionInfo string
			if len(goos) > 0 && len(goarch) > 0 {
				versionInfo = fmt.Sprintf("%s.%s-%s", goos, goarch, version.VERSION)
			} else {
				versionInfo = fmt.Sprintf("dev-%s", version.VERSION)
			}

			subtitle := collectorName + utils.LeftPad2Len(versionInfo, " ", 65-len(collectorName))
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
				Name:   "run",
				Usage:  "Run the scrutiny filesystem capacity collector",
				Action: runCollectorAction(appConfig, bootstrapLogger),
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Usage: "Specify the path to the config file"},
					&cli.StringFlag{Name: flagAPIEndpoint, Usage: "The api server endpoint", EnvVars: []string{"COLLECTOR_FILESYSTEM_API_ENDPOINT", "COLLECTOR_API_ENDPOINT"}},
					&cli.StringFlag{Name: flagLogFile, Usage: "Path to file for logging. Leave empty to use STDOUT", EnvVars: []string{"COLLECTOR_FILESYSTEM_LOG_FILE", "COLLECTOR_LOG_FILE"}},
					&cli.BoolFlag{Name: "debug", Usage: "Enable debug logging", EnvVars: []string{"COLLECTOR_FILESYSTEM_DEBUG", "COLLECTOR_DEBUG", "DEBUG"}},
					&cli.StringFlag{Name: flagHostID, Usage: "Host identifier/label, used for grouping filesystems", EnvVars: []string{"COLLECTOR_FILESYSTEM_HOST_ID", "COLLECTOR_HOST_ID"}},
					&cli.StringFlag{Name: flagAPIToken, Usage: "API token for authenticating with the Scrutiny server", EnvVars: []string{"COLLECTOR_FILESYSTEM_API_TOKEN", "COLLECTOR_API_TOKEN"}},
				},
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(color.HiRedString("ERROR: %v", err))
	}
}

// resolveConfigPath returns the first existing collector config file, preferring the
// filesystem-specific config and falling back to the shared collector config. Defaults to the
// primary filesystem path when none exist.
func resolveConfigPath() string {
	candidates := []string{
		"/opt/scrutiny/config/collector-filesystem.yaml",
		"/opt/scrutiny/config/collector-filesystem.yml",
		"/opt/scrutiny/config/collector.yaml",
		"/opt/scrutiny/config/collector.yml",
	}
	for _, path := range candidates {
		if utils.FileExists(path) {
			return path
		}
	}
	return candidates[0]
}

// applyRunFlags reads an explicit --config file (if set) and overlays CLI flag values onto appConfig.
func applyRunFlags(c *cli.Context, appConfig config.Interface, bootstrapLogger *logrus.Entry) error {
	if c.IsSet("config") {
		if err := appConfig.ReadConfig(c.String("config"), bootstrapLogger); err != nil {
			fmt.Printf("Could not find config file at specified path: %s", c.String("config"))
			return err
		}
	}
	if c.IsSet(flagHostID) {
		appConfig.Set("host.id", c.String(flagHostID))
	}
	if c.Bool("debug") {
		appConfig.Set("log.level", "DEBUG")
	}
	if c.IsSet(flagLogFile) {
		appConfig.Set(configKeyLogFile, c.String(flagLogFile))
	}
	if c.IsSet(flagAPIEndpoint) {
		appConfig.Set("api.endpoint", strings.TrimSuffix(c.String(flagAPIEndpoint), "/")+"/")
	}
	if c.IsSet(flagAPIToken) {
		appConfig.Set("api.token", c.String(flagAPIToken))
	}
	return nil
}

// logRedactedSettings debug-logs the effective settings with the API token redacted.
func logRedactedSettings(logger *logrus.Entry, appConfig config.Interface) {
	settingsMap := appConfig.AllSettings()
	if apiMap, ok := settingsMap["api"].(map[string]interface{}); ok {
		if token, hasToken := apiMap["token"]; hasToken && token != "" {
			apiMap["token"] = "[REDACTED]"
		}
	}
	settingsData, err := json.MarshalIndent(settingsMap, "", "\t")
	if err != nil {
		logger.Warnf("Failed to marshal settings for debug logging: %v", err)
		return
	}
	logger.Debug(string(settingsData))
}

// runCollectorAction builds the cli action that configures and runs the filesystem collector.
func runCollectorAction(appConfig config.Interface, bootstrapLogger *logrus.Entry) cli.ActionFunc {
	return func(c *cli.Context) error {
		if err := applyRunFlags(c, appConfig, bootstrapLogger); err != nil {
			return err
		}

		collectorLogger, logFile, loggerErr := CreateLogger(appConfig)
		if logFile != nil {
			defer logFile.Close()
		}
		if loggerErr != nil {
			return loggerErr
		}

		logRedactedSettings(collectorLogger, appConfig)

		filesystemCollector, collectorErr := filesystem.CreateCollector(
			appConfig,
			collectorLogger,
			appConfig.GetString("api.endpoint"),
		)
		if collectorErr != nil {
			return collectorErr
		}

		return filesystemCollector.Run()
	}
}

func CreateLogger(appConfig config.Interface) (*logrus.Entry, *os.File, error) {
	logger := logrus.WithFields(logrus.Fields{
		"type": "filesystem",
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
