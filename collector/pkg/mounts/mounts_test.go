package mounts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A container's mount table: Docker reports its single-file binds with the
// backing block device as their source, and bind-mounted host paths keep the
// real source device and filesystem type.
const containerProcMounts = `overlay / overlay rw,relatime,lowerdir=/var/lib/docker/overlay2/l/ABC 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev tmpfs rw,nosuid,size=65536k,mode=755 0 0
/dev/sda2 /etc/resolv.conf ext4 rw,relatime 0 0
/dev/sda2 /etc/hostname ext4 rw,relatime 0 0
/dev/sdb1 /data ext4 rw,relatime 0 0
/dev/sdc1 /mnt/my\040disk xfs rw,relatime 0 0
/dev/sdd1 /mnt/archive xfs ro,relatime 0 0
`

func TestParse_FieldsAndOptions(t *testing.T) {
	entries, err := Parse(strings.NewReader(containerProcMounts))
	require.NoError(t, err)
	require.Len(t, entries, 8)

	require.Equal(t, "/dev/sdb1", entries[5].Source)
	require.Equal(t, "/data", entries[5].MountPoint)
	require.Equal(t, "ext4", entries[5].FSType)
	require.False(t, entries[5].ReadOnly())
	require.True(t, entries[7].ReadOnly())
}

func TestParse_DecodesOctalEscapes(t *testing.T) {
	entries, err := Parse(strings.NewReader(containerProcMounts))
	require.NoError(t, err)
	require.Equal(t, "/mnt/my disk", entries[6].MountPoint)
}

func TestParse_SkipsShortLines(t *testing.T) {
	entries, err := Parse(strings.NewReader("broken line\n/dev/sdb1 /data ext4 rw 0 0\n"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "/data", entries[0].MountPoint)
}

func TestParse_ToleratesMissingOptions(t *testing.T) {
	entries, err := Parse(strings.NewReader("/dev/sdb1 /data ext4\n"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.False(t, entries[0].ReadOnly())
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "/data", want: "/data"},
		{in: `/mnt/my\040disk`, want: "/mnt/my disk"},
		{in: `/mnt/tab\011here`, want: "/mnt/tab\there"},
		{in: `/mnt/back\134slash`, want: `/mnt/back\slash`},
		{in: `/mnt/new\012line`, want: "/mnt/new\nline"},
		// Not a complete octal escape: left alone rather than mangled.
		{in: `/mnt/trailing\04`, want: `/mnt/trailing\04`},
		{in: `/mnt/not\9xx`, want: `/mnt/not\9xx`},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			require.Equal(t, test.want, Unescape(test.in))
		})
	}
}

func TestReadOnly_MatchesWholeOption(t *testing.T) {
	// "ro" must match an option, not a substring of one such as "remount-ro".
	require.False(t, Entry{Options: "rw,relatime,errors=remount-ro"}.ReadOnly())
	require.True(t, Entry{Options: "ro,relatime"}.ReadOnly())
	require.True(t, Entry{Options: "noatime,ro"}.ReadOnly())
	require.False(t, Entry{Options: ""}.ReadOnly())
}

func TestFindForPath(t *testing.T) {
	entries, err := Parse(strings.NewReader(containerProcMounts))
	require.NoError(t, err)

	tests := []struct {
		name      string
		path      string
		wantMount string
		wantFound bool
	}{
		{name: "exact mount point", path: "/data", wantMount: "/data", wantFound: true},
		{name: "path inside a mount", path: "/data/.scrutiny_perf_bench", wantMount: "/data", wantFound: true},
		{name: "longest match wins over /dev", path: "/dev", wantMount: "/dev", wantFound: true},
		// Regression: the root mount must own paths that nothing else claims,
		// or its read-only and fstype checks are skipped entirely.
		{name: "root mount owns unclaimed paths", path: "/var/tmp/bench", wantMount: "/", wantFound: true},
		{name: "unescaped mount point is matched", path: "/mnt/my disk", wantMount: "/mnt/my disk", wantFound: true},
		{name: "a sibling prefix is not a match", path: "/datastore", wantMount: "/", wantFound: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, found := FindForPath(entries, test.path)
			require.Equal(t, test.wantFound, found)
			require.Equal(t, test.wantMount, entry.MountPoint)
		})
	}
}

func TestFindForPath_LastShadowingMountWins(t *testing.T) {
	// Two mounts at the same path: the kernel resolves to the later one.
	entries, err := Parse(strings.NewReader("/dev/sdb1 /data ext4 ro,relatime 0 0\n/dev/sdc1 /data xfs rw,relatime 0 0\n"))
	require.NoError(t, err)

	entry, found := FindForPath(entries, "/data")
	require.True(t, found)
	require.Equal(t, "/dev/sdc1", entry.Source)
	require.False(t, entry.ReadOnly())
}

func TestFindForPath_NoMatch(t *testing.T) {
	entries, err := Parse(strings.NewReader("/dev/sdb1 /data ext4 rw 0 0\n"))
	require.NoError(t, err)
	_, found := FindForPath(entries, "/elsewhere")
	require.False(t, found)
}

func TestExclusions(t *testing.T) {
	require.True(t, ExcludedFSType("tmpfs"))
	require.True(t, ExcludedFSType("overlay"))
	require.True(t, ExcludedFSType("devtmpfs"))
	require.False(t, ExcludedFSType("ext4"))
	require.False(t, ExcludedFSType("btrfs"))

	require.True(t, ExcludedMountPoint("/etc/resolv.conf"))
	require.True(t, ExcludedMountPoint("/opt/scrutiny/config"))
	require.False(t, ExcludedMountPoint("/data"))
}
