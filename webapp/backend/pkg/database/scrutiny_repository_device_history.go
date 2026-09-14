package database

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
// WWN leaves them out rather than guessing.
func deviceHistoryPredicate(deviceID, wwn string, wwnUnique bool) string {
	predicate := fmt.Sprintf(`r["device_id"] == %s`, strconv.Quote(deviceID))
	if wwnUnique {
		predicate += fmt.Sprintf(` or (not exists r["device_id"] and r["device_wwn"] == %s)`, strconv.Quote(wwn))
	}
	return predicate
}
