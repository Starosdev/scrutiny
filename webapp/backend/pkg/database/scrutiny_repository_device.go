package database

import (
	"context"
	"fmt"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"gorm.io/gorm/clause"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Device
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// insert device into DB (and update specified columns if device is already registered)
// update device fields that may change: (DeviceType, HostID)
func (sr *scrutinyRepository) RegisterDevice(ctx context.Context, dev models.Device) error {
	if err := sr.gormClient.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "wwn"}},
		DoUpdates: clause.AssignmentColumns([]string{"host_id", "device_name", "device_type", "device_uuid", "device_serial_id", "device_label"}),
	}).Create(&dev).Error; err != nil {
		return err
	}
	return nil
}

// get a list of all devices (only device metadata, no SMART data)
func (sr *scrutinyRepository) GetDevices(ctx context.Context) ([]models.Device, error) {
	//Get a list of all the active devices.
	devices := []models.Device{}
	if err := sr.gormClient.WithContext(ctx).Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("Could not get device summary from DB: %v", err)
	}
	return devices, nil
}

// update device (only metadata) from collector
func (sr *scrutinyRepository) UpdateDevice(ctx context.Context, wwn string, collectorSmartData collector.SmartInfo) (models.Device, error) {
	var device models.Device
	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return device, fmt.Errorf("Could not get device from DB: %v", err)
	}

	//TODO catch GormClient err
	err := device.UpdateFromCollectorSmartInfo(collectorSmartData)
	if err != nil {
		return device, err
	}
	return device, sr.gormClient.Model(&device).Updates(device).Error
}

// Update Device Status
func (sr *scrutinyRepository) UpdateDeviceStatus(ctx context.Context, wwn string, status pkg.DeviceStatus) (models.Device, error) {
	var device models.Device
	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return device, fmt.Errorf("Could not get device from DB: %v", err)
	}

	device.DeviceStatus = pkg.DeviceStatusSet(device.DeviceStatus, status)
	return device, sr.gormClient.Model(&device).Updates(device).Error
}

// ResetDeviceStatus clears all failure flags when device SMART data shows all attributes passing
func (sr *scrutinyRepository) ResetDeviceStatus(ctx context.Context, wwn string) (models.Device, error) {
	var device models.Device
	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return device, fmt.Errorf("Could not get device from DB: %v", err)
	}

	device.DeviceStatus = pkg.DeviceStatusPassed
	return device, sr.gormClient.Model(&device).Updates(device).Error
}

// RecalculateDeviceStatusFromHistory re-evaluates device status from stored SMART data
// with current overrides applied. Used when overrides are added/modified/deleted.
func (sr *scrutinyRepository) RecalculateDeviceStatusFromHistory(ctx context.Context, wwn string) error {
	// 1. Get device to know its protocol and current status
	device, err := sr.GetDeviceDetails(ctx, wwn)
	if err != nil {
		return fmt.Errorf("could not get device: %w", err)
	}

	// 2. Get latest SMART data from InfluxDB (limit 1)
	smartHistory, err := sr.GetSmartAttributeHistory(ctx, wwn, "week", 1, 0, nil)
	if err != nil {
		return fmt.Errorf("could not get SMART history: %w", err)
	}
	if len(smartHistory) == 0 {
		// No SMART data yet, nothing to recalculate
		return nil
	}
	latestSmart := smartHistory[0]

	// 3. Get merged overrides (config + database)
	mergedOverrides := sr.GetMergedOverrides(ctx)

	// 4. Re-evaluate each attribute with overrides applied
	newStatus := pkg.DeviceStatusPassed
	for attrId, attr := range latestSmart.Attributes {
		attrStatus := attr.GetStatus()

		// Apply override logic
		if result := overrides.ApplyWithOverrides(mergedOverrides, device.DeviceProtocol, attrId, wwn); result != nil {
			if result.ShouldIgnore {
				// Attribute is ignored - don't count its failure
				continue
			}
			if result.Status != nil {
				// Force status overrides the stored status
				attrStatus = *result.Status
			}
		}

		// If attribute still has failure status, propagate to device
		if pkg.AttributeStatusHas(attrStatus, pkg.AttributeStatusFailedScrutiny) {
			newStatus = pkg.DeviceStatusSet(newStatus, pkg.DeviceStatusFailedScrutiny)
		}
	}

	// 5. Update device status if changed
	if newStatus == pkg.DeviceStatusPassed && device.DeviceStatus != pkg.DeviceStatusPassed {
		_, err = sr.ResetDeviceStatus(ctx, wwn)
		if err != nil {
			return fmt.Errorf("could not reset device status: %w", err)
		}
		sr.logger.Infof("Device %s status recalculated to passed after override change", wwn)
	} else if newStatus != pkg.DeviceStatusPassed && device.DeviceStatus == pkg.DeviceStatusPassed {
		_, err = sr.UpdateDeviceStatus(ctx, wwn, newStatus)
		if err != nil {
			return fmt.Errorf("could not update device status: %w", err)
		}
		sr.logger.Infof("Device %s status recalculated to failed after override change", wwn)
	}

	return nil
}

