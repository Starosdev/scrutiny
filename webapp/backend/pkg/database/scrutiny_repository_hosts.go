package database

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"gorm.io/gorm"
)

func (sr *scrutinyRepository) GetHosts(ctx context.Context) ([]models.HostSummary, error) {
	hosts := []models.HostSummary{}
	err := sr.gormClient.WithContext(ctx).
		Model(&models.Device{}).
		Select(`
			host_id,
			SUM(CASE WHEN archived = false THEN 1 ELSE 0 END) AS active_devices,
			SUM(CASE WHEN archived = true THEN 1 ELSE 0 END) AS archived_devices,
			COUNT(*) AS total_devices
		`).
		Where("TRIM(host_id) <> ''").
		Group("host_id").
		Order("LOWER(host_id) ASC, host_id ASC").
		Scan(&hosts).Error
	if err != nil {
		return nil, fmt.Errorf("could not list SMART hosts: %w", err)
	}

	// A host that uploaded filesystem capacity but no SMART device has no row
	// in the devices table. Listing only devices hid it, and because purging
	// works from this list, nothing could ever remove it. Collectors older
	// than the derived host id filed every unconfigured machine's filesystems
	// under the literal "default", so an upgrade leaves exactly such a host.
	filesystemHosts, err := sr.filesystemOnlyHostIDs(ctx, hosts)
	if err != nil {
		return nil, err
	}
	for _, hostID := range filesystemHosts {
		hosts = append(hosts, models.HostSummary{HostID: hostID})
	}
	sort.Slice(hosts, func(i, j int) bool {
		left, right := strings.ToLower(hosts[i].HostID), strings.ToLower(hosts[j].HostID)
		if left != right {
			return left < right
		}
		return hosts[i].HostID < hosts[j].HostID
	})

	return hosts, nil
}

// filesystemOnlyHostIDs returns the host ids that the filesystem collector
// reported and the SMART collector did not.
func (sr *scrutinyRepository) filesystemOnlyHostIDs(ctx context.Context, smartHosts []models.HostSummary) ([]string, error) {
	known := make(map[string]struct{}, len(smartHosts))
	for i := range smartHosts {
		known[smartHosts[i].HostID] = struct{}{}
	}

	found := []string{}
	for _, model := range []interface{}{&models.FilesystemHostStatus{}, &models.FilesystemCapacity{}} {
		hostIDs := []string{}
		if err := sr.gormClient.WithContext(ctx).
			Model(model).
			Distinct().
			Where("TRIM(host_id) <> ''").
			Pluck("host_id", &hostIDs).Error; err != nil {
			return nil, fmt.Errorf("could not list filesystem hosts: %w", err)
		}
		for _, hostID := range hostIDs {
			if _, seen := known[hostID]; seen {
				continue
			}
			known[hostID] = struct{}{}
			found = append(found, hostID)
		}
	}

	return found, nil
}

func (sr *scrutinyRepository) UpdateHostArchived(ctx context.Context, hostID string, archived bool) (int64, error) {
	result := sr.gormClient.WithContext(ctx).
		Model(&models.Device{}).
		Where("host_id = ?", hostID).
		Update("archived", archived)
	if result.Error != nil {
		return 0, fmt.Errorf("could not update host %q: %w", hostID, result.Error)
	}
	return result.RowsAffected, nil
}

