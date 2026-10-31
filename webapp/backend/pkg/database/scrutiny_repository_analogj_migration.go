package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const analogJScrutinyUUIDMigrationID = "m20260216155600"

var legacyScrutinyNamespace = uuid.MustParse("3ea22b35-682b-49fb-a655-abffed108e48")

// migrateM20260803000000 repairs databases imported from AnalogJ/scrutiny 0.9.x.
// That release replaced device_wwn InfluxDB tags with scrutiny_uuid tags and
// cleared serial-fallback WWNs. Staros retained device_wwn as its history key.
func (sr *scrutinyRepository) migrateM20260803000000(ctx context.Context, tx *gorm.DB) error {
	var legacyMigrationCount int64
	if err := tx.WithContext(ctx).Table("migrations").
		Where("id = ?", analogJScrutinyUUIDMigrationID).
		Count(&legacyMigrationCount).Error; err != nil {
		return fmt.Errorf("could not inspect migration lineage for AnalogJ compatibility: %w", err)
	}
	if legacyMigrationCount == 0 {
		return nil
	}

	mapping, err := prepareAnalogJDeviceRepair(ctx, tx)
	if err != nil {
		return err
	}
	if err := sr.mergeAnalogJWWNDuplicates(ctx, tx); err != nil {
		return err
	}
	if len(mapping) == 0 {
		return nil
	}
	sr.logger.Infof("Repairing AnalogJ/scrutiny 0.9.x history for %d device identities", len(mapping))
	return sr.rewriteAnalogJInfluxHistory(ctx, mapping)
}

// legacyScrutinyUUID reproduces AnalogJ/scrutiny 0.9.x device identity exactly.
// Cross-fork migration needs this value because 0.9.x replaced device_wwn tags
// in InfluxDB with scrutiny_uuid tags before Staros migration runs.
func legacyScrutinyUUID(modelName, serialNumber, wwn string) string {
	return uuid.NewSHA1(legacyScrutinyNamespace, []byte(modelName+serialNumber+wwn)).String()
}

// prepareAnalogJDeviceRepair restores serial-fallback WWNs cleared by AnalogJ/scrutiny
// 0.9.x and merges any duplicate row created by a Staros collector after migration.
// It returns legacy scrutiny_uuid -> current device_wwn mappings for InfluxDB repair.
func prepareAnalogJDeviceRepair(ctx context.Context, tx *gorm.DB) (map[string]string, error) {
	var devices []models.Device
	if err := tx.WithContext(ctx).Unscoped().Order("created_at ASC, device_id ASC").Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("could not load devices for AnalogJ migration repair: %w", err)
	}

	mapping := make(map[string]string, len(devices))
	for i := range devices {
		device := &devices[i]
		originalWWN := strings.TrimSpace(device.WWN)
		serial := strings.TrimSpace(device.SerialNumber)

		if originalWWN == "" && serial != "" {
			candidateIndexes := analogJRepairCandidates(devices, i)
			switch len(candidateIndexes) {
			case 0:
				targetWWN := strings.ToLower(serial)
				targetID := deviceid.GenerateWithFallback(
					device.ModelName,
					device.SerialNumber,
					targetWWN,
					device.DeviceName,
					device.HostId,
				)
				if err := tx.WithContext(ctx).Unscoped().Model(&models.Device{}).
					Where(queryDeviceID, device.DeviceID).
					Updates(map[string]interface{}{"device_id": targetID, "wwn": targetWWN}).Error; err != nil {
					return nil, fmt.Errorf("could not restore serial-fallback WWN for %s: %w", device.DeviceID, err)
				}
				device.DeviceID = targetID
				device.WWN = targetWWN
				if err := addAnalogJLegacyMapping(mapping, legacyScrutinyUUID(device.ModelName, device.SerialNumber, ""), targetWWN); err != nil {
					return nil, err
				}
			case 1:
				canonical := &devices[candidateIndexes[0]]
				if err := mergeAnalogJDeviceRows(ctx, tx, device, canonical); err != nil {
					return nil, err
				}
				if err := addAnalogJLegacyMapping(mapping, legacyScrutinyUUID(device.ModelName, device.SerialNumber, ""), strings.ToLower(strings.TrimSpace(canonical.WWN))); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf(
					"cannot safely repair AnalogJ device %s: found %d matching non-empty WWNs",
					device.DeviceID,
					len(candidateIndexes),
				)
			}
			continue
		}

		if originalWWN == "" {
			continue
		}

		targetWWN := strings.ToLower(originalWWN)
		legacyWWN := originalWWN
		if serial != "" && strings.EqualFold(originalWWN, serial) {
			legacyWWN = ""
		}
		if err := addAnalogJLegacyMapping(mapping, legacyScrutinyUUID(device.ModelName, device.SerialNumber, legacyWWN), targetWWN); err != nil {
			return nil, err
		}

		if device.WWN != targetWWN {
			if err := tx.WithContext(ctx).Unscoped().Model(&models.Device{}).
				Where(queryDeviceID, device.DeviceID).
				Update("wwn", targetWWN).Error; err != nil {
				return nil, fmt.Errorf("could not normalize WWN for %s: %w", device.DeviceID, err)
			}
		}
	}

	return mapping, nil
}

