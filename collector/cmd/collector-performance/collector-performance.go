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

var goos string
var goarch string

func main() {
	config, err := config.Create()
	if err != nil {
		fmt.Printf("FATAL: %+v\n", err)
		os.Exit(1)
	}

	// Use separate config file for performance collector
	configFilePath := "/opt/scrutiny/config/collector-performance.yaml"
	configFilePathAlternative := "/opt/scrutiny/config/collector-performance.yml"
	// Fall back to main collector config if performance-specific config doesn't exist
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
	bootstrapLogger := logrus.WithFields(logrus.Fields{"type": "performance"})
	bootstrapLogger.Logger.SetLevel(logrus.InfoLevel)

	// Load the config file
	err = config.ReadConfig(configFilePath, bootstrapLogger)
	if _, ok := err.(errors.ConfigFileMissingError); ok {
		// Ignore "could not find config file"
	} else if err != nil {
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
				Name:  "run",
				Usage: "Run the scrutiny performance benchmark collector",
				Action: func(c *cli.Context) error {
					if c.IsSet("config") {
						err = config.ReadConfig(c.String("config"), bootstrapLogger)
						if err != nil {
							fmt.Printf("Could not find config file at specified path: %s", c.String("config"))
							return err
						}
					}

					// Override config with flags if set
					if c.IsSet("host-id") {
						config.Set("host.id", c.String("host-id"))
					}

					if c.Bool("debug") {
						config.Set("log.level", "DEBUG")
					}

					if c.IsSet("log-file") {
						config.Set("log.file", c.String("log-file"))
					}

					if c.IsSet("api-endpoint") {
						apiEndpoint := strings.TrimSuffix(c.String("api-endpoint"), "/") + "/"
						config.Set("api.endpoint", apiEndpoint)
					}

					if c.IsSet("profile") {
						config.Set("performance.profile", c.String("profile"))
					}

					var collectorLogger *logrus.Entry
				var logFile *os.File
				collectorLogger, logFile, err = CreateLogger(config)
					if logFile != nil {
						defer logFile.Close()
					}
					if err != nil {
						return err
					}

					settingsData, settingsErr := json.MarshalIndent(config.AllSettings(), "", "\t")
					collectorLogger.Debug(string(settingsData), settingsErr)

					perfCollector, err := performance.CreateCollector(
						config,
						collectorLogger,
						config.GetString("api.endpoint"),
					)
					if err != nil {
						return err
					}

					return perfCollector.Run()
				},

				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "Specify the path to the config file",
					},
					&cli.StringFlag{
						Name:    "api-endpoint",
						Usage:   "The api server endpoint",
						EnvVars: []string{"COLLECTOR_PERF_API_ENDPOINT", "COLLECTOR_API_ENDPOINT"},
					},
					&cli.StringFlag{
						Name:    "log-file",
						Usage:   "Path to file for logging. Leave empty to use STDOUT",
						EnvVars: []string{"COLLECTOR_PERF_LOG_FILE", "COLLECTOR_LOG_FILE"},
					},
					&cli.BoolFlag{
						Name:    "debug",
						Usage:   "Enable debug logging",
						EnvVars: []string{"COLLECTOR_PERF_DEBUG", "COLLECTOR_DEBUG", "DEBUG"},
					},
					&cli.StringFlag{
						Name:    "host-id",
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
				},
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(color.HiRedString("ERROR: %v", err))
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
	if appConfig.IsSet("log.file") && len(appConfig.GetString("log.file")) > 0 {
		logFile, err = os.OpenFile(appConfig.GetString("log.file"), os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Logger.Errorf("Failed to open log file %s for output: %s", appConfig.GetString("log.file"), err)
			return nil, logFile, err
		}
		logger.Logger.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}
	return logger, logFile, nil
}
