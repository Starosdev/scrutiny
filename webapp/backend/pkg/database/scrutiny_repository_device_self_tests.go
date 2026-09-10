package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"gorm.io/gorm"
)

const maxDeviceSelfTestsPerDevice = 21

func deviceSelfTestIdentity(device *models.Device) string {
	if strings.TrimSpace(device.WWN) != "" {
		return strings.TrimSpace(device.WWN)
	}
	return device.DeviceID
}

// The controller log is newest first. Observation order also works when the
// power-on counter is missing, wrapped, or reset; raw lifetime order does not.
const selfTestHistoryOrder = "observed_at DESC, log_index ASC, id DESC"
const selfTestLifetimePeriod int64 = 65536

// selfTestLifetimeBounds intersects the possible epochs with both log order and
// the current power-on age. A lone wrapped value can have several valid epochs.
func selfTestLifetimeBounds(entries []collector.AtaSmartSelfTestLogEntry, powerOnHours int64) ([]int64, []int64) {
	minimum := make([]int64, len(entries))
	maximum := make([]int64, len(entries))
	for i := range maximum {
		maximum[i] = -1
	}
	if powerOnHours <= 0 {
		return minimum, maximum
	}
	for _, entry := range entries {
		if entry.LifetimeHours < 0 || int64(entry.LifetimeHours) >= selfTestLifetimePeriod {
			return minimum, maximum
		}
	}
	var lower int64
	for i := len(entries) - 1; i >= 0; i-- {
		raw := int64(entries[i].LifetimeHours)
		lower += (raw - lower%selfTestLifetimePeriod + selfTestLifetimePeriod) % selfTestLifetimePeriod
		minimum[i] = lower
	}
	upper := powerOnHours
	for i, entry := range entries {
		raw := int64(entry.LifetimeHours)
		upper -= (upper%selfTestLifetimePeriod - raw + selfTestLifetimePeriod) % selfTestLifetimePeriod
		if upper < minimum[i] {
			for j := range maximum {
				maximum[j] = -1
			}
			return minimum, maximum
		}
		maximum[i] = upper
	}
	return minimum, maximum
}

func (sr *scrutinyRepository) syncDeviceSelfTests(ctx context.Context, device *models.Device, collectorSmartData *collector.SmartInfo, powerOnHours int64) error {
	if collectorSmartData.Device.Protocol != pkg.DeviceProtocolAta {
		return nil
	}
	entries := collectorSmartData.AtaSmartSelfTestLog.Entries()
	if len(entries) == 0 {
		return nil
	}
	identity := deviceSelfTestIdentity(device)
	observedAt := collectorSmartData.LocalTime.TimeT
	if observedAt <= 0 {
		observedAt = time.Now().Unix()
	}
	minimum, maximum := selfTestLifetimeBounds(entries, powerOnHours)

	return sr.gormClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []models.DeviceSelfTest
		if err := tx.Where("device_identity = ?", identity).Order(selfTestHistoryOrder).Find(&existing).Error; err != nil {
			return err
		}
		// Replayed older collections must not reorder or prune newer history.
		if len(existing) > 0 && existing[0].ObservedAt > observedAt {
			return nil
		}
		used := make(map[uint]bool)
		for index, entry := range entries {
			row := models.DeviceSelfTest{
				DeviceID: device.DeviceID, DeviceWWN: device.WWN, DeviceIdentity: identity,
				TypeValue: entry.Type.Value, TypeString: entry.Type.String,
				StatusValue: entry.Status.Value, StatusString: entry.Status.String,
				StatusPassed: entry.Status.Passed, LifetimeHours: entry.LifetimeHours,
				ObservedAt: observedAt, LogIndex: index, ObservationPowerOnHours: powerOnHours,
			}
			if maximum[index] >= 0 && minimum[index] == maximum[index] {
				hours := maximum[index]
				row.EffectiveLifetimeHours = &hours
			}
			for oldIndex := range existing {
				old := &existing[oldIndex]
				if used[old.ID] || old.TypeValue != row.TypeValue || old.LifetimeHours != row.LifetimeHours {
					continue
				}
				if row.EffectiveLifetimeHours != nil {
					if old.EffectiveLifetimeHours != nil && *old.EffectiveLifetimeHours != *row.EffectiveLifetimeHours {
						continue
					}
					if old.ObservationPowerOnHours > 0 && *row.EffectiveLifetimeHours > old.ObservationPowerOnHours {
						continue
					}
				} else {
					// A newly possible epoch can represent a different test even
					// when the raw value and type match an earlier observation.
					if old.ObservationPowerOnHours > 0 && (maximum[index] > old.ObservationPowerOnHours || powerOnHours-old.ObservationPowerOnHours >= selfTestLifetimePeriod) {
						continue
					}
					if old.EffectiveLifetimeHours != nil && maximum[index] >= 0 {
						if *old.EffectiveLifetimeHours < minimum[index] || *old.EffectiveLifetimeHours > maximum[index] {
							continue
						}
					}
				}
				row.ID, row.CreatedAt = old.ID, old.CreatedAt
				used[old.ID] = true
				break
			}
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		var staleIDs []uint
		if err := tx.Model(&models.DeviceSelfTest{}).Where("device_identity = ?", identity).
			Order(selfTestHistoryOrder).Offset(maxDeviceSelfTestsPerDevice).Pluck("id", &staleIDs).Error; err != nil {
			return err
		}
		if len(staleIDs) == 0 {
			return nil
		}
		return tx.Delete(&models.DeviceSelfTest{}, staleIDs).Error
	})
}

func (sr *scrutinyRepository) GetDeviceSelfTests(ctx context.Context, deviceID string) ([]models.DeviceSelfTest, error) {
	device, err := sr.GetDeviceDetails(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	deviceIdentity := deviceSelfTestIdentity(&device)
	selfTests := []models.DeviceSelfTest{}
	if err := sr.gormClient.WithContext(ctx).
		Where("device_identity = ?", deviceIdentity).
		Order(selfTestHistoryOrder).
		Find(&selfTests).Error; err != nil {
		return nil, fmt.Errorf("could not get device self-tests from DB: %v", err)
	}

	return selfTests, nil
}

func (sr *scrutinyRepository) GetLatestDeviceSelfTest(ctx context.Context, deviceID string) (*models.DeviceSelfTest, error) {
	device, err := sr.GetDeviceDetails(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	identity := deviceSelfTestIdentity(&device)
	var selfTest models.DeviceSelfTest
	err = sr.gormClient.WithContext(ctx).
		Where("device_identity = ?", identity).
		Order(selfTestHistoryOrder).
		First(&selfTest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get latest device self-test from DB: %v", err)
	}
	return &selfTest, nil
}