// mergeAnalogJWWNDuplicates collapses the rows AnalogJ/scrutiny 0.9.x created for a
// single drive. 0.9.x keyed devices by a UUID over model + serial + WWN, so a drive
// whose reported model name changed forked into a second row carrying the same WWN and
// the same serial (#1056). Both rows survive into Staros, where device_id is derived
// from the same three fields, and the drive then appears twice with its history split.
//
// Rows that only share a WWN are left alone. Drives behind one controller, or reporting
// a vendor-default WWN, legitimately share it and are told apart by their serials
// (#851), so serial equality is what identifies one physical drive here. A group whose
// rows disagree on a non-empty host_id is also left alone, because that is two
// collectors reporting the drive rather than one duplicated row; the
// /api/device/{id}/merge_into endpoint handles those on request.
func (sr *scrutinyRepository) mergeAnalogJWWNDuplicates(ctx context.Context, tx *gorm.DB) error {
	// Newest row first, and on a tie the row that still reports a model name: 0.9.x
	// blanked the model name on the row that stopped updating.
	const canonicalOrder = "updated_at DESC, (model_name IS NULL OR model_name = '') ASC, device_id ASC"

	var devices []models.Device
	if err := tx.WithContext(ctx).Unscoped().Order(canonicalOrder).Find(&devices).Error; err != nil {
		return fmt.Errorf("could not load devices for AnalogJ duplicate merge: %w", err)
	}

	groupKeys := make([]string, 0, len(devices))
	groups := make(map[string][]int, len(devices))
	for i := range devices {
		wwn := strings.ToLower(strings.TrimSpace(devices[i].WWN))
		serial := strings.ToLower(strings.TrimSpace(devices[i].SerialNumber))
		if wwn == "" || serial == "" {
			continue
		}
		key := wwn + "\x00" + serial
		if _, seen := groups[key]; !seen {
			groupKeys = append(groupKeys, key)
		}
		groups[key] = append(groups[key], i)
	}

	for _, key := range groupKeys {
		indexes := groups[key]
		if len(indexes) < 2 {
			continue
		}
		canonical := &devices[indexes[0]]
		if conflictingHostIDs(devices, indexes) {
			sr.logger.Warnf(
				"leaving %d AnalogJ device rows for serial %s unmerged: they report different host ids",
				len(indexes),
				canonical.SerialNumber,
			)
			continue
		}
		for _, legacyIndex := range indexes[1:] {
			legacy := &devices[legacyIndex]
			sr.logger.Infof(
				"Merging duplicate AnalogJ device row %s into %s (serial %s)",
				legacy.DeviceID,
				canonical.DeviceID,
				canonical.SerialNumber,
			)
			if err := mergeAnalogJDeviceRows(ctx, tx, legacy, canonical); err != nil {
				return err
			}
			if legacy.CreatedAt.Before(canonical.CreatedAt) {
				canonical.CreatedAt = legacy.CreatedAt
			}
		}
	}

	return nil
}

