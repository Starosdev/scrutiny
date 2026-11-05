package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

// Btrfs failure types, ordered from most to least severe. A notification that
// carries more than one reason reports the most severe one as its FailureType.
const NotifyFailureTypeBtrfsDegraded = "BtrfsDegraded"
const NotifyFailureTypeBtrfsDeviceErrors = "BtrfsDeviceErrors"
const NotifyFailureTypeBtrfsScrubErrors = "BtrfsScrubErrors"

// BtrfsFailureTypes reports which Btrfs problems in current warrant a
// notification, given the state recorded at the previous upload. previous is
// nil on the first upload of a filesystem. With repeat set, a problem that is
// still present notifies again; otherwise only a new problem notifies: a
// filesystem that became degraded, a device error counter that grew, or a scrub
// that finished with errors since the last upload.
func BtrfsFailureTypes(previous *models.BtrfsFilesystem, current *models.BtrfsFilesystem, repeat bool) []string {
	var types []string
	if current.Status == models.BtrfsFilesystemStatusDegraded && (repeat || previous == nil || previous.Status != models.BtrfsFilesystemStatusDegraded) {
		types = append(types, NotifyFailureTypeBtrfsDegraded)
	}
	if btrfsDeviceErrorsGrew(previous, current, repeat) {
		types = append(types, NotifyFailureTypeBtrfsDeviceErrors)
	}
	if btrfsScrubFoundErrors(previous, current, repeat) {
		types = append(types, NotifyFailureTypeBtrfsScrubErrors)
	}
	return types
}

func btrfsDeviceErrorsGrew(previous *models.BtrfsFilesystem, current *models.BtrfsFilesystem, repeat bool) bool {
	before := map[int]models.BtrfsDevice{}
	if previous != nil {
		for _, device := range previous.Devices {
			before[device.DeviceID] = device
		}
	}
	for _, device := range current.Devices {
		if repeat && device.HasErrors() {
			return true
		}
		// A device not seen before counts from zero, so any error on it is new.
		last := before[device.DeviceID]
		if device.ReadIOErrors > last.ReadIOErrors ||
			device.WriteIOErrors > last.WriteIOErrors ||
			device.FlushIOErrors > last.FlushIOErrors ||
			device.CorruptionErrors > last.CorruptionErrors ||
			device.GenerationErrors > last.GenerationErrors {
			return true
		}
	}
	return false
}

func btrfsScrubFoundErrors(previous *models.BtrfsFilesystem, current *models.BtrfsFilesystem, repeat bool) bool {
	if current.ScrubState != models.BtrfsScrubStateFinished || !current.HasErrors() {
		return false
	}
	if repeat || previous == nil || !previous.HasErrors() {
		return true
	}
	// Same error counts can belong to a newer scrub; the finish time tells them apart.
	return !sameTime(previous.ScrubFinishedAt, current.ScrubFinishedAt)
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func btrfsDisplayName(filesystem *models.BtrfsFilesystem) string {
	switch {
	case filesystem.Label != "":
		return filesystem.Label
	case filesystem.MountPoint != "":
		return filesystem.MountPoint
	default:
		return filesystem.UUID
	}
}

func btrfsDeviceErrorLines(filesystem *models.BtrfsFilesystem) []string {
	var lines []string
	for _, device := range filesystem.Devices {
		if device.Missing {
			lines = append(lines, fmt.Sprintf("Device %d (%s): missing", device.DeviceID, device.Path))
		}
		if device.HasErrors() {
			lines = append(lines, fmt.Sprintf("Device %d (%s): write_io_errs=%d read_io_errs=%d flush_io_errs=%d corruption_errs=%d generation_errs=%d",
				device.DeviceID, device.Path, device.WriteIOErrors, device.ReadIOErrors, device.FlushIOErrors, device.CorruptionErrors, device.GenerationErrors))
		}
	}
	return lines
}

// NewBtrfsNotify builds a notification for filesystem carrying every failure
// type in failureTypes, which must be non-empty and ordered by severity.
func NewBtrfsNotify(logger logrus.FieldLogger, appconfig config.Interface, filesystem *models.BtrfsFilesystem, failureTypes []string) Notify {
	name := btrfsDisplayName(filesystem)
	date := time.Now().Format(time.RFC3339)
	scrubErrors := fmt.Sprintf("read=%d csum=%d verify=%d super=%d",
		filesystem.ScrubReadErrors, filesystem.ScrubCsumErrors, filesystem.ScrubVerifyErrors, filesystem.ScrubSuperErrors)

	rows := [][2]string{
		{notifyRowFailureType, strings.Join(failureTypes, ", ")},
		{"Filesystem", name},
		{"Filesystem UUID", filesystem.UUID},
		{"Host Id", filesystem.HostID},
		{"Mount Point", filesystem.MountPoint},
		{"Status", string(filesystem.Status)},
		{"Missing Devices", fmt.Sprintf("%d", filesystem.DeviceMissing)},
		{"Scrub State", string(filesystem.ScrubState)},
		{"Scrub Errors", scrubErrors},
	}
	messageParts := []string{
		fmt.Sprintf("Scrutiny Btrfs problem notification for filesystem: %s", name),
		fmt.Sprintf("Failure Type: %s", strings.Join(failureTypes, ", ")),
		fmt.Sprintf("Filesystem UUID: %s", filesystem.UUID),
		fmt.Sprintf(fmtHostId, filesystem.HostID),
		fmt.Sprintf("Mount Point: %s", filesystem.MountPoint),
		fmt.Sprintf("Status: %s", filesystem.Status),
		fmt.Sprintf("Missing Devices: %d", filesystem.DeviceMissing),
		fmt.Sprintf("Scrub State: %s", filesystem.ScrubState),
		fmt.Sprintf("Scrub Errors: %s", scrubErrors),
	}
	for _, line := range btrfsDeviceErrorLines(filesystem) {
		rows = append(rows, [2]string{"Device Errors", line})
		messageParts = append(messageParts, line)
	}
	rows = append(rows, [2]string{"Date", date})
	messageParts = append(messageParts, "", fmt.Sprintf(fmtDate, date))

	payload := Payload{
		HostId:       filesystem.HostID,
		DeviceType:   "Btrfs",
		DeviceName:   name,
		DeviceSerial: filesystem.UUID,
		DeviceLabel:  filesystem.Label,
		Test:         false,
		Date:         date,
		FailureType:  failureTypes[0],
		Subject:      fmt.Sprintf("Scrutiny Btrfs problem (%s) detected on filesystem: %s", failureTypes[0], name),
		Message:      strings.Join(messageParts, "\n"),
	}
	payload.HTMLMessage = formatNotificationHTML(
		payload.Subject,
		"Scrutiny Btrfs filesystem notification",
		"BTRFS PROBLEM",
		"#dc3545",
		rows,
		notifyFooterText,
	)

	return Notify{
		Logger:  logger,
		Config:  appconfig,
		Payload: payload,
	}
}
