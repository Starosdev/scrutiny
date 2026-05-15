//go:build !windows

package filesystem

import (
	"fmt"
	"syscall"
)

func statfsForPath(path string) (statfsResult, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return statfsResult{}, err
	}

	bsize := int64(stat.Bsize)
	if bsize <= 0 {
		return statfsResult{}, fmt.Errorf("unexpected block size %d", stat.Bsize)
	}

	totalBytes := int64(stat.Blocks) * bsize     //nolint:gosec // uint64->int64 overflow only at 9EB+ disk sizes
	availableBytes := int64(stat.Bavail) * bsize //nolint:gosec // uint64->int64 overflow only at 9EB+ disk sizes

	return statfsResult{
		totalBytes:     totalBytes,
		availableBytes: availableBytes,
	}, nil
}
