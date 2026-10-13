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
	"time"
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

const (
	lockSuffix      = ".lock"
	staleLockAge    = 30 * time.Second
	lockPollEvery   = 10 * time.Millisecond
	lockWaitTimeout = 10 * time.Second
	maxAttempts     = 5
)

// EnsureSecret returns the secret stored in dir/name. When the file is missing or empty it generates a
// new random secret (64 hex characters), writes it with mode 0600, and reports created=true.
// The directory is created (0700) if it does not exist. It never returns an empty secret without an error.
//
// The value is written to a 0600 temp file in the same directory, synced, and published with a hard
// link, which fails if the target exists. Concurrent callers (for example replicas sharing a volume)
// therefore converge on whichever value landed first: the loser discards its own and returns the
// winner's with created=false. On filesystems without hard links, and to replace an empty leftover,
// the publish happens under an exclusive <name>.lock file: re-check the target, then rename the synced
// temp file over it, so readers never see a partial value. A symlink or non-regular file at the target
// is an error and is never followed or modified. Temp and lock files are always removed.
func EnsureSecret(dir, name string) (secret string, created bool, err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, fmt.Errorf("create secret directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	useLock := false

	for attempt := 0; attempt < maxAttempts; attempt++ {
		existing, readErr := readRegular(path)
		if readErr == nil && existing != "" {
			return existing, false, nil
		}
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return "", false, readErr
		}
		// An empty target is a leftover; it is replaced only under the lock.
		replaceEmpty := readErr == nil

		buf := make([]byte, secretBytes)
		if _, randErr := rand.Read(buf); randErr != nil {
			return "", false, fmt.Errorf("generate secret: %w", randErr)
		}
		candidate := hex.EncodeToString(buf)

		tmpName, err := writeTemp(dir, name, candidate)
		if err != nil {
			return "", false, err
		}

		published := false
		if !useLock && !replaceEmpty {
			lerr := linkFile(tmpName, path)
			switch {
			case lerr == nil:
				published = true
			case errors.Is(lerr, os.ErrExist):
				// Another caller won the race; the next pass re-reads their value.
			case hardLinksUnsupported(lerr):
				useLock = true
			default:
				_ = os.Remove(tmpName)
				return "", false, fmt.Errorf("publish secret file %s: %w", path, lerr)
			}
		} else {
			published, err = publishLocked(path, tmpName)
			if err != nil {
				_ = os.Remove(tmpName)
				return "", false, err
			}
		}
		_ = os.Remove(tmpName) // after a rename this is a harmless ENOENT

		if !published {
			continue
		}
		onDisk, err := readRegular(path)
		if err != nil {
			return "", false, err
		}
		if onDisk == "" {
			continue
		}
		return onDisk, onDisk == candidate, nil
	}
	return "", false, fmt.Errorf("could not settle secret file %s after %d attempts", path, maxAttempts)
}

// writeTemp creates a 0600 temp file in dir holding value, synced to disk.
func writeTemp(dir, name, value string) (string, error) {
	tmp, err := os.CreateTemp(dir, name+".tmp-*") // created 0600
	if err != nil {
		return "", fmt.Errorf("create temp secret file in %s: %w", dir, err)
	}
	_, werr := tmp.WriteString(value + "\n")
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("write secret file in %s: %w", dir, err)
	}
	return tmp.Name(), nil
}

// hardLinksUnsupported reports errors that mean the filesystem cannot create hard links.
func hardLinksUnsupported(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EXDEV) || errors.Is(err, errors.ErrUnsupported)
}

// publishLocked renames tmpName over path while holding path+".lock". Under the lock it re-checks the
// target: a non-empty value means another caller won, so it reports false and publishes nothing.
func publishLocked(path, tmpName string) (bool, error) {
	release, err := acquireLock(path + lockSuffix)
	if err != nil {
		return false, err
	}
	defer release()

	existing, err := readRegular(path)
	if err == nil && existing != "" {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, fmt.Errorf("publish secret file %s: %w", path, err)
	}
	return true, nil
}

// acquireLock creates lockPath exclusively, waiting for a holder to finish. A lock older than
// staleLockAge belongs to a crashed process and is removed.
func acquireLock(lockPath string) (release func(), err error) {
	deadline := time.Now().Add(lockWaitTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("lock %s: %w", lockPath, err)
		}
		if info, statErr := os.Lstat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for lock %s", lockPath)
		}
		time.Sleep(lockPollEvery)
	}
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
