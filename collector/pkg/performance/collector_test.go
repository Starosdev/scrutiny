package performance

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSizeToBytes_Megabytes(t *testing.T) {
	size, err := parseSizeToBytes("256M")
	require.NoError(t, err)
	require.Equal(t, uint64(256*1024*1024), size)
}

func TestParseSizeToBytes_Gigabytes(t *testing.T) {
	size, err := parseSizeToBytes("1G")
	require.NoError(t, err)
	require.Equal(t, uint64(1024*1024*1024), size)
}

func TestParseSizeToBytes_Kilobytes(t *testing.T) {
	size, err := parseSizeToBytes("512K")
	require.NoError(t, err)
	require.Equal(t, uint64(512*1024), size)
}

func TestParseSizeToBytes_Terabytes(t *testing.T) {
	size, err := parseSizeToBytes("2T")
	require.NoError(t, err)
	require.Equal(t, uint64(2*1024*1024*1024*1024), size)
}

func TestParseSizeToBytes_LowercaseNormalized(t *testing.T) {
	size, err := parseSizeToBytes("256m")
	require.NoError(t, err)
	require.Equal(t, uint64(256*1024*1024), size)
}

func TestParseSizeToBytes_RawBytes(t *testing.T) {
	size, err := parseSizeToBytes("1048576")
	require.NoError(t, err)
	require.Equal(t, uint64(1048576), size)
}

func TestParseSizeToBytes_WhitespaceHandled(t *testing.T) {
	size, err := parseSizeToBytes("  256M  ")
	require.NoError(t, err)
	require.Equal(t, uint64(256*1024*1024), size)
}

func TestParseSizeToBytes_EmptyString(t *testing.T) {
	_, err := parseSizeToBytes("")
	require.Error(t, err)
}

func TestParseSizeToBytes_InvalidSuffix(t *testing.T) {
	_, err := parseSizeToBytes("256X")
	require.Error(t, err)
}

func TestParseSizeToBytes_InvalidNumber(t *testing.T) {
	_, err := parseSizeToBytes("abcM")
	require.Error(t, err)
}

func TestCanCreateFileIn_WritableDirectory(t *testing.T) {
	ok, reason := canCreateFileIn(t.TempDir())
	require.True(t, ok)
	require.Empty(t, reason)
}

func TestCanCreateFileIn_LeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	ok, _ := canCreateFileIn(dir)
	require.True(t, ok)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "probe file must be removed")
}

func TestCanCreateFileIn_UnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	// The mount table cannot report this: the filesystem is writable, this
	// process is not allowed to write to the directory.
	dir := t.TempDir() + "/readonly"
	require.NoError(t, os.Mkdir(dir, 0o500))

	ok, reason := canCreateFileIn(dir)
	require.False(t, ok)
	require.Contains(t, reason, "cannot create a file in")
}

func TestCanCreateFileIn_MissingDirectory(t *testing.T) {
	ok, reason := canCreateFileIn(t.TempDir() + "/absent")
	require.False(t, ok)
	require.Contains(t, reason, "cannot create a file in")
}