// conflictingHostIDs reports whether the grouped rows name more than one non-empty host.
func conflictingHostIDs(devices []models.Device, indexes []int) bool {
	hostID := ""
	for _, i := range indexes {
		current := strings.TrimSpace(devices[i].HostId)
		if current == "" {
			continue
		}
		if hostID != "" && hostID != current {
			return true
		}
		hostID = current
	}
	return false
}

func analogJRepairCandidates(devices []models.Device, legacyIndex int) []int {
	legacy := &devices[legacyIndex]
	exactCandidates := make([]int, 0, 1)
	serialCandidates := make([]int, 0, 1)
	for i := range devices {
		if i == legacyIndex || strings.TrimSpace(devices[i].WWN) == "" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(devices[i].SerialNumber), strings.TrimSpace(legacy.SerialNumber)) {
			continue
		}
		if devices[i].HostId != "" && legacy.HostId != "" && devices[i].HostId != legacy.HostId {
			continue
		}
		serialCandidates = append(serialCandidates, i)
		if strings.EqualFold(strings.TrimSpace(devices[i].ModelName), strings.TrimSpace(legacy.ModelName)) {
			exactCandidates = append(exactCandidates, i)
		}
	}
	if len(exactCandidates) > 0 {
		return exactCandidates
	}
	return serialCandidates
}

func mergeAnalogJDeviceRows(ctx context.Context, tx *gorm.DB, legacy, canonical *models.Device) error {
	updates := map[string]interface{}{}
	if legacy.CreatedAt.Before(canonical.CreatedAt) {
		updates["created_at"] = legacy.CreatedAt
	}
	if legacy.DeletedAt == nil && canonical.DeletedAt != nil {
		updates["deleted_at"] = nil
	}
	if canonical.Label == "" && legacy.Label != "" {
		updates["label"] = legacy.Label
	}
	if legacy.Archived {
		updates["archived"] = true
	}
	if legacy.Muted {
		updates["muted"] = true
	}
	if !canonical.HasForcedFailure && legacy.HasForcedFailure {
		updates["has_forced_failure"] = true
	}
	if canonical.MissedPingTimeoutOverride == 0 && legacy.MissedPingTimeoutOverride != 0 {
		updates["missed_ping_timeout_override"] = legacy.MissedPingTimeoutOverride
	}
	if (canonical.SmartDisplayMode == "" || canonical.SmartDisplayMode == "scrutiny") &&
		legacy.SmartDisplayMode != "" && legacy.SmartDisplayMode != "scrutiny" {
		updates["smart_display_mode"] = legacy.SmartDisplayMode
	}

	if len(updates) > 0 {
		if err := tx.WithContext(ctx).Unscoped().Model(&models.Device{}).
			Where(queryDeviceID, canonical.DeviceID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("could not preserve metadata while merging AnalogJ device %s: %w", legacy.DeviceID, err)
		}
	}
	if err := tx.WithContext(ctx).Unscoped().Where(queryDeviceID, legacy.DeviceID).Delete(&models.Device{}).Error; err != nil {
		return fmt.Errorf("could not remove duplicate AnalogJ device %s: %w", legacy.DeviceID, err)
	}
	return nil
}

func addAnalogJLegacyMapping(mapping map[string]string, legacyUUID, targetWWN string) error {
	if existing, ok := mapping[legacyUUID]; ok && existing != targetWWN {
		return fmt.Errorf("legacy scrutiny_uuid %s maps to conflicting WWNs %q and %q", legacyUUID, existing, targetWWN)
	}
	mapping[legacyUUID] = targetWWN
	return nil
}

