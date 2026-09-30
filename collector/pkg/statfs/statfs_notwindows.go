//go:build !windows

package statfs

import (
	"fmt"
	"syscall"
)

// Stat returns capacity figures for the filesystem containing path.
func Stat(path string) (Result, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Result{}, err
	}

	bsize := int64(stat.Bsize)
	if bsize <= 0 {
		return Result{}, fmt.Errorf("unexpected block size %d", stat.Bsize)
	}

	return Result{
		Total:     int64(stat.Blocks) * bsize,                       //nolint:gosec // uint64->int64 overflow only at 9EB+ disk sizes
		Used:      (int64(stat.Blocks) - int64(stat.Bfree)) * bsize, //nolint:gosec // uint64->int64 overflow only at 9EB+ disk sizes
		Available: int64(stat.Bavail) * bsize,                       //nolint:gosec // uint64->int64 overflow only at 9EB+ disk sizes
	}, nil
}
