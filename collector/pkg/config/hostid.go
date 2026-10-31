package config

import (
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// KeyHostID is the key every collector groups its uploads under. It is
// exported because the collector binaries set it from their --host-id flag.
const KeyHostID = "host.id"

// machineIDPaths hold a systemd/dbus machine id, which survives a hostname
// change. They are the fallback rather than the first choice because a machine
// id is opaque in the dashboard, where a host id is a heading a person reads.
var machineIDPaths = []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}

var (
	derivedHostIDOnce  sync.Once
	derivedHostIDValue string
)

// GetHostID returns the identity this collector files its uploads under.
//
// Every collector reads it through this one method so that they cannot disagree
// about what an unset host id means. They used to: the SMART collector recorded
// an empty string, the filesystem collector the literal "default", mdadm
// "unknown-host", and btrfs and zfs left the field out entirely. A single
// machine therefore uploaded drives and filesystems under two different host
// ids and nothing was ever attributed, while two unconfigured machines shared
// one id and overwrote each other's filesystem rows on every upload.
//
// An explicit host.id always wins. Without one the identity is derived from the
// machine, because an empty default is the same value on every host.
func (c *configuration) GetHostID() string {
	if explicit := strings.TrimSpace(c.GetString(KeyHostID)); explicit != "" {
		return explicit
	}
	return derivedHostID()
}

// derivedHostID resolves the machine identity once per process and warns that
// it was not configured. A derived id is unique per machine but not permanent:
// it follows the hostname, and in a container the hostname is the container id,
// so recreating the container files the same machine under a new host id.
func derivedHostID() string {
	derivedHostIDOnce.Do(func() {
		derivedHostIDValue = detectHostID(os.Hostname, os.ReadFile)
		if derivedHostIDValue == "" {
			logrus.Warnln("No host.id is configured and no machine identity could be read. This collector uploads under an empty host id, which every other unconfigured host also uses, so their data is merged. Set COLLECTOR_HOST_ID.")
			return
		}
		logrus.Warnf("No host.id is configured; using %q, derived from this machine. Set COLLECTOR_HOST_ID to pin it: a derived id changes with the hostname, and in a container it changes when the container is recreated.", derivedHostIDValue)
	})
	return derivedHostIDValue
}

// detectHostID prefers the hostname, then a machine id file. Both sources are
// injected so the resolution order is testable without touching the host.
func detectHostID(hostname func() (string, error), readFile func(string) ([]byte, error)) string {
	if name, err := hostname(); err == nil {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}

	for _, path := range machineIDPaths {
		raw, err := readFile(path)
		if err != nil {
			continue
		}
		// systemd writes "uninitialized" from an initrd, and a Docker image can
		// ship the file empty. Neither identifies a machine.
		if id := strings.TrimSpace(string(raw)); id != "" && id != "uninitialized" {
			return id
		}
	}

	return ""
}
