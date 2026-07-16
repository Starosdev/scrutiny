package startup

import (
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.uber.org/automaxprocs/maxprocs"
)

const (
	NoLogoEnv   = "SCRUTINY_NO_LOGO"
	logLevelKey = "log.level"
)

type StringGetter interface {
	GetString(key string) string
}

func ShouldPrintBanner() bool {
	return !logoSuppressed(os.Getenv(NoLogoEnv))
}

func NewBootstrapLogger(component string, cfg StringGetter) *logrus.Entry {
	logger := logrus.WithField("type", component)
	logger.Logger.SetLevel(bootstrapLevel(cfg))
	return logger
}

func ConfigureMaxProcs(logger *logrus.Entry) {
	_, err := maxprocs.Set(maxprocs.Logger(func(format string, args ...interface{}) {
		logger.Infof(strings.TrimSuffix(format, "\n"), args...)
	}))
	if err != nil {
		logger.Warnf("maxprocs: %v", err)
	}
}

func bootstrapLevel(cfg StringGetter) logrus.Level {
	if level, err := logrus.ParseLevel(cfg.GetString(logLevelKey)); err == nil {
		return level
	}
	return logrus.InfoLevel
}

func logoSuppressed(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}
