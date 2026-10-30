package config

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectHostID(t *testing.T) {
	t.Parallel()

	files := func(contents map[string]string) func(string) ([]byte, error) {
		return func(path string) ([]byte, error) {
			if body, found := contents[path]; found {
				return []byte(body), nil
			}
			return nil, os.ErrNotExist
		}
	}

	tests := []struct {
		name     string
		hostname func() (string, error)
		readFile func(string) ([]byte, error)
		expected string
	}{
		{
			name:     "hostname wins",
			hostname: func() (string, error) { return "atlas\n", nil },
			readFile: files(map[string]string{"/etc/machine-id": "7b2c1d"}),
			expected: "atlas",
		},
		{
			name:     "machine id when the hostname is unreadable",
			hostname: func() (string, error) { return "", errors.New("no hostname") },
			readFile: files(map[string]string{"/etc/machine-id": " 7b2c1d\n"}),
			expected: "7b2c1d",
		},
		{
			name:     "an empty or uninitialized machine id is not an identity",
			hostname: func() (string, error) { return "   ", nil },
			readFile: files(map[string]string{
				"/etc/machine-id":          "uninitialized\n",
				"/var/lib/dbus/machine-id": "fallback",
			}),
			expected: "fallback",
		},
		{
			name:     "nothing to derive from",
			hostname: func() (string, error) { return "", nil },
			readFile: files(nil),
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, detectHostID(test.hostname, test.readFile))
		})
	}
}

func TestGetHostIDPrefersExplicitConfig(t *testing.T) {
	t.Parallel()

	testConfig, err := Create()
	require.NoError(t, err)
	testConfig.Set(KeyHostID, "  atlas  ")

	require.Equal(t, "atlas", testConfig.GetHostID())
}

// An unconfigured host id must not resolve to the empty string, because every
// other unconfigured host would resolve to the same one and the server groups
// uploads by this value.
func TestGetHostIDDerivesWhenUnconfigured(t *testing.T) {
	// Not parallel: t.Setenv forbids it, and the config reads COLLECTOR_HOST_ID
	// from the environment, which a CI runner may well have set.
	t.Setenv("COLLECTOR_HOST_ID", "")

	testConfig, err := Create()
	require.NoError(t, err)

	hostname, err := os.Hostname()
	require.NoError(t, err)
	require.NotEmpty(t, hostname, "this test needs a hostname to derive from")

	require.Equal(t, hostname, testConfig.GetHostID())
}
