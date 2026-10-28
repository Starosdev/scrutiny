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
			// The links are replaced with the snapshot that owns them. A drive
			// removed from an array, or a mount that moved to another disk,
			// must not leave an attribution behind that outlives the fact.
			if err := tx.Where("host_id = ?", hostID).Delete(&models.FilesystemDevice{}).Error; err != nil {
				return err
			}
		}

		if len(payload.Filesystems) > 0 {
			if err := tx.Create(&payload.Filesystems).Error; err != nil {
				return err
			}
		}

		if links := filesystemDeviceLinks(payload.Filesystems); len(links) > 0 {
			if err := tx.Create(&links).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// filesystemDeviceLinks flattens each snapshot's backing devices into link rows.
// A snapshot with no backing devices contributes none, which is how an
// unattributed filesystem stays unattributed instead of becoming attributed to
// nothing.
func filesystemDeviceLinks(filesystems []models.FilesystemCapacity) []models.FilesystemDevice {
	links := make([]models.FilesystemDevice, 0)
	for i := range filesystems {
		filesystem := &filesystems[i]
		seen := make(map[string]struct{}, len(filesystem.BackingDevices))
		for _, deviceName := range filesystem.BackingDevices {
			if deviceName == "" {
				continue
			}
			// A collector is free to repeat a disk (both halves of a mirror
			// resolving to one device); the primary key is not.
			if _, duplicate := seen[deviceName]; duplicate {
				continue
			}
			seen[deviceName] = struct{}{}
			links = append(links, models.FilesystemDevice{
				HostID:     filesystem.HostID,
				MountPoint: filesystem.MountPoint,
				DeviceName: deviceName,
			})
		}
	}
	return links
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

	links, err := sr.filesystemDevicesByMount(ctx)
	if err != nil {
		return nil, nil, err
	}

	filesystemsByHost := make(map[string][]models.FilesystemCapacity)
	for i := range filesystems {
		filesystems[i].BackingDevices = links[filesystemMountKey(filesystems[i].HostID, filesystems[i].MountPoint)]
		filesystemsByHost[filesystems[i].HostID] = append(filesystemsByHost[filesystems[i].HostID], filesystems[i])
	}

	hostStatuses := make(map[string]*models.FilesystemHostStatus)
	for i := range hostStatusesList {
		hostStatus := hostStatusesList[i]
		hostStatuses[hostStatus.HostID] = &hostStatus
	}

	return filesystemsByHost, hostStatuses, nil
}

// filesystemsByHostDevice returns the filesystems backed by each drive, keyed
// by host ID and then by the drive's kernel device name. A drive with no
// attributed filesystem is absent from the map rather than present and empty,
// so a caller cannot read "nothing known about this drive" as "no free space".
func (sr *scrutinyRepository) filesystemsByHostDevice(ctx context.Context) (map[string]map[string][]models.FilesystemCapacity, error) {
	filesystemsByHost, _, err := sr.GetFilesystemSummary(ctx)
	if err != nil {
		return nil, err
	}

	byHostDevice := make(map[string]map[string][]models.FilesystemCapacity)
	for hostID, filesystems := range filesystemsByHost {
		for i := range filesystems {
			for _, deviceName := range filesystems[i].BackingDevices {
				if byHostDevice[hostID] == nil {
					byHostDevice[hostID] = make(map[string][]models.FilesystemCapacity)
				}
				byHostDevice[hostID][deviceName] = append(byHostDevice[hostID][deviceName], filesystems[i])
			}
		}
	}
	return byHostDevice, nil
}

// filesystemDevicesByMount loads every link, grouped by the filesystem it
// belongs to. One query serves both the per-host and per-drive views: the table
// holds one row per (mount, disk) pair, so it is bounded by the number of
// mounts across the fleet.
func (sr *scrutinyRepository) filesystemDevicesByMount(ctx context.Context) (map[string][]string, error) {
	links := []models.FilesystemDevice{}
	if err := sr.gormClient.WithContext(ctx).Order("host_id ASC, mount_point ASC, device_name ASC").Find(&links).Error; err != nil {
		return nil, fmt.Errorf("could not get filesystem device links from DB: %v", err)
	}

	byMount := make(map[string][]string)
	for _, link := range links {
		key := filesystemMountKey(link.HostID, link.MountPoint)
		byMount[key] = append(byMount[key], link.DeviceName)
	}
	return byMount, nil
}

// filesystemMountKey identifies a filesystem by its primary key. The separator
// is a null byte, which cannot appear in a host ID or a mount point, so no pair
// of distinct filesystems can collide on one key.
func filesystemMountKey(hostID string, mountPoint string) string {
	return hostID + "\x00" + mountPoint
}
