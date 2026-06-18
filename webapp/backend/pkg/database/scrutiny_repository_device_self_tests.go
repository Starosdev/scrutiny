package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxDeviceSelfTestsPerDevice = 21

func deviceSelfTestIdentity(device *models.Device) string {
	if strings.TrimSpace(device.WWN) != "" {
		return strings.TrimSpace(device.WWN)
	}
	return device.DeviceID
}

func (sr *scrutinyRepository) syncDeviceSelfTests(ctx context.Context, device *models.Device, collectorSmartData *collector.SmartInfo) error {
	if collectorSmartData.Device.Protocol != pkg.DeviceProtocolAta {
		return nil
	}

	selfTestEntries := collectorSmartData.AtaSmartSelfTestLog.Entries()
	if len(selfTestEntries) == 0 {
		return nil
	}

	deviceIdentity := deviceSelfTestIdentity(device)

	return sr.gormClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, selfTestEntry := range selfTestEntries {
			row := models.DeviceSelfTest{
				DeviceID:       device.DeviceID,
				DeviceWWN:      device.WWN,
				DeviceIdentity: deviceIdentity,
				TypeValue:      selfTestEntry.Type.Value,
				TypeString:     selfTestEntry.Type.String,
				StatusValue:    selfTestEntry.Status.Value,
				StatusString:   selfTestEntry.Status.String,
				StatusPassed:   selfTestEntry.Status.Passed,
				LifetimeHours:  selfTestEntry.LifetimeHours,
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "device_identity"},
					{Name: "type_value"},
					{Name: "lifetime_hours"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"device_id",
					"device_wwn",
					"type_string",
					"status_value",
					"status_string",
					"status_passed",
					"updated_at",
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}

		var staleIDs []uint
		if err := tx.Model(&models.DeviceSelfTest{}).
			Where("device_identity = ?", deviceIdentity).
			Order("lifetime_hours DESC, updated_at DESC, id DESC").
			Offset(maxDeviceSelfTestsPerDevice).
			Pluck("id", &staleIDs).Error; err != nil {
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
		Order("lifetime_hours DESC, id DESC").
		Find(&selfTests).Error; err != nil {
		return nil, fmt.Errorf("could not get device self-tests from DB: %v", err)
	}

	return selfTests, nil
}
