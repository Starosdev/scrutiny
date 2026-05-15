package database

import (
	"context"
	"fmt"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"gorm.io/gorm"
)

func (sr *scrutinyRepository) SaveFilesystemSummary(ctx context.Context, payload models.FilesystemSummaryUpload) error {
	return sr.gormClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		hostIDs := make([]string, 0, len(payload.Hosts))
		for _, host := range payload.Hosts {
			hostIDs = append(hostIDs, host.HostID)
			if err := tx.Save(&host).Error; err != nil {
				return err
			}
		}

		for _, hostID := range hostIDs {
			if err := tx.Where("host_id = ?", hostID).Delete(&models.FilesystemCapacity{}).Error; err != nil {
				return err
			}
		}

		if len(payload.Filesystems) > 0 {
			if err := tx.Create(&payload.Filesystems).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (sr *scrutinyRepository) GetFilesystemSummary(ctx context.Context) (map[string][]models.FilesystemCapacity, map[string]*models.FilesystemHostStatus, error) {
	filesystems := []models.FilesystemCapacity{}
	if err := sr.gormClient.WithContext(ctx).Order("host_id ASC, mount_point ASC").Find(&filesystems).Error; err != nil {
		return nil, nil, fmt.Errorf("could not get filesystems from DB: %v", err)
	}

	hostStatusesList := []models.FilesystemHostStatus{}
	if err := sr.gormClient.WithContext(ctx).Order("host_id ASC").Find(&hostStatusesList).Error; err != nil {
		return nil, nil, fmt.Errorf("could not get filesystem host statuses from DB: %v", err)
	}

	filesystemsByHost := make(map[string][]models.FilesystemCapacity)
	for i := range filesystems {
		filesystemsByHost[filesystems[i].HostID] = append(filesystemsByHost[filesystems[i].HostID], filesystems[i])
	}

	hostStatuses := make(map[string]*models.FilesystemHostStatus)
	for i := range hostStatusesList {
		hostStatus := hostStatusesList[i]
		hostStatuses[hostStatus.HostID] = &hostStatus
	}

	return filesystemsByHost, hostStatuses, nil
}
