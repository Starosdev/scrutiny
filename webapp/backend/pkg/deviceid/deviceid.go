package deviceid

import (
	"strings"

	"github.com/google/uuid"
)

// ScrutinyDeviceNamespace is a fixed UUIDv5 namespace for generating deterministic device IDs.
// This must never change once deployed, as it would invalidate all existing device IDs.
var ScrutinyDeviceNamespace = uuid.MustParse("a3d5e5b0-7c94-4e4e-bf3d-8c3b0e5e1f2a")

// Generate creates a deterministic UUIDv5 device identifier from the device's
// model name, serial number, and WWN. The result is stable: identical inputs
// always produce the same UUID. Empty components are included as empty strings,
// so a device with no WWN still gets a valid, unique identifier.
func Generate(modelName, serialNumber, wwn string) string {
	input := strings.ToLower(strings.TrimSpace(modelName)) + ":" +
		strings.ToLower(strings.TrimSpace(serialNumber)) + ":" +
		strings.ToLower(strings.TrimSpace(wwn))
	return uuid.NewSHA1(ScrutinyDeviceNamespace, []byte(input)).String()
}
