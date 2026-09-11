package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands the current user's home directory and returns an absolute path.
func ExpandPath(filePath string) (string, error) {
	if strings.HasPrefix(filePath, "~") {
		if len(filePath) > 1 && filePath[1] != '/' && filePath[1] != '\\' {
			return "", errors.New("cannot expand user-specific home dir")
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		filePath = filepath.Join(homeDir, filePath[1:])
	}

	return filepath.Abs(filePath)
}

// FileExists reports whether path resolves to an existing filesystem entry.
func FileExists(filePath string) bool {
	filePath, err := ExpandPath(filePath)
	if err != nil {
		return false
	}

	if _, err := os.Stat(filePath); err != nil {
		return !os.IsNotExist(err)
	}
	return true
}
