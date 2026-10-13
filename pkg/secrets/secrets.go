// Package secrets generates random secrets and persists them as 0600 files in the config directory,
// so a value created at first start survives restarts and can be read by sibling processes.
package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultDir is the config directory inside the Scrutiny containers.
	DefaultDir = "/opt/scrutiny/config"

	// AuthTokenFile holds the generated web.auth.token (omnibus only).
	AuthTokenFile = ".auth-token" //nolint:gosec // file name, not a credential
	// JWTSecretFile holds the generated web.auth.jwt_secret.
	JWTSecretFile = ".jwt-secret"

	secretBytes = 32
)

// EnsureSecret returns the secret stored in dir/name. When the file is missing or empty it generates a
// new random secret (64 hex characters), writes it with mode 0600, and reports created=true.
// The directory is created (0700) if it does not exist.
func EnsureSecret(dir, name string) (secret string, created bool, err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, fmt.Errorf("create secret directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)

	if existing, readErr := Read(dir, name); readErr == nil && existing != "" {
		return existing, false, nil
	} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return "", false, readErr
	}

	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", false, fmt.Errorf("generate secret: %w", err)
	}
	secret = hex.EncodeToString(buf)

	// O_EXCL is not used: an empty leftover file must be replaceable. 0600 applies on create; Chmod
	// covers a pre-existing file with looser permissions.
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return "", false, fmt.Errorf("write secret file %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", false, fmt.Errorf("restrict secret file %s: %w", path, err)
	}
	return secret, true, nil
}

// Read returns the trimmed contents of dir/name. A missing file returns an error wrapping os.ErrNotExist.
func Read(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
