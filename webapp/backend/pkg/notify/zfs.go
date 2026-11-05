package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

// ZFS failure types, ordered from most to least severe. A notification that
// carries more than one reason reports the most severe one as its FailureType.
const NotifyFailureTypeZFSPoolDegraded = "ZFSPoolDegraded"
const NotifyFailureTypeZFSDeviceErrors = "ZFSDeviceErrors"
const NotifyFailureTypeZFSScrubErrors = "ZFSScrubErrors"
const NotifyFailureTypeZFSUsageHigh = "ZFSUsageHigh"

// ZFSFailureTypes reports which pool problems in current warrant a
// notification, given the state recorded at the previous upload. previous is
// nil on the first upload of a pool. With repeat set, a problem that is still
// present notifies again; otherwise only a new problem notifies: a pool whose
// status changed away from ONLINE, an error total that grew, or a scrub that
// finished with errors since the last upload. usageThreshold is a percent
// compared with the pool's capacity_percent; 0 disables the usage check.
func ZFSFailureTypes(previous *models.ZFSPool, current *models.ZFSPool, repeat bool, usageThreshold int) []string {
	var types []string
	if !current.IsHealthy() && (repeat || previous == nil || previous.Status != current.Status) {
		types = append(types, NotifyFailureTypeZFSPoolDegraded)
	}
	if zfsErrorTotalsGrew(previous, current, repeat) {
		types = append(types, NotifyFailureTypeZFSDeviceErrors)
	}
	if zfsScrubFoundErrors(previous, current, repeat) {
		types = append(types, NotifyFailureTypeZFSScrubErrors)
	}
	var previousUsage float64
	if previous != nil {
		previousUsage = previous.CapacityPercent
	}
	if usageCrossed(previousUsage, current.CapacityPercent, previous == nil, repeat, usageThreshold) {
		types = append(types, NotifyFailureTypeZFSUsageHigh)
	}
	return types
}

func zfsErrorTotalsGrew(previous *models.ZFSPool, current *models.ZFSPool, repeat bool) bool {
	if repeat {
		return current.HasErrors()
	}
	// A pool not seen before counts from zero, so any error on it is new.
	last := models.ZFSPool{}
	if previous != nil {
		last = *previous
	}
	return current.TotalReadErrors > last.TotalReadErrors ||
		current.TotalWriteErrors > last.TotalWriteErrors ||
		current.TotalChecksumErrors > last.TotalChecksumErrors
}

func zfsScrubFoundErrors(previous *models.ZFSPool, current *models.ZFSPool, repeat bool) bool {
	if current.ScrubState != models.ZFSScrubStateFinished || current.ScrubErrorsCount <= 0 {
		return false
	}
	if repeat || previous == nil || previous.ScrubErrorsCount <= 0 {
		return true
	}
	// The same error count can belong to a newer scrub; the end time tells them apart.
	return !sameTime(previous.ScrubEndTime, current.ScrubEndTime)
}

func zfsDisplayName(pool *models.ZFSPool) string {
	switch {
	case pool.Label != "":
		return pool.Label
	case pool.Name != "":
		return pool.Name
	default:
		return pool.GUID
	}
}

// NewZFSNotify builds a notification for pool carrying every failure type in
// failureTypes, which must be non-empty and ordered by severity.
func NewZFSNotify(logger logrus.FieldLogger, appconfig config.Interface, pool *models.ZFSPool, failureTypes []string) Notify {
	name := zfsDisplayName(pool)
	date := time.Now().Format(time.RFC3339)
	errorTotals := fmt.Sprintf("read=%d write=%d checksum=%d", pool.TotalReadErrors, pool.TotalWriteErrors, pool.TotalChecksumErrors)

	rows := [][2]string{
		{notifyRowFailureType, strings.Join(failureTypes, ", ")},
		{"Pool", name},
		{"Pool GUID", pool.GUID},
		{"Host Id", pool.HostID},
		{"Status", string(pool.Status)},
		{"Health", pool.Health},
		{"Usage", fmt.Sprintf("%.1f%%", pool.CapacityPercent)},
		{"Error Totals", errorTotals},
		{"Scrub State", string(pool.ScrubState)},
		{"Scrub Errors", fmt.Sprintf("%d", pool.ScrubErrorsCount)},
		{"Date", date},
	}
	messageParts := []string{
		fmt.Sprintf("Scrutiny ZFS pool problem notification for pool: %s", name),
		fmt.Sprintf("Failure Type: %s", strings.Join(failureTypes, ", ")),
		fmt.Sprintf("Pool GUID: %s", pool.GUID),
		fmt.Sprintf(fmtHostId, pool.HostID),
		fmt.Sprintf("Status: %s", pool.Status),
		fmt.Sprintf("Health: %s", pool.Health),
		fmt.Sprintf("Usage: %.1f%%", pool.CapacityPercent),
		fmt.Sprintf("Error Totals: %s", errorTotals),
		fmt.Sprintf("Scrub State: %s", pool.ScrubState),
		fmt.Sprintf("Scrub Errors: %d", pool.ScrubErrorsCount),
		"",
		fmt.Sprintf(fmtDate, date),
	}

	payload := Payload{
		HostId:       pool.HostID,
		DeviceType:   "ZFS",
		DeviceName:   name,
		DeviceSerial: pool.GUID,
		DeviceLabel:  pool.Label,
		Test:         false,
		Date:         date,
		FailureType:  failureTypes[0],
		Subject:      fmt.Sprintf("Scrutiny ZFS pool problem (%s) detected on pool: %s", failureTypes[0], name),
		Message:      strings.Join(messageParts, "\n"),
	}
	payload.HTMLMessage = formatNotificationHTML(
		payload.Subject,
		"Scrutiny ZFS pool notification",
		"ZFS POOL PROBLEM",
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
