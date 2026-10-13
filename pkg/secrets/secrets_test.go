package secrets

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

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

func TestEnsureSecretRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "elsewhere")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, AuthTokenFile)))

	_, _, err := EnsureSecret(dir, AuthTokenFile)
	require.Error(t, err)
	got, _ := os.ReadFile(target)
	require.Equal(t, "x", string(got), "symlink target must not be modified")
}

func TestEnsureSecretConcurrentCallersConverge(t *testing.T) {
	dir := t.TempDir()
	const n = 16
	results := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _, errs[i] = EnsureSecret(dir, AuthTokenFile)
		}(i)
	}
	wg.Wait()
	for i, v := range results {
		require.NoError(t, errs[i])
		require.Equal(t, results[0], v)
		require.Len(t, v, 64)
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temp files left behind")
}

func TestEnsureSecretFallsBackWhenHardLinksUnsupported(t *testing.T) {
	orig := linkFile
	t.Cleanup(func() { linkFile = orig })
	linkFile = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.EPERM} }

	dir := t.TempDir()
	secret, created, err := EnsureSecret(dir, AuthTokenFile)
	require.NoError(t, err)
	require.True(t, created)
	require.Len(t, secret, 64)

	info, err := os.Stat(filepath.Join(dir, AuthTokenFile))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	entries, _ := os.ReadDir(dir)
	require.Len(t, entries, 1, "temp file removed on fallback")

	again, created, err := EnsureSecret(dir, AuthTokenFile)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, secret, again)
}

func TestEnsureSecretOtherLinkErrorsFail(t *testing.T) {
	orig := linkFile
	t.Cleanup(func() { linkFile = orig })
	linkFile = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.EIO} }

	dir := t.TempDir()
	_, _, err := EnsureSecret(dir, AuthTokenFile)
	require.Error(t, err)
	entries, _ := os.ReadDir(dir)
	require.Empty(t, entries)
}

func TestEnsureSecretFallbackConcurrentCallersConverge(t *testing.T) {
	orig := linkFile
	t.Cleanup(func() { linkFile = orig })
	linkFile = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.EPERM} }

	dir := t.TempDir()
	const n = 16
	results := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _, errs[i] = EnsureSecret(dir, AuthTokenFile)
		}(i)
	}
	wg.Wait()
	for i, v := range results {
		require.NoError(t, errs[i])
		require.Len(t, v, 64)
		require.Equal(t, results[0], v)
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temp or lock files left behind")
}

func TestEnsureSecretReplacesEmptyFileNeverReturnsIt(t *testing.T) {
	for _, linkBroken := range []bool{false, true} {
		if linkBroken {
			orig := linkFile
			t.Cleanup(func() { linkFile = orig })
			linkFile = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.EPERM} }
		}
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, AuthTokenFile), nil, 0o600))

		secret, created, err := EnsureSecret(dir, AuthTokenFile)
		require.NoError(t, err)
		require.True(t, created)
		require.Len(t, secret, 64)
		onDisk, err := Read(dir, AuthTokenFile)
		require.NoError(t, err)
		require.Equal(t, secret, onDisk)
		entries, _ := os.ReadDir(dir)
		require.Len(t, entries, 1)
	}
}

func TestEnsureSecretStaleLockIsBroken(t *testing.T) {
	orig := linkFile
	t.Cleanup(func() { linkFile = orig })
	linkFile = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.EPERM} }

	dir := t.TempDir()
	lock := filepath.Join(dir, AuthTokenFile+lockSuffix)
	require.NoError(t, os.WriteFile(lock, nil, 0o600))
	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(lock, old, old))

	secret, _, err := EnsureSecret(dir, AuthTokenFile)
	require.NoError(t, err)
	require.Len(t, secret, 64)
}
