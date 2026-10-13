package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/analogj/scrutiny/pkg/secrets"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
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

	// Where the InfluxDB token came from. Logged when InfluxDB rejects it; the value never is.
	influxTokenSourceConfig = "config (web.influxdb.token or SCRUTINY_WEB_INFLUXDB_TOKEN)" //nolint:gosec // description, not a credential
	influxTokenSourceFile   = "saved file (" + secrets.InfluxTokenFile + ")"
	influxTokenSourceLegacy = "legacy default token"
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
func resolveInfluxToken(appConfig config.Interface, logger logrus.FieldLogger, setupComplete bool) (string, string, error) {
	if token := appConfig.GetString(cfgInfluxDBToken); token != "" {
		if token == LegacyInfluxToken {
			logger.Warnf("web.influxdb.token is the well-known default token, which is public. Rotate it: see docs/UPGRADE_2.0.md, Generated InfluxDB credentials")
		}
		return token, influxTokenSourceConfig, nil
	}

	dir := influxSecretsDir(appConfig)
	if !setupComplete {
		token, created, err := secrets.EnsureSecret(dir, secrets.InfluxTokenFile)
		if err != nil {
			return "", "", fmt.Errorf("generate influxdb token: %w. Set web.influxdb.token, or make %s writable", err, dir)
		}
		if created {
			logger.Infof("generated an InfluxDB admin token and saved it to %s", filepath.Join(dir, secrets.InfluxTokenFile))
		}
		return token, influxTokenSourceFile, nil
	}

	token, err := secrets.Read(dir, secrets.InfluxTokenFile)
	switch {
	case err == nil && token != "":
		return token, influxTokenSourceFile, nil
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return "", "", fmt.Errorf("read influxdb token: %w", err)
	}
	logger.Warnf("InfluxDB was initialized before Scrutiny 2.0 and web.influxdb.token is not set: using the legacy default token, which is public. Rotate it: see docs/UPGRADE_2.0.md, Generated InfluxDB credentials")
	return LegacyInfluxToken, influxTokenSourceLegacy, nil
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

// warnIfInfluxRejectsToken makes the common upgrade failure readable. When an onboarded InfluxDB answers
// 401, the log names where the token came from, so the operator knows whether to fix the setting, the
// saved file, or the restored data. Other errors are left for the normal setup path to report.
func warnIfInfluxRejectsToken(ctx context.Context, client influxdb2.Client, org, source string, logger logrus.FieldLogger) {
	_, err := client.OrganizationsAPI().FindOrganizationByName(ctx, org)
	// The client flattens API errors to text of the form "unauthorized: unauthorized access".
	if err != nil && strings.HasPrefix(strings.ToLower(err.Error()), "unauthorized") {
		logger.Errorf("InfluxDB rejected the token (401). The token came from: %s. Set web.influxdb.token to a valid token for this InfluxDB; see docs/UPGRADE_2.0.md, Generated InfluxDB credentials", source)
	}
}
