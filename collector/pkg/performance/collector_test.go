package performance

import (
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
