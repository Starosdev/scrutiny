package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureSecretCreatesAndPersists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")

	first, created, err := EnsureSecret(dir, AuthTokenFile)
	require.NoError(t, err)
	require.True(t, created)
	require.Len(t, first, 64)

	info, err := os.Stat(filepath.Join(dir, AuthTokenFile))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	second, created, err := EnsureSecret(dir, AuthTokenFile)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first, second)
}

func TestEnsureSecretReplacesEmptyFileAndTightensMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, JWTSecretFile)
	require.NoError(t, os.WriteFile(path, nil, 0o644))

	secret, created, err := EnsureSecret(dir, JWTSecretFile)
	require.NoError(t, err)
	require.True(t, created)
	require.NotEmpty(t, secret)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestReadMissingFile(t *testing.T) {
	_, err := Read(t.TempDir(), AuthTokenFile)
	require.ErrorIs(t, err, os.ErrNotExist)
}