func (sr *scrutinyRepository) GetDeviceDetails(ctx context.Context, wwn string) (models.Device, error) {
	var device models.Device

	sr.logger.Debugln("GetDeviceDetails from GORM")

	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return models.Device{}, err
	}

	return device, nil
}

// Update Device Archived State
func (sr *scrutinyRepository) UpdateDeviceArchived(ctx context.Context, wwn string, archived bool) error {
	var device models.Device
	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return fmt.Errorf("Could not get device from DB: %v", err)
	}

	return sr.gormClient.Model(&device).Where("wwn = ?", wwn).Update("archived", archived).Error
}

// Update Device Muted State
func (sr *scrutinyRepository) UpdateDeviceMuted(ctx context.Context, wwn string, muted bool) error {
	var device models.Device
	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return fmt.Errorf("Could not get device from DB: %v", err)
	}

	return sr.gormClient.Model(&device).Where("wwn = ?", wwn).Update("muted", muted).Error
}

// Update Device Label (custom user-provided name)
func (sr *scrutinyRepository) UpdateDeviceLabel(ctx context.Context, wwn string, label string) error {
	var device models.Device
	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return fmt.Errorf("Could not get device from DB: %v", err)
	}

	return sr.gormClient.Model(&device).Where("wwn = ?", wwn).Update("label", label).Error
}

// Update Device Smart Display Mode (user preference for attribute value display)
func (sr *scrutinyRepository) UpdateDeviceSmartDisplayMode(ctx context.Context, wwn string, mode string) error {
	// Validate mode is one of the allowed values
	validModes := map[string]bool{"scrutiny": true, "raw": true, "normalized": true}
	if !validModes[mode] {
		return fmt.Errorf("invalid smart_display_mode: %s (must be 'scrutiny', 'raw', or 'normalized')", mode)
	}

	var device models.Device
	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).First(&device).Error; err != nil {
		return fmt.Errorf("Could not get device from DB: %v", err)
	}

	return sr.gormClient.Model(&device).Where("wwn = ?", wwn).Update("smart_display_mode", mode).Error
}

func (sr *scrutinyRepository) DeleteDevice(ctx context.Context, wwn string) error {
	// Validate WWN format before using in delete predicate (defense-in-depth, DeleteAPI doesn't support params)
	if err := validation.ValidateWWN(wwn); err != nil {
		return fmt.Errorf("invalid WWN: %w", err)
	}

	if err := sr.gormClient.WithContext(ctx).Where("wwn = ?", wwn).Delete(&models.Device{}).Error; err != nil {
		return err
	}

	//delete data from influxdb.
	buckets := []string{
		sr.appConfig.GetString("web.influxdb.bucket"),
		fmt.Sprintf("%s_weekly", sr.appConfig.GetString("web.influxdb.bucket")),
		fmt.Sprintf("%s_monthly", sr.appConfig.GetString("web.influxdb.bucket")),
		fmt.Sprintf("%s_yearly", sr.appConfig.GetString("web.influxdb.bucket")),
	}

	for _, bucket := range buckets {
		sr.logger.Infof("Deleting data for %s in bucket: %s", wwn, bucket)
		if err := sr.influxClient.DeleteAPI().DeleteWithName(
			ctx,
			sr.appConfig.GetString("web.influxdb.org"),
			bucket,
			time.Now().AddDate(-10, 0, 0),
			time.Now(),
			fmt.Sprintf(`device_wwn="%s"`, wwn),
		); err != nil {
			return err
		}
	}

	return nil
}
