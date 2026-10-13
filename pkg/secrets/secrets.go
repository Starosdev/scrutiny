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
	"syscall"
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

// linkFile publishes a temp file under its final name without overwriting. Tests replace it.
var linkFile = os.Link

// EnsureSecret returns the secret stored in dir/name. When the file is missing or empty it generates a
// new random secret (64 hex characters), writes it with mode 0600, and reports created=true.
// The directory is created (0700) if it does not exist.
//
// The value is written to a 0600 temp file in the same directory, synced, and published with a hard
// link, which fails if the target exists. Concurrent callers (for example replicas sharing a volume)
// therefore converge on whichever value landed first: the loser discards its own and returns the
// winner's with created=false. On filesystems without hard links the value is written directly to a
// file created with O_EXCL. A symlink or non-regular file at the target is an error and is never
// followed or modified. The temp file is always removed.
func EnsureSecret(dir, name string) (secret string, created bool, err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, fmt.Errorf("create secret directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)

	for attempt := 0; attempt < 3; attempt++ {
		existing, err := readRegular(path)
		if err == nil && existing != "" {
			return existing, false, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
		if err == nil {
			// Empty leftover: remove it so the exclusive publish below can succeed.
			_ = os.Remove(path)
		}

		buf := make([]byte, secretBytes)
		if _, randErr := rand.Read(buf); randErr != nil {
			return "", false, fmt.Errorf("generate secret: %w", randErr)
		}
		candidate := hex.EncodeToString(buf)

		published, err := publish(dir, name, candidate)
		if err != nil {
			return "", false, err
		}
		if !published {
			continue // another caller won the race; re-read their value
		}
		onDisk, err := readRegular(path)
		if err != nil {
			return "", false, err
		}
		return onDisk, onDisk == candidate, nil
	}
	return "", false, fmt.Errorf("could not settle secret file %s after repeated races", path)
}

// publish stores value at dir/name only if the name does not exist. It reports false when the name
// already exists.
func publish(dir, name, value string) (bool, error) {
	path := filepath.Join(dir, name)
	tmp, err := os.CreateTemp(dir, name+".tmp-*") // created 0600
	if err != nil {
		return false, fmt.Errorf("create temp secret file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	_, werr := tmp.WriteString(value + "\n")
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return false, fmt.Errorf("write secret file %s: %w", path, err)
	}

	lerr := linkFile(tmpName, path)
	switch {
	case lerr == nil:
		return true, nil
	case errors.Is(lerr, os.ErrExist):
		return false, nil
	case hardLinksUnsupported(lerr):
		return writeExclusive(path, value)
	default:
		return false, fmt.Errorf("publish secret file %s: %w", path, lerr)
	}
}

// hardLinksUnsupported reports errors that mean the filesystem cannot create hard links.
func hardLinksUnsupported(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EXDEV) || errors.Is(err, errors.ErrUnsupported)
}

// writeExclusive is the fallback for filesystems without hard links: create the final name with
// O_EXCL (never following a symlink) and write directly.
func writeExclusive(path, value string) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, fmt.Errorf("create secret file %s: %w", path, err)
	}
	_, werr := f.WriteString(value + "\n")
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = os.Remove(path)
		return false, fmt.Errorf("write secret file %s: %w", path, err)
	}
	return true, nil
}

// readRegular reads path but refuses to follow a symlink or read a non-regular file.
func readRegular(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("secret file %s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// Read returns the trimmed contents of dir/name. A missing file returns an error wrapping os.ErrNotExist.
func Read(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
