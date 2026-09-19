package database

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

// ErrAmbiguousWWN is returned by a WWN lookup that matches more than one device.
// device_id is the identity; a WWN can be shared, for example by drives behind one
// RAID controller or drives reporting a vendor default (#851).
var ErrAmbiguousWWN = errors.New("wwn is shared by more than one device")

// deviceHistoryFilter loads a device and returns the Flux predicate that selects its
// InfluxDB points.
func (sr *scrutinyRepository) deviceHistoryFilter(ctx context.Context, deviceID string) (models.Device, string, error) {
	device, err := sr.GetDeviceDetails(ctx, deviceID)
	if err != nil {
		return models.Device{}, "", fmt.Errorf("could not find device %s: %w", deviceID, err)
	}
	wwnUnique, err := sr.wwnIsUnique(ctx, device.WWN)
	if err != nil {
		return models.Device{}, "", err
	}
	return device, deviceHistoryPredicate(device.DeviceID, device.WWN, wwnUnique), nil
}

// wwnIsUnique reports whether exactly one device holds the WWN.
func (sr *scrutinyRepository) wwnIsUnique(ctx context.Context, wwn string) (bool, error) {
	if strings.TrimSpace(wwn) == "" {
		return false, nil
	}
	var count int64
	if err := sr.gormClient.WithContext(ctx).Model(&models.Device{}).Where("wwn = ?", wwn).Count(&count).Error; err != nil {
		return false, fmt.Errorf("could not count devices by wwn: %w", err)
	}
	return count == 1, nil
}

// deviceHistoryPredicate builds the Flux predicate for a device's points. Points tagged with
// its device_id always match. While the device alone holds its WWN, every point with that WWN
// matches too: points written before device_id tagging, temperature aggregates that lost the
// tag, and points tagged with an earlier device_id of the same device, which legacy identity
// reconciliation leaves behind when it re-keys the device. With a shared WWN nothing in those
// points tells the devices apart, so only the device_id matches. Comparisons are guarded with
// exists, as in GetTemperatureNotificationHistory, rather than comparing a column a point may lack.
func deviceHistoryPredicate(deviceID, wwn string, wwnUnique bool) string {
	predicate := fmt.Sprintf(`(exists r["device_id"] and r["device_id"] == %s)`, strconv.Quote(deviceID))
	if wwnUnique {
		predicate += fmt.Sprintf(` or (exists r["device_wwn"] and r["device_wwn"] == %s)`, strconv.Quote(wwn))
	}
	return predicate
}

// deleteDeviceInfluxHistory deletes a device's points from every history bucket. Points tagged with
// its device_id always go. Points are also deleted by device_wwn, the only way to reach untagged
// legacy points, when deleteByWWN is set; callers set it only when no device outside the deletion
// holds that WWN, because a WWN predicate removes every holder's points.
func (sr *scrutinyRepository) deleteDeviceInfluxHistory(ctx context.Context, device *models.Device, deleteByWWN bool) error {
	predicates := []string{fmt.Sprintf("device_id=%q", device.DeviceID)}
	if deleteByWWN && strings.TrimSpace(device.WWN) != "" {
		predicates = append(predicates, fmt.Sprintf("device_wwn=%q", device.WWN))
	}
	for _, bucket := range sr.deviceHistoryBuckets() {
		for _, predicate := range predicates {
			sr.logger.Infof("Deleting history for device %s in bucket %s where %s", device.DeviceID, bucket, predicate)
			if err := sr.influxClient.DeleteAPI().DeleteWithName(
				ctx,
				sr.appConfig.GetString(cfgInfluxDBOrg),
				bucket,
				time.Now().AddDate(-10, 0, 0),
				time.Now().AddDate(10, 0, 0),
				predicate,
			); err != nil {
				return fmt.Errorf("could not delete history for device %s from bucket %q: %w", device.DeviceID, bucket, err)
			}
		}
	}
	return nil
}

// historyOwners attributes aggregated InfluxDB records to registered devices.
type historyOwners struct {
	deviceIDs map[string]struct{}
	// uniqueWWNs maps each WWN held by exactly one device to that device's ID. A shared WWN is
	// left out: a record identified only by that WWN cannot be attributed to one of its holders.
	uniqueWWNs map[string]string
}

func newHistoryOwners(devices []models.Device) historyOwners {
	owners := historyOwners{deviceIDs: map[string]struct{}{}, uniqueWWNs: map[string]string{}}
	holders := map[string]int{}
	for i := range devices {
		owners.deviceIDs[devices[i].DeviceID] = struct{}{}
		if strings.TrimSpace(devices[i].WWN) != "" {
			holders[devices[i].WWN]++
		}
	}
	for i := range devices {
		if holders[devices[i].WWN] == 1 {
			owners.uniqueWWNs[devices[i].WWN] = devices[i].DeviceID
		}
	}
	return owners
}

// deviceFor returns the device a record belongs to: the registered device named by its device_id
// tag, otherwise the device that alone holds its device_wwn. The fallback covers untagged records
// and records tagged with an earlier device_id of that device, as deviceHistoryPredicate does.
func (o historyOwners) deviceFor(values map[string]interface{}) (string, bool) {
	if deviceID, ok := values["device_id"].(string); ok {
		if _, registered := o.deviceIDs[deviceID]; registered {
			return deviceID, true
		}
	}
	deviceWWN, ok := values["device_wwn"].(string)
	if !ok {
		return "", false
	}
	deviceID, ok := o.uniqueWWNs[deviceWWN]
	return deviceID, ok
}