func (sr *scrutinyRepository) rewriteAnalogJInfluxHistory(ctx context.Context, mapping map[string]string) error {
	if sr.appConfig == nil || sr.influxQueryApi == nil || sr.influxClient == nil {
		return fmt.Errorf("InfluxDB client unavailable for AnalogJ history migration")
	}

	baseBucket := sr.appConfig.GetString(cfgInfluxDBBucket)
	buckets := []string{baseBucket, baseBucket + "_weekly", baseBucket + "_monthly", baseBucket + "_yearly"}
	legacyUUIDs := make([]string, 0, len(mapping))
	for legacyUUID := range mapping {
		legacyUUIDs = append(legacyUUIDs, legacyUUID)
	}
	sort.Strings(legacyUUIDs)

	org := sr.appConfig.GetString(cfgInfluxDBOrg)
	deleteStart := time.Unix(0, 0).UTC()
	deleteStop := time.Now().UTC().Add(24 * time.Hour)
	for _, bucket := range buckets {
		bucketInfo, err := findBucketInOrg(ctx, sr.influxClient.BucketsAPI(), org, bucket)
		if err != nil {
			return fmt.Errorf("could not inspect retention for AnalogJ history in bucket %s: %w", bucket, err)
		}
		var retentionSeconds int64
		if len(bucketInfo.RetentionRules) > 0 {
			retentionSeconds = bucketInfo.RetentionRules[0].EverySeconds
		}
		for _, legacyUUID := range legacyUUIDs {
			targetWWN := mapping[legacyUUID]
			sr.logger.Debugf("Rewriting AnalogJ history in bucket %s from scrutiny_uuid %s to device_wwn %s", bucket, legacyUUID, targetWWN)
			query := analogJInfluxMigrationQuery(bucket, org, legacyUUID, targetWWN, retentionSeconds)
			result, err := sr.influxQueryApi.Query(ctx, query)
			if err != nil {
				return fmt.Errorf("could not rewrite AnalogJ history in bucket %s for %s: %w", bucket, legacyUUID, err)
			}
			for result.Next() {
			}
			resultErr := result.Err()
			if closeErr := result.Close(); resultErr == nil {
				resultErr = closeErr
			}
			if resultErr != nil {
				return fmt.Errorf("could not finish AnalogJ history rewrite in bucket %s for %s: %w", bucket, legacyUUID, resultErr)
			}

			if err := sr.influxClient.DeleteAPI().DeleteWithName(
				ctx,
				org,
				bucket,
				deleteStart,
				deleteStop,
				fmt.Sprintf("scrutiny_uuid=%q", legacyUUID),
			); err != nil {
				return fmt.Errorf("could not remove migrated AnalogJ history in bucket %s for %s: %w", bucket, legacyUUID, err)
			}
		}
	}
	return nil
}

func analogJInfluxMigrationQuery(bucket, org, legacyUUID, targetWWN string, retentionSeconds int64) string {
	// Expired shards can remain queryable even though their points can no longer
	// be written. Use the server's retention window, including custom durations.
	start := "0"
	if retentionSeconds > 0 {
		start = fmt.Sprintf("-%ds", retentionSeconds)
	}
	return fmt.Sprintf(`from(bucket: %q)
|> range(start: %s)
|> filter(fn: (r) => r["_measurement"] == "smart" or r["_measurement"] == "temp")
|> filter(fn: (r) => exists r["scrutiny_uuid"] and r["scrutiny_uuid"] == %q)
|> drop(columns: ["scrutiny_uuid"])
|> set(key: "device_wwn", value: %q)
|> to(bucket: %q, org: %q)`, bucket, start, legacyUUID, targetWWN, bucket, org)
}
