//go:build linux

package btrfs

import "syscall"

// statfsFree returns the bytes available to unprivileged users at path, as reported by statfs(2).
// This is the same figure `df` and NAS UIs use for "free".
func statfsFree(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
