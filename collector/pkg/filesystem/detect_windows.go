//go:build windows

package filesystem

import "errors"

func statfsForPath(_ string) (statfsResult, error) {
	return statfsResult{}, errors.New("filesystem stat not supported on Windows")
}
