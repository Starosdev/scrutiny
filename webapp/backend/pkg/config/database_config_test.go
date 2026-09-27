package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateConfig_DatabaseType(t *testing.T) {
	created, err := Create()
	require.NoError(t, err)
	cfg := created.(*configuration)
	require.NoError(t, cfg.ValidateConfig(), "sqlite is the default")

	cfg.Set("web.database.type", "postgres")
	require.ErrorContains(t, cfg.ValidateConfig(), "web.database.dsn")

	cfg.Set("web.database.dsn", "postgres://scrutiny@db/scrutiny")
	require.NoError(t, cfg.ValidateConfig())

	cfg.Set("web.database.type", "mariadb")
	require.ErrorContains(t, cfg.ValidateConfig(), "must be sqlite or postgres")
}