func (sr *scrutinyRepository) PurgeHosts(ctx context.Context, hostIDs []string) ([]models.HostActionResult, error) {
	// Load every device, host-less ones included, to know which WWNs are held outside the selection.
	var devices []models.Device
	if err := sr.gormClient.WithContext(ctx).Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("could not load SMART hosts for purge: %w", err)
	}

	selectedHosts := make(map[string]struct{}, len(hostIDs))
	devicesByHost := make(map[string][]models.Device, len(hostIDs))
	for _, hostID := range hostIDs {
		selectedHosts[hostID] = struct{}{}
	}
	for i := range devices {
		if strings.TrimSpace(devices[i].HostId) == "" {
			continue
		}
		if _, selected := selectedHosts[devices[i].HostId]; selected {
			devicesByHost[devices[i].HostId] = append(devicesByHost[devices[i].HostId], devices[i])
		}
	}

	deletableWWNs := wwnsHeldOnlyBySelectedHosts(devices, selectedHosts)
	results := make([]models.HostActionResult, 0, len(hostIDs))
	for _, hostID := range hostIDs {
		hostDevices := devicesByHost[hostID]
		result := models.HostActionResult{
			HostID:      hostID,
			DeviceCount: int64(len(hostDevices)),
		}
		// A host with no devices can still hold filesystem rows, and those are
		// the only thing left to purge for it. Reporting "host not found" and
		// stopping, as this did, made those rows unreachable.
		hasFilesystems, err := sr.hostHasFilesystemRows(ctx, hostID)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		if len(hostDevices) == 0 && !hasFilesystems {
			result.Error = "host not found"
			results = append(results, result)
			continue
		}
		if err := sr.deleteHostInfluxHistory(ctx, hostDevices, deletableWWNs); err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		if err := sr.gormClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("host_id = ?", hostID).Delete(&models.Device{}).Error; err != nil {
				return err
			}
			// A purged host's filesystem rows outlive every device they could
			// describe. Left behind, they attribute the old host's free space
			// to whatever drive later registers under the same device name on
			// the same host id.
			for _, model := range []interface{}{
				&models.FilesystemDevice{},
				&models.FilesystemCapacity{},
				&models.FilesystemHostStatus{},
			} {
				if err := tx.Where("host_id = ?", hostID).Delete(model).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			result.Error = fmt.Sprintf("could not delete host devices: %v", err)
			results = append(results, result)
			continue
		}

		result.Success = true
		results = append(results, result)
	}
	return results, nil
}

// hostHasFilesystemRows reports whether any filesystem table still holds rows
// for this host.
func (sr *scrutinyRepository) hostHasFilesystemRows(ctx context.Context, hostID string) (bool, error) {
	for _, model := range []interface{}{
		&models.FilesystemHostStatus{},
		&models.FilesystemCapacity{},
		&models.FilesystemDevice{},
	} {
		var count int64
		if err := sr.gormClient.WithContext(ctx).
			Model(model).
			Where("host_id = ?", hostID).
			Count(&count).Error; err != nil {
			return false, fmt.Errorf("could not count filesystem rows for host %q: %w", hostID, err)
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

// wwnsHeldOnlyBySelectedHosts returns the WWNs whose every holder belongs to a selected host. Purging
// the selection may delete points by such a WWN, which also reaches untagged legacy points. A WWN held
// by any other device, host-less ones included, is purged only through each device's device_id.
func wwnsHeldOnlyBySelectedHosts(devices []models.Device, selectedHosts map[string]struct{}) map[string]struct{} {
	heldOutside := map[string]bool{}
	for i := range devices {
		wwn := devices[i].WWN
		if strings.TrimSpace(wwn) == "" {
			continue
		}
		_, selected := selectedHosts[devices[i].HostId]
		outside := strings.TrimSpace(devices[i].HostId) == "" || !selected
		heldOutside[wwn] = heldOutside[wwn] || outside
	}

	deletable := map[string]struct{}{}
	for wwn, outside := range heldOutside {
		if !outside {
			deletable[wwn] = struct{}{}
		}
	}
	return deletable
}

// deleteHostInfluxHistory deletes the history of a purged host's devices. History used to be deleted
// by WWN, so a WWN shared with another host blocked the purge; it is now deleted by device_id, and by
// WWN only for WWNs no device outside the selection holds.
func (sr *scrutinyRepository) deleteHostInfluxHistory(ctx context.Context, devices []models.Device, deletableWWNs map[string]struct{}) error {
	for i := range devices {
		_, deleteByWWN := deletableWWNs[devices[i].WWN]
		if err := sr.deleteDeviceInfluxHistory(ctx, &devices[i], deleteByWWN); err != nil {
			return err
		}
	}
	return nil
}
