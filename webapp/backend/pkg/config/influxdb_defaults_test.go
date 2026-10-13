package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfluxDBCredentialDefaultsAreEmpty(t *testing.T) {
	t.Setenv("SCRUTINY_WEB_INFLUXDB_TOKEN", "")
	t.Setenv("SCRUTINY_WEB_INFLUXDB_INIT_PASSWORD", "")

	testConfig, err := Create()
	require.NoError(t, err)

	require.Empty(t, testConfig.GetString("web.influxdb.token"))
	require.Empty(t, testConfig.GetString("web.influxdb.init_password"))
}
