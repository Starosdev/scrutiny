//go:build !linux

package btrfs

import "errors"

func statfsFree(string) (int64, error) {
	return 0, errors.New("statfs is not supported on this platform")
}
