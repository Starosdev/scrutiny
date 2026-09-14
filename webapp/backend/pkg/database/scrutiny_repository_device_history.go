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

// deviceHistoryPredicate builds the Flux predicate for a device's points. Points
// tagged with device_id always match on it. Points written before device_id tagging,
// and every downsampled point aggregated before the downsample tasks kept device_id,
// carry only device_wwn. Those can only be attributed while no other device holds the
// same WWN, because nothing in the point tells two such devices apart, so a shared
// WWN leaves them out rather than guessing. Comparisons are guarded with exists, as in
// GetTemperatureNotificationHistory, rather than comparing a column a point may lack.
func deviceHistoryPredicate(deviceID, wwn string, wwnUnique bool) string {
	predicate := fmt.Sprintf(`(exists r["device_id"] and r["device_id"] == %s)`, strconv.Quote(deviceID))
	if wwnUnique {
		predicate += fmt.Sprintf(` or (not exists r["device_id"] and r["device_wwn"] == %s)`, strconv.Quote(wwn))
	}
	return predicate
}

// deleteDeviceInfluxHistory deletes a device's points from every history bucket. Points tagged with
// its device_id always go. Points are also deleted by device_wwn, the only way to reach untagged
// legacy points, when deleteByWWN is set; callers set it only when no device outside the deletion
// holds that WWN, because a WWN predicate removes every holder's points.
func (sr *scrutinyRepository) deleteDeviceInfluxHistory(ctx context.Context, device models.Device, deleteByWWN bool) error {
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

// uniqueWWNDeviceIDs maps each WWN held by exactly one device to that device's ID. A WWN
// shared by several devices is left out: an untagged point with that WWN cannot be
// attributed to any one of them.
func uniqueWWNDeviceIDs(devices []models.Device) map[string]string {
	holders := map[string]int{}
	for i := range devices {
		if strings.TrimSpace(devices[i].WWN) != "" {
			holders[devices[i].WWN]++
		}
	}
	uniqueWWNs := map[string]string{}
	for i := range devices {
		if holders[devices[i].WWN] == 1 {
			uniqueWWNs[devices[i].WWN] = devices[i].DeviceID
		}
	}
	return uniqueWWNs
}

// historyRecordDeviceID returns the device an aggregated InfluxDB record belongs to: its
// device_id tag, or, for a record without one, the device that alone holds its device_wwn.
func historyRecordDeviceID(values map[string]interface{}, uniqueWWNs map[string]string) (string, bool) {
	if deviceID, ok := values["device_id"].(string); ok && deviceID != "" {
		return deviceID, true
	}
	deviceWWN, ok := values["device_wwn"].(string)
	if !ok {
		return "", false
	}
	deviceID, ok := uniqueWWNs[deviceWWN]
	return deviceID, ok
}
