package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactSecrets(t *testing.T) {
	settings := map[string]interface{}{
		"web": map[string]interface{}{
			"database": map[string]interface{}{"dsn": "postgres://u:secret@db/x", "type": "postgres"},
			"auth":     map[string]interface{}{"token": ""},
			"influxdb": map[string]interface{}{"token": "influx-secret", "host": "influx"},
		},
	}
	redacted := redactSecrets(settings)
	web := redacted["web"].(map[string]interface{})
	require.Equal(t, "REDACTED", web["database"].(map[string]interface{})["dsn"])
	require.Equal(t, "postgres", web["database"].(map[string]interface{})["type"])
	require.Equal(t, "", web["auth"].(map[string]interface{})["token"], "an empty secret stays visibly empty")
	require.Equal(t, "REDACTED", web["influxdb"].(map[string]interface{})["token"])
	require.Equal(t, "influx", web["influxdb"].(map[string]interface{})["host"])
}
