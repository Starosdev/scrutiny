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
	"github.com/analogj/scrutiny/collector/pkg/performance"
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
	config, err := config.Create()
	if err != nil {
		fmt.Printf("FATAL: %+v\n", err)
		os.Exit(1)
	}

	// Use separate config file for performance collector, falling back to the shared collector config
	configFilePath := resolveConfigPath()

	// Create a bootstrap logger for config loading
	bootstrapLogger := logrus.WithFields(logrus.Fields{"type": "performance"})
	bootstrapLogger.Logger.SetLevel(logrus.InfoLevel)

	// Load the config file (ignore "could not find config file")
	err = config.ReadConfig(configFilePath, bootstrapLogger)
	if _, missing := err.(errors.ConfigFileMissingError); err != nil && !missing {
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
		Name:     "scrutiny-collector-performance",
		Usage:    "fio performance benchmark collector for scrutiny",
		Version:  version.VERSION,
		Compiled: time.Now(),
		Authors: []*cli.Author{
			{
				Name:  "Scrutiny Contributors",
				Email: "https://github.com/Starosdev/scrutiny",
			},
		},
		Before: func(c *cli.Context) error {
			collectorPerf := "Starosdev/scrutiny/performance"

			var versionInfo string
			if len(goos) > 0 && len(goarch) > 0 {
				versionInfo = fmt.Sprintf("%s.%s-%s", goos, goarch, version.VERSION)
			} else {
				versionInfo = fmt.Sprintf("dev-%s", version.VERSION)
			}

			subtitle := collectorPerf + utils.LeftPad2Len(versionInfo, " ", 65-len(collectorPerf))

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
				Usage:  "Run the scrutiny performance benchmark collector",
				Action: runCollectorAction(config, bootstrapLogger),

				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "Specify the path to the config file",
					},
					&cli.StringFlag{
						Name:    flagApiEndpoint,
						Usage:   "The api server endpoint",
						EnvVars: []string{"COLLECTOR_PERF_API_ENDPOINT", "COLLECTOR_API_ENDPOINT"},
					},
					&cli.StringFlag{
						Name:    flagLogFile,
						Usage:   "Path to file for logging. Leave empty to use STDOUT",
						EnvVars: []string{"COLLECTOR_PERF_LOG_FILE", "COLLECTOR_LOG_FILE"},
					},
					&cli.BoolFlag{
						Name:    "debug",
						Usage:   "Enable debug logging",
						EnvVars: []string{"COLLECTOR_PERF_DEBUG", "COLLECTOR_DEBUG", "DEBUG"},
					},
					&cli.StringFlag{
						Name:    flagHostId,
						Usage:   "Host identifier/label, used for grouping devices",
						Value:   "",
						EnvVars: []string{"COLLECTOR_PERF_HOST_ID", "COLLECTOR_HOST_ID"},
					},
					&cli.StringFlag{
						Name:    "profile",
						Usage:   "Benchmark profile: 'quick' or 'comprehensive'",
						Value:   "",
						EnvVars: []string{"COLLECTOR_PERF_PROFILE"},
					},
					&cli.StringFlag{
						Name:    flagApiToken,
						Usage:   "API token for authenticating with the Scrutiny server",
						EnvVars: []string{"COLLECTOR_PERF_API_TOKEN", "COLLECTOR_API_TOKEN"},
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

// resolveConfigPath returns the first existing collector config file, preferring the
// performance-specific config and falling back to the shared collector config. Defaults to the
// primary performance path when none exist.
func resolveConfigPath() string {
	candidates := []string{
		"/opt/scrutiny/config/collector-performance.yaml",
		"/opt/scrutiny/config/collector-performance.yml",
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
	if c.IsSet(flagHostId) {
		appConfig.Set("host.id", c.String(flagHostId))
	}
	if c.Bool("debug") {
		appConfig.Set("log.level", "DEBUG")
	}
	if c.IsSet(flagLogFile) {
		appConfig.Set(configKeyLogFile, c.String(flagLogFile))
	}
	if c.IsSet(flagApiEndpoint) {
		appConfig.Set("api.endpoint", strings.TrimSuffix(c.String(flagApiEndpoint), "/")+"/")
	}
	if c.IsSet(flagApiToken) {
		appConfig.Set("api.token", c.String(flagApiToken))
	}
	if c.IsSet("profile") {
		appConfig.Set("performance.profile", c.String("profile"))
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

// runCollectorAction builds the cli action that configures and runs the performance collector.
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

		perfCollector, collectorErr := performance.CreateCollector(
			appConfig,
			collectorLogger,
			appConfig.GetString("api.endpoint"),
		)
		if collectorErr != nil {
			return collectorErr
		}

		return perfCollector.Run()
	}
}

// CreateLogger creates a logger for the performance collector
func CreateLogger(appConfig config.Interface) (*logrus.Entry, *os.File, error) {
	logger := logrus.WithFields(logrus.Fields{
		"type": "performance",
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
