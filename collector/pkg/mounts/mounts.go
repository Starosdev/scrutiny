// Package mounts parses the mount table (/proc/mounts) for the collectors.
//
// Both the filesystem capacity collector and the fio performance collector need
// to know which filesystems exist, what backs them, and which ones to ignore.
// They share this parser so the exclusion rules cannot drift apart.
package mounts

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// ProcMounts is the Linux mount table.
const ProcMounts = "/proc/mounts"

// Entry is one parsed line of the mount table.
type Entry struct {
	Source     string
	MountPoint string
	FSType     string
	Options    string
}

// ReadOnly reports whether the filesystem is mounted read-only. The option is
// matched whole, so "errors=remount-ro" does not count as read-only.
func (e Entry) ReadOnly() bool {
	for _, opt := range strings.Split(e.Options, ",") {
		if opt == "ro" {
			return true
		}
	}
	return false
}

// pseudoFSTypes are kernel and container filesystems. They are not host
// storage, so neither capacity nor benchmark results mean anything on them.
var pseudoFSTypes = map[string]struct{}{
	"autofs":      {},
	"binfmt_misc": {},
	"bpf":         {},
	"cgroup":      {},
	"cgroup2":     {},
	"configfs":    {},
	"debugfs":     {},
	"devpts":      {},
	"devtmpfs":    {},
	"fusectl":     {},
	"hugetlbfs":   {},
	"mqueue":      {},
	"nsfs":        {},
	"overlay":     {},
	"proc":        {},
	"pstore":      {},
	"securityfs":  {},
	"sysfs":       {},
	"tmpfs":       {},
	"tracefs":     {},
}

// excludedMountPoints are paths a container runtime bind-mounts into every
// container. Docker reports its single-file binds (/etc/hosts and friends) with
// the backing block device as their source, so a device lookup matches them even
// though the mount point is a file, not host storage.
var excludedMountPoints = map[string]struct{}{
	"/etc/hostname":          {},
	"/etc/hosts":             {},
	"/etc/resolv.conf":       {},
	"/opt/scrutiny/config":   {},
	"/opt/scrutiny/influxdb": {},
}

// PseudoFSType reports whether a filesystem type is never host storage.
func PseudoFSType(fsType string) bool {
	_, excluded := pseudoFSTypes[fsType]
	return excluded
}

// ExcludedCapacityFSType reports whether the filesystem capacity collector
// skips a filesystem type. It skips every pseudo-filesystem, and additionally
// zfs, which the dedicated zfs collector reports instead. The benchmark
// collector deliberately does not share that second rule: a zfs dataset is a
// real filesystem a benchmark can write a test file to.
func ExcludedCapacityFSType(fsType string) bool {
	return PseudoFSType(fsType) || fsType == "zfs"
}

// ExcludedMountPoint reports whether a mount point belongs to the container
// runtime rather than to the host.
func ExcludedMountPoint(mountPoint string) bool {
	_, excluded := excludedMountPoints[mountPoint]
	return excluded
}

// Load reads and parses /proc/mounts.
func Load() ([]Entry, error) {
	file, err := os.Open(ProcMounts)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

// Parse reads mount entries from a reader such as /proc/mounts.
func Parse(reader io.Reader) ([]Entry, error) {
	entries := make([]Entry, 0)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		entry := Entry{
			Source:     Unescape(fields[0]),
			MountPoint: Unescape(fields[1]),
			FSType:     fields[2],
		}
		if len(fields) > 3 {
			entry.Options = fields[3]
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// Unescape decodes the octal escapes the kernel writes into the source and
// mount point fields, where space, tab, newline and backslash appear as \040,
// \011, \012 and \134. Without this a mount point containing a space cannot be
// matched or stat'ed.
func Unescape(field string) string {
	if !strings.Contains(field, `\`) {
		return field
	}

	var out strings.Builder
	out.Grow(len(field))
	for i := 0; i < len(field); {
		if field[i] == '\\' && i+3 < len(field) {
			if value, err := strconv.ParseUint(field[i+1:i+4], 8, 8); err == nil {
				out.WriteByte(byte(value))
				i += 4
				continue
			}
		}
		out.WriteByte(field[i])
		i++
	}
	return out.String()
}

// FindForPath returns the entry for the mount point that owns path: the longest
// matching mount point, and the last of those when a path is mounted more than
// once, which is the mount the kernel resolves to.
func FindForPath(entries []Entry, path string) (Entry, bool) {
	best := Entry{}
	found := false
	for _, entry := range entries {
		// TrimSuffix keeps the root mount working: "/" + "/" would be "//",
		// which no path starts with.
		if path != entry.MountPoint && !strings.HasPrefix(path, strings.TrimSuffix(entry.MountPoint, "/")+"/") {
			continue
		}
		if !found || len(entry.MountPoint) >= len(best.MountPoint) {
			best = entry
			found = true
		}
	}
	return best, found
}
