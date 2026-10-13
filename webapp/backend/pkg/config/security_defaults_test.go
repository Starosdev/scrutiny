package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecurityDefaults(t *testing.T) {
	t.Setenv("SCRUTINY_WEB_AUTH_ENABLED", "")

	cfg, err := Create()
	require.NoError(t, err)

	require.True(t, cfg.GetBool("web.auth.enabled"), "authentication must default to on")
	require.Empty(t, cfg.GetStringSlice("web.listen.trusted_proxies"))
	require.Equal(t, DefaultMaxBodyBytes, cfg.GetInt64("web.listen.max_body_bytes"))
	require.Equal(t, "/opt/scrutiny/reports", cfg.GetString("web.reports.path"))
	require.Equal(t, "/opt/scrutiny/config", cfg.GetString("web.secrets.dir"))
}

func TestAuthCanBeDisabledExplicitly(t *testing.T) {
	t.Setenv("SCRUTINY_WEB_AUTH_ENABLED", "false")

	cfg, err := Create()
	require.NoError(t, err)
	require.False(t, cfg.GetBool("web.auth.enabled"))
}

func TestRemovedDeprecatedNotifyKeysNoLongerFailValidation(t *testing.T) {
	cfg, err := Create()
	require.NoError(t, err)

	cfg.Set("notify.level", 2)
	cfg.Set("notify.filter_attributes", "all")
	require.NoError(t, cfg.(*configuration).ValidateConfig())
}
