package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	_ "go.uber.org/automaxprocs"

	utils "github.com/analogj/go-util/utils"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/errors"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var goos string
var goarch string

func main() {
	// Create a bootstrap logger early so all startup errors use structured logging
	bootstrapLogger := newBootstrapLogger()

	cfg, err := config.Create()
	if err != nil {
		bootstrapLogger.Fatalf("FATAL: %+v", err)
	}

	if err := readOptionalConfig(cfg, resolveWebConfigPath(), bootstrapLogger); err != nil {
		bootstrapLogger.Error(color.HiRedString("CONFIG ERROR: %v", err))
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

	app := newCLIApp(cfg, bootstrapLogger)

	err = app.Run(os.Args)
	if err != nil {
		bootstrapLogger.Fatal(color.HiRedString("ERROR: %v", err))
	}

}

func newBootstrapLogger() *logrus.Entry {
	bootstrapLogger := logrus.WithFields(logrus.Fields{"type": "web"})
	bootstrapLogger.Logger.SetLevel(logrus.InfoLevel)
	return bootstrapLogger
}

func resolveWebConfigPath() string {
	configFilePath := "/opt/scrutiny/config/scrutiny.yaml"
	configFilePathAlternative := "/opt/scrutiny/config/scrutiny.yml"
	if !utils.FileExists(configFilePath) && utils.FileExists(configFilePathAlternative) {
		return configFilePathAlternative
	}
	return configFilePath
}

func readOptionalConfig(cfg config.Interface, configFilePath string, bootstrapLogger *logrus.Entry) error {
	err := cfg.ReadConfig(configFilePath, bootstrapLogger)
	if _, ok := err.(errors.ConfigFileMissingError); ok {
		return nil
	}
	return err
}

func newCLIApp(cfg config.Interface, bootstrapLogger *logrus.Entry) *cli.App {
	return &cli.App{
		Name:     "scrutiny",
		Usage:    "WebUI for smartd S.M.A.R.T monitoring",
		Version:  version.VERSION,
		Compiled: time.Now(),
		Authors: []*cli.Author{
			{
				Name:  "Jason Kulatunga",
				Email: "jason@thesparktree.com",
			},
		},
		Before: func(c *cli.Context) error {
			color.New(color.FgGreen).Fprintf(c.App.Writer, "%s", scrutinyBanner("github.com/AnalogJ/scrutiny"))
			return nil
		},

		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start the scrutiny server",
				Action: func(c *cli.Context) error {
					fmt.Fprintln(c.App.Writer, c.Command.Usage)
					if c.IsSet("config") {
						if err := cfg.ReadConfig(c.String("config"), bootstrapLogger); err != nil { // Find and read the config file
							//ignore "could not find config file"
							bootstrapLogger.Printf("Could not find config file at specified path: %s", c.String("config"))
							return err
						}
					}

					if c.Bool("debug") {
						cfg.Set("log.level", "DEBUG")
					}

					if c.IsSet("log-file") {
						cfg.Set("log.file", c.String("log-file"))
					}

					webLogger, logFile, err := CreateLogger(cfg)
					if logFile != nil {
						defer logFile.Close()
					}
					if err != nil {
						return err
					}

					settingsData, err := json.Marshal(cfg.AllSettings())
					webLogger.Debug(string(settingsData), err)

					webServer := web.AppEngine{Config: cfg, Logger: webLogger}

					return webServer.Start()
				},

				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "config",
						Usage: "Specify the path to the config file",
					},
					&cli.StringFlag{
						Name:    "log-file",
						Usage:   "Path to file for logging. Leave empty to use STDOUT",
						Value:   "",
						EnvVars: []string{"SCRUTINY_LOG_FILE"},
					},

					&cli.BoolFlag{
						Name:    "debug",
						Usage:   "Enable debug logging",
						EnvVars: []string{"SCRUTINY_DEBUG", "DEBUG"},
					},
				},
			},
		},
	}
}

func scrutinyBanner(projectName string) string {
	versionInfo := "dev-" + version.VERSION
	if len(goos) > 0 && len(goarch) > 0 {
		versionInfo = fmt.Sprintf("%s.%s-%s", goos, goarch, version.VERSION)
	}
	subtitle := projectName + utils.LeftPad2Len(versionInfo, " ", 65-len(projectName))
	return fmt.Sprintf(utils.StripIndent(
		`
		 ___   ___  ____  __  __  ____  ____  _  _  _  _
		/ __) / __)(  _ \(  )(  )(_  _)(_  _)( \( )( \/ )
		\__ \( (__  )   / )(__)(   )(   _)(_  )  (  \  /
		(___/ \___)(_)\_)(______) (__) (____)(_)\_) (__)
		%s

		`), subtitle)
}

func CreateLogger(appConfig config.Interface) (*logrus.Entry, *os.File, error) {
	logger := logrus.WithFields(logrus.Fields{
		"type": "web",
	})
	//set default log level
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
