package auth

import (
	"fmt"
	"os/exec"

	"github.com/analogj/scrutiny/pkg/secrets"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	configKeyAuthEnabled = "web.auth.enabled"
	configKeyAuthToken   = "web.auth.token"      //nolint:gosec // config key name, not a credential
	configKeyJWTSecret   = "web.auth.jwt_secret" //nolint:gosec // config key name, not a credential
	configKeySecretsDir  = "web.secrets.dir"     //nolint:gosec // config key name, not a credential

	localCollectorBinary = "scrutiny-collector-metrics"
)

// HasLocalCollector reports whether collectors are installed next to the web server, which is the
// omnibus image layout. Only then can the server hand a generated token to its own collectors.
func HasLocalCollector() bool {
	_, err := exec.LookPath(localCollectorBinary)
	return err == nil
}

// EnsureStartupAuth resolves the authentication secrets before the server starts listening.
//
//   - Auth disabled: logs a warning and changes nothing.
//   - web.auth.token set: used as is.
//   - web.auth.token empty, localCollector true (omnibus): a token is generated once, persisted to
//     <web.secrets.dir>/.auth-token with mode 0600, and used. The in-container collectors read the same file.
//   - web.auth.token empty, localCollector false (hub/spoke): returns a ConfigValidationError.
//
// A missing web.auth.jwt_secret is generated and persisted the same way so sessions survive restarts.
// If the JWT secret cannot be persisted the server falls back to a per-process secret and warns.
// Token values are never logged.
func EnsureStartupAuth(appConfig config.Interface, logger logrus.FieldLogger, localCollector bool) error {
	if !appConfig.GetBool(configKeyAuthEnabled) {
		logger.Warn("API authentication is DISABLED (web.auth.enabled=false): every endpoint, including settings, device deletion and collector triggers, is open to anyone who can reach this server")
		return nil
	}

	dir := appConfig.GetString(configKeySecretsDir)

	if appConfig.GetString(configKeyAuthToken) == "" {
		if !localCollector {
			return errors.ConfigValidationError("`web.auth.token` is required because authentication is enabled by default. " +
				"Set web.auth.token in scrutiny.yaml or the SCRUTINY_WEB_AUTH_TOKEN environment variable (collectors must send the same value as COLLECTOR_API_TOKEN), " +
				"or set web.auth.enabled to false (SCRUTINY_WEB_AUTH_ENABLED=false) to run without authentication.")
		}
		token, created, err := secrets.EnsureSecret(dir, secrets.AuthTokenFile)
		if err != nil {
			return errors.ConfigValidationError(fmt.Sprintf("web.auth.token is not set and a generated token could not be stored in %s: %v. Make the directory writable or set web.auth.token.", dir, err))
		}
		appConfig.Set(configKeyAuthToken, token)
		if created {
			logger.Infof("No web.auth.token configured: generated an API token and saved it to %s/%s (mode 0600). Read it with: docker exec <container> cat %s/%s", dir, secrets.AuthTokenFile, dir, secrets.AuthTokenFile)
		} else {
			logger.Infof("No web.auth.token configured: using the generated API token in %s/%s", dir, secrets.AuthTokenFile)
		}
	}

	if appConfig.GetString(configKeyJWTSecret) == "" {
		secret, _, err := secrets.EnsureSecret(dir, secrets.JWTSecretFile)
		if err != nil {
			logger.Warnf("Could not persist a generated JWT secret in %s: %v. Sessions will end when the server restarts; set web.auth.jwt_secret or make the directory writable.", dir, err)
			return nil
		}
		appConfig.Set(configKeyJWTSecret, secret)
	}
	return nil
}
