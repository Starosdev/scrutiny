package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/pkg/secrets"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/errors"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func newTestConfig(t *testing.T, dir string) config.Interface {
	t.Helper()
	t.Setenv("SCRUTINY_WEB_AUTH_ENABLED", "")
	t.Setenv("SCRUTINY_WEB_AUTH_TOKEN", "")
	cfg, err := config.Create()
	require.NoError(t, err)
	cfg.Set("web.secrets.dir", dir)
	return cfg
}

func TestAuthIsEnabledByDefault(t *testing.T) {
	require.True(t, IsAuthEnabled(newTestConfig(t, t.TempDir())))
}

func TestEnsureStartupAuth_HubSpokeWithoutTokenFails(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(t, dir)

	err := EnsureStartupAuth(cfg, logrus.New(), false)

	var validationErr errors.ConfigValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Contains(t, err.Error(), "web.auth.token")
	require.Contains(t, err.Error(), "web.auth.enabled")
	entries, _ := os.ReadDir(dir)
	require.Empty(t, entries, "hub/spoke must not generate secrets")
}

func TestEnsureStartupAuth_OmnibusGeneratesAndPersistsToken(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(t, dir)
	logger, hook := logtest.NewNullLogger()

	require.NoError(t, EnsureStartupAuth(cfg, logger, true))

	token := cfg.GetString("web.auth.token")
	require.Len(t, token, 64)
	onDisk, err := secrets.Read(dir, secrets.AuthTokenFile)
	require.NoError(t, err)
	require.Equal(t, token, onDisk)

	info, err := os.Stat(filepath.Join(dir, secrets.AuthTokenFile))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	jwt, err := secrets.Read(dir, secrets.JWTSecretFile)
	require.NoError(t, err)
	require.Equal(t, jwt, cfg.GetString("web.auth.jwt_secret"))

	for _, entry := range hook.AllEntries() {
		require.NotContains(t, entry.Message, token, "token value must never be logged")
	}

	// Second start reuses both values.
	cfg2 := newTestConfig(t, dir)
	require.NoError(t, EnsureStartupAuth(cfg2, logger, true))
	require.Equal(t, token, cfg2.GetString("web.auth.token"))
	require.Equal(t, jwt, cfg2.GetString("web.auth.jwt_secret"))
}

func TestEnsureStartupAuth_ConfiguredTokenIsKept(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(t, dir)
	cfg.Set("web.auth.token", "operator-token")

	require.NoError(t, EnsureStartupAuth(cfg, logrus.New(), false))

	require.Equal(t, "operator-token", cfg.GetString("web.auth.token"))
	_, err := secrets.Read(dir, secrets.AuthTokenFile)
	require.ErrorIs(t, err, os.ErrNotExist, "no token file when the operator supplied one")
	// The JWT secret is still persisted so sessions survive restarts.
	require.NotEmpty(t, cfg.GetString("web.auth.jwt_secret"))
}

func TestEnsureStartupAuth_ConfiguredJWTSecretIsKept(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(t, dir)
	cfg.Set("web.auth.token", "operator-token")
	cfg.Set("web.auth.jwt_secret", "operator-jwt")

	require.NoError(t, EnsureStartupAuth(cfg, logrus.New(), false))

	require.Equal(t, "operator-jwt", cfg.GetString("web.auth.jwt_secret"))
	_, err := secrets.Read(dir, secrets.JWTSecretFile)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsureStartupAuth_ExplicitlyDisabledWarnsAndSkips(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(t, dir)
	cfg.Set("web.auth.enabled", false)
	logger, hook := logtest.NewNullLogger()

	require.NoError(t, EnsureStartupAuth(cfg, logger, false))

	require.Equal(t, 1, len(hook.AllEntries()))
	require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	entries, _ := os.ReadDir(dir)
	require.Empty(t, entries)
}

func TestEnsureStartupAuth_OmnibusUnwritableDirFails(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	cfg := newTestConfig(t, filepath.Join(blocker, "config"))

	err := EnsureStartupAuth(cfg, logrus.New(), true)

	var validationErr errors.ConfigValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Contains(t, err.Error(), "web.auth.token")
}

func TestEnsureStartupAuth_JWTPersistFailureDegradesToWarning(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	cfg := newTestConfig(t, filepath.Join(blocker, "config"))
	cfg.Set("web.auth.token", "operator-token")
	logger, hook := logtest.NewNullLogger()

	require.NoError(t, EnsureStartupAuth(cfg, logger, false))
	require.Empty(t, cfg.GetString("web.auth.jwt_secret"))
	require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
}
