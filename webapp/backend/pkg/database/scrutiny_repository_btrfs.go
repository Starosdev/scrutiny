package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"gorm.io/gorm"
)

const queryBtrfsFilesystemUUID = "filesystem_uuid = ?"

func (sr *scrutinyRepository) RegisterBtrfsFilesystem(ctx context.Context, filesystem *models.BtrfsFilesystem) error {
	filesystem.UpdatedAt = time.Now()

	var existing models.BtrfsFilesystem
	result := sr.gormClient.WithContext(ctx).Where(queryUUID, filesystem.UUID).First(&existing)

	switch {
	case errors.Is(result.Error, gorm.ErrRecordNotFound):
		if err := sr.gormClient.WithContext(ctx).Create(filesystem).Error; err != nil {
			return err
		}
	case result.Error != nil:
		return result.Error
	default:
		if err := sr.gormClient.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"host_id":              filesystem.HostID,
			"label":                filesystem.Label,
			"status":               filesystem.Status,
			"mount_point":          filesystem.MountPoint,
			"device_count":         filesystem.DeviceCount,
			"device_size":          filesystem.DeviceSize,
			"device_allocated":     filesystem.DeviceAllocated,
			"device_unallocated":   filesystem.DeviceUnallocated,
			"device_missing":       filesystem.DeviceMissing,
			"used":                 filesystem.Used,
			"free_estimated":       filesystem.FreeEstimated,
			"free_min":             filesystem.FreeMin,
			"free_statfs":          filesystem.FreeStatfs,
			"data_ratio":           filesystem.DataRatio,
			"metadata_ratio":       filesystem.MetadataRatio,
			"multiple_profiles":    filesystem.MultipleProfiles,
			"data_profile":         filesystem.DataProfile,
			"metadata_profile":     filesystem.MetadataProfile,
			"system_profile":       filesystem.SystemProfile,
			"data_total":           filesystem.DataTotal,
			"data_used":            filesystem.DataUsed,
			"metadata_total":       filesystem.MetadataTotal,
			"metadata_used":        filesystem.MetadataUsed,
			"system_total":         filesystem.SystemTotal,
			"system_used":          filesystem.SystemUsed,
			"scrub_state":          filesystem.ScrubState,
			"scrub_started_at":     filesystem.ScrubStartedAt,
			"scrub_finished_at":    filesystem.ScrubFinishedAt,
			"scrub_duration":       filesystem.ScrubDuration,
			"scrub_total_bytes":    filesystem.ScrubTotalBytes,
			"scrub_scrubbed_bytes": filesystem.ScrubScrubbedBytes,
			"scrub_error_summary":  filesystem.ScrubErrorSummary,
			"scrub_read_errors":    filesystem.ScrubReadErrors,
			"scrub_csum_errors":    filesystem.ScrubCsumErrors,
			"scrub_verify_errors":  filesystem.ScrubVerifyErrors,
			"scrub_super_errors":   filesystem.ScrubSuperErrors,
			"updated_at":           filesystem.UpdatedAt,
		}).Error; err != nil {
			return err
		}
	}

	if err := sr.gormClient.WithContext(ctx).Where(queryBtrfsFilesystemUUID, filesystem.UUID).Delete(&models.BtrfsDevice{}).Error; err != nil {
		return err
	}
	if len(filesystem.Devices) > 0 {
		for i := range filesystem.Devices {
			filesystem.Devices[i].FilesystemUUID = filesystem.UUID
			filesystem.Devices[i].ID = 0
		}
		if err := sr.gormClient.WithContext(ctx).Create(&filesystem.Devices).Error; err != nil {
			return err
		}
	}

	return nil
}

func (sr *scrutinyRepository) GetBtrfsFilesystems(ctx context.Context) ([]models.BtrfsFilesystem, error) {
	filesystems := []models.BtrfsFilesystem{}
	if err := sr.gormClient.WithContext(ctx).Where("archived = ?", false).Find(&filesystems).Error; err != nil {
		return nil, fmt.Errorf("could not get Btrfs filesystems from DB: %v", err)
	}
	return filesystems, nil
}

func (sr *scrutinyRepository) GetBtrfsFilesystemDetails(ctx context.Context, uuid string) (models.BtrfsFilesystem, error) {
	var filesystem models.BtrfsFilesystem
	if err := sr.gormClient.WithContext(ctx).Where(queryUUID, uuid).First(&filesystem).Error; err != nil {
		return models.BtrfsFilesystem{}, err
	}
	var devices []models.BtrfsDevice
	if err := sr.gormClient.WithContext(ctx).Where(queryBtrfsFilesystemUUID, uuid).Order("device_id ASC").Find(&devices).Error; err != nil {
		return filesystem, err
	}
	filesystem.Devices = devices
	return filesystem, nil
}

func (sr *scrutinyRepository) UpdateBtrfsFilesystemArchived(ctx context.Context, uuid string, archived bool) error {
	var filesystem models.BtrfsFilesystem
	if err := sr.gormClient.WithContext(ctx).Where(queryUUID, uuid).First(&filesystem).Error; err != nil {
		return fmt.Errorf(errBtrfsFilesystemNotFound, err)
	}
	return sr.gormClient.Model(&filesystem).Where(queryUUID, uuid).Update("archived", archived).Error
}

