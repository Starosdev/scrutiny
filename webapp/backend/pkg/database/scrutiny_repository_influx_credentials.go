package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/analogj/scrutiny/pkg/secrets"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/sirupsen/logrus"
)

const (
	cfgInfluxDBToken        = "web.influxdb.token"         //nolint:gosec // config key name, not a credential
	cfgInfluxDBInitPassword = "web.influxdb.init_password" //nolint:gosec // config key name, not a credential
	cfgSecretsDir           = "web.secrets.dir"            //nolint:gosec // config key name, not a credential

	// LegacyInfluxToken is the admin token every Scrutiny release before 2.0 used when
	// web.influxdb.token was not set. An InfluxDB that was already initialized without a token
	// setting was onboarded with this value, so it stays the fallback for such installs.
	LegacyInfluxToken = "scrutiny-default-admin-token"
)

// influxSecretsDir returns the directory holding generated InfluxDB credentials.
func influxSecretsDir(appConfig config.Interface) string {
	if dir := appConfig.GetString(cfgSecretsDir); dir != "" {
		return dir
	}
	return secrets.DefaultDir
}

// resolveInfluxToken picks the InfluxDB API token.
//
//   - A token set in config or env always wins.
//   - First-time setup (setupComplete=false): generate a token, save it under web.secrets.dir, and use it
//     for onboarding. A leftover file from an interrupted earlier start is reused.
//   - Already onboarded: use the saved token if one exists (a 2.0 install), otherwise the legacy default,
//     because 1.x installs onboarded InfluxDB with it. The legacy path logs a rotation warning.
//
// The token value is never logged.
func resolveInfluxToken(appConfig config.Interface, logger logrus.FieldLogger, setupComplete bool) (string, error) {
	if token := appConfig.GetString(cfgInfluxDBToken); token != "" {
		if token == LegacyInfluxToken {
			logger.Warnf("web.influxdb.token is the well-known default token, which is public. Rotate it: see docs/UPGRADE_2.0.md, Generated InfluxDB credentials")
		}
		return token, nil
	}

	dir := influxSecretsDir(appConfig)
	if !setupComplete {
		token, created, err := secrets.EnsureSecret(dir, secrets.InfluxTokenFile)
		if err != nil {
			return "", fmt.Errorf("generate influxdb token: %w. Set web.influxdb.token, or make %s writable", err, dir)
		}
		if created {
			logger.Infof("generated an InfluxDB admin token and saved it to %s", filepath.Join(dir, secrets.InfluxTokenFile))
		}
		return token, nil
	}

	token, err := secrets.Read(dir, secrets.InfluxTokenFile)
	switch {
	case err == nil && token != "":
		return token, nil
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("read influxdb token: %w", err)
	}
	logger.Warnf("InfluxDB was initialized before Scrutiny 2.0 and web.influxdb.token is not set: using the legacy default token, which is public. Rotate it: see docs/UPGRADE_2.0.md, Generated InfluxDB credentials")
	return LegacyInfluxToken, nil
}

// resolveInfluxPassword picks the admin password used only during first-time InfluxDB setup.
// A configured value wins; otherwise one is generated and saved under web.secrets.dir so the operator
// can still sign in to the InfluxDB UI.
func resolveInfluxPassword(appConfig config.Interface, logger logrus.FieldLogger) (string, error) {
	if password := appConfig.GetString(cfgInfluxDBInitPassword); password != "" {
		return password, nil
	}
	dir := influxSecretsDir(appConfig)
	password, created, err := secrets.EnsureSecret(dir, secrets.InfluxPasswordFile)
	if err != nil {
		return "", fmt.Errorf("generate influxdb admin password: %w. Set web.influxdb.init_password, or make %s writable", err, dir)
	}
	if created {
		logger.Infof("generated an InfluxDB admin password and saved it to %s", filepath.Join(dir, secrets.InfluxPasswordFile))
	}
	return password, nil
}
