//go:build windows

package statfs

import "errors"

// Stat is not supported on Windows.
func Stat(_ string) (Result, error) {
	return Result{}, errors.New("filesystem stat not supported on Windows")
}