func (sr *scrutinyRepository) UpdateBtrfsFilesystemMuted(ctx context.Context, uuid string, muted bool) error {
	var filesystem models.BtrfsFilesystem
	if err := sr.gormClient.WithContext(ctx).Where(queryUUID, uuid).First(&filesystem).Error; err != nil {
		return fmt.Errorf(errBtrfsFilesystemNotFound, err)
	}
	return sr.gormClient.Model(&filesystem).Where(queryUUID, uuid).Update("muted", muted).Error
}

func (sr *scrutinyRepository) UpdateBtrfsFilesystemLabel(ctx context.Context, uuid string, label string) error {
	var filesystem models.BtrfsFilesystem
	if err := sr.gormClient.WithContext(ctx).Where(queryUUID, uuid).First(&filesystem).Error; err != nil {
		return fmt.Errorf(errBtrfsFilesystemNotFound, err)
	}
	return sr.gormClient.Model(&filesystem).Where(queryUUID, uuid).Update("label", label).Error
}

func (sr *scrutinyRepository) DeleteBtrfsFilesystem(ctx context.Context, uuid string) error {
	if err := validation.ValidateUUID(uuid); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}
	if err := sr.gormClient.WithContext(ctx).Where(queryBtrfsFilesystemUUID, uuid).Delete(&models.BtrfsDevice{}).Error; err != nil {
		return err
	}
	if err := sr.gormClient.WithContext(ctx).Where(queryUUID, uuid).Delete(&models.BtrfsFilesystem{}).Error; err != nil {
		return err
	}

	buckets := []string{
		sr.appConfig.GetString(cfgInfluxDBBucket),
		fmt.Sprintf("%s_weekly", sr.appConfig.GetString(cfgInfluxDBBucket)),
		fmt.Sprintf("%s_monthly", sr.appConfig.GetString(cfgInfluxDBBucket)),
		fmt.Sprintf("%s_yearly", sr.appConfig.GetString(cfgInfluxDBBucket)),
	}

	for _, bucket := range buckets {
		if err := sr.influxClient.DeleteAPI().DeleteWithName(
			ctx,
			sr.appConfig.GetString(cfgInfluxDBOrg),
			bucket,
			time.Now().AddDate(-10, 0, 0),
			time.Now(),
			fmt.Sprintf(`filesystem_uuid=%q`, uuid),
		); err != nil {
			return err
		}
	}
	return nil
}

func (sr *scrutinyRepository) GetBtrfsFilesystemsSummary(ctx context.Context) (map[string]*models.BtrfsFilesystem, error) {
	filesystems, err := sr.GetBtrfsFilesystems(ctx)
	if err != nil {
		return nil, err
	}
	summary := make(map[string]*models.BtrfsFilesystem)
	for i := range filesystems {
		summary[filesystems[i].UUID] = &filesystems[i]
	}
	return summary, nil
}

func (sr *scrutinyRepository) SaveBtrfsMetrics(ctx context.Context, filesystem *models.BtrfsFilesystem) error {
	metrics := measurements.BtrfsMetrics{
		Date:              time.Now(),
		FilesystemUUID:    filesystem.UUID,
		HostID:            filesystem.HostID,
		Label:             filesystem.Label,
		DeviceSize:        filesystem.DeviceSize,
		DeviceAllocated:   filesystem.DeviceAllocated,
		DeviceUnallocated: filesystem.DeviceUnallocated,
		DeviceMissing:     filesystem.DeviceMissing,
		Used:              filesystem.Used,
		FreeEstimated:     filesystem.FreeEstimated,
		FreeStatfs:        filesystem.FreeStatfs,
		DataRatio:         filesystem.DataRatio,
		MetadataRatio:     filesystem.MetadataRatio,
		Status:            string(filesystem.Status),
		ScrubState:        string(filesystem.ScrubState),
		ScrubReadErrors:   filesystem.ScrubReadErrors,
		ScrubCsumErrors:   filesystem.ScrubCsumErrors,
		ScrubVerifyErrors: filesystem.ScrubVerifyErrors,
		ScrubSuperErrors:  filesystem.ScrubSuperErrors,
	}
	tags, fields := metrics.Flatten()
	return sr.saveDatapoint(sr.influxWriteApi, "btrfs_filesystem", tags, fields, metrics.Date, ctx)
}

func (sr *scrutinyRepository) GetBtrfsMetricsHistory(ctx context.Context, uuid string, durationKey string) ([]measurements.BtrfsMetrics, error) {
	bucketName := sr.lookupBucketName(durationKey)
	duration := sr.lookupDuration(durationKey)
	queryStr := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["_measurement"] == "btrfs_filesystem")
		|> filter(fn: (r) => r["filesystem_uuid"] == params.uuid)
		|> aggregateWindow(every: 1h, fn: last, createEmpty: false)
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> sort(columns: ["_time"], desc: false)
	`, bucketName, duration[0], duration[1])

	result, err := sr.influxQueryApi.QueryWithParams(ctx, queryStr, map[string]interface{}{"uuid": uuid})
	if err != nil {
		return nil, fmt.Errorf("failed to query Btrfs metrics: %v", err)
	}
	defer result.Close()

	history := []measurements.BtrfsMetrics{}
	for result.Next() {
		metrics, err := measurements.NewBtrfsMetricsFromInfluxDB(result.Record().Values())
		if err != nil {
			sr.logger.Warnf("Failed to parse Btrfs metrics: %v", err)
			continue
		}
		history = append(history, *metrics)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return history, nil
}
