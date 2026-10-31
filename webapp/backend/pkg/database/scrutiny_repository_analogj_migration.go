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

// analogJHistoryTarget is the device that 0.9.x history keyed by one legacy scrutiny_uuid
// belongs to. Both tags are needed. A WWN alone does not identify a device: drives behind
// one controller can share one, and deviceHistoryPredicate admits the device_wwn branch
// only while a single device holds that WWN, so history rewritten with a WWN tag alone is
// unreadable for every holder once the WWN is shared.
type analogJHistoryTarget struct {
	DeviceID string
	WWN      string
}

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
	if err := sr.mergeAnalogJWWNDuplicates(ctx, tx, mapping); err != nil {
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
func prepareAnalogJDeviceRepair(ctx context.Context, tx *gorm.DB) (map[string]analogJHistoryTarget, error) {
	var devices []models.Device
	if err := tx.WithContext(ctx).Unscoped().Order("created_at ASC, device_id ASC").Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("could not load devices for AnalogJ migration repair: %w", err)
	}

	mapping := make(map[string]analogJHistoryTarget, len(devices))
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
				target := analogJHistoryTarget{DeviceID: targetID, WWN: targetWWN}
				if err := addAnalogJLegacyMapping(mapping, legacyScrutinyUUID(device.ModelName, device.SerialNumber, ""), target); err != nil {
					return nil, err
				}
			case 1:
				canonical := &devices[candidateIndexes[0]]
				if err := mergeAnalogJDeviceRows(ctx, tx, device, canonical); err != nil {
					return nil, err
				}
				target := analogJHistoryTarget{DeviceID: canonical.DeviceID, WWN: strings.ToLower(strings.TrimSpace(canonical.WWN))}
				if err := addAnalogJLegacyMapping(mapping, legacyScrutinyUUID(device.ModelName, device.SerialNumber, ""), target); err != nil {
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
		target := analogJHistoryTarget{DeviceID: device.DeviceID, WWN: targetWWN}
		if err := addAnalogJLegacyMapping(mapping, legacyScrutinyUUID(device.ModelName, device.SerialNumber, legacyWWN), target); err != nil {
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
// collectors reporting the drive rather than one duplicated row, and so is a group whose
// rows disagree on a non-zero capacity, because enclosure firmware that fabricates a WWN
// can fabricate a serial too and capacity is the cheapest evidence that two rows are two
// drives. Both checks cover the whole group: one disagreeing row leaves every row in
// place, and the /api/device/{id}/merge_into endpoint handles those on request.
//
// Rows can only reach a group with differing model names, because device_id is the primary
// key and is derived from model + serial + WWN, so an identical triple is one row already.
func (sr *scrutinyRepository) mergeAnalogJWWNDuplicates(ctx context.Context, tx *gorm.DB, mapping map[string]analogJHistoryTarget) error {
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
		if reason := analogJUnmergeableReason(devices, indexes); reason != "" {
			sr.logger.Warnf(
				"leaving %d AnalogJ device rows for serial %s unmerged: %s",
				len(indexes),
				canonical.SerialNumber,
				reason,
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
			// The merge writes through to the row, and a third duplicate is compared against
			// what the row now holds rather than what it held before the first merge, so an
			// older legacy cannot overwrite a newer one's metadata.
			if err := tx.WithContext(ctx).Unscoped().Where(queryDeviceID, canonical.DeviceID).First(canonical).Error; err != nil {
				return fmt.Errorf("could not reload merged AnalogJ device %s: %w", canonical.DeviceID, err)
			}
			// The mapping was built before the merge, so entries naming the deleted row still
			// point at it. Repoint them, or its 0.9.x history is rewritten onto a device_id
			// that no longer exists.
			for legacyUUID, target := range mapping {
				if target.DeviceID == legacy.DeviceID {
					mapping[legacyUUID] = analogJHistoryTarget{
						DeviceID: canonical.DeviceID,
						WWN:      strings.ToLower(strings.TrimSpace(canonical.WWN)),
					}
				}
			}
		}
	}

	return nil
}

// analogJUnmergeableReason reports why a group of rows sharing a WWN and serial is not one
// drive, or an empty string when nothing argues against merging them.
func analogJUnmergeableReason(devices []models.Device, indexes []int) string {
	hostID := ""
	var capacity int64
	for _, i := range indexes {
		if current := strings.TrimSpace(devices[i].HostId); current != "" {
			if hostID != "" && hostID != current {
				return "they report different host ids"
			}
			hostID = current
		}
		if current := devices[i].Capacity; current != 0 {
			if capacity != 0 && capacity != current {
				return "they report different capacities"
			}
			capacity = current
		}
	}
	return ""
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
	// Self-test history is keyed by device_id, so carry it to the surviving identity, as
	// mergeLegacyDevice does at registration time. m20260914000001 later re-keys
	// device_identity from device_id, which would cement an orphan left here.
	if tx.Migrator().HasTable(&models.DeviceSelfTest{}) {
		if err := tx.WithContext(ctx).Model(&models.DeviceSelfTest{}).
			Where("device_id = ?", legacy.DeviceID).
			Updates(map[string]interface{}{
				"device_id":       canonical.DeviceID,
				"device_identity": canonical.DeviceID,
			}).Error; err != nil {
			return fmt.Errorf("could not move self-test history off AnalogJ device %s: %w", legacy.DeviceID, err)
		}
	}
	if err := tx.WithContext(ctx).Unscoped().Where(queryDeviceID, legacy.DeviceID).Delete(&models.Device{}).Error; err != nil {
		return fmt.Errorf("could not remove duplicate AnalogJ device %s: %w", legacy.DeviceID, err)
	}
	return nil
}

func addAnalogJLegacyMapping(mapping map[string]analogJHistoryTarget, legacyUUID string, target analogJHistoryTarget) error {
	if existing, ok := mapping[legacyUUID]; ok && existing != target {
		return fmt.Errorf(
			"legacy scrutiny_uuid %s maps to conflicting devices %q/%q and %q/%q",
			legacyUUID, existing.DeviceID, existing.WWN, target.DeviceID, target.WWN,
		)
	}
	mapping[legacyUUID] = target
	return nil
}

func (sr *scrutinyRepository) rewriteAnalogJInfluxHistory(ctx context.Context, mapping map[string]analogJHistoryTarget) error {
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
			target := mapping[legacyUUID]
			sr.logger.Debugf(
				"Rewriting AnalogJ history in bucket %s from scrutiny_uuid %s to device_id %s, device_wwn %s",
				bucket, legacyUUID, target.DeviceID, target.WWN,
			)
			query := analogJInfluxMigrationQuery(bucket, org, legacyUUID, target.DeviceID, target.WWN, retentionSeconds)
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

func analogJInfluxMigrationQuery(bucket, org, legacyUUID, targetDeviceID, targetWWN string, retentionSeconds int64) string {
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
|> set(key: "device_id", value: %q)
|> set(key: "device_wwn", value: %q)
|> to(bucket: %q, org: %q)`, bucket, start, legacyUUID, targetDeviceID, targetWWN, bucket, org)
}
