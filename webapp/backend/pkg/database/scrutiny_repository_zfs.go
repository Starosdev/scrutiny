package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"gorm.io/gorm"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// ZFS Pool
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RegisterZFSPool inserts or updates a ZFS pool in the database
func (sr *scrutinyRepository) RegisterZFSPool(ctx context.Context, pool models.ZFSPool) error {
	return sr.registerZFSPoolWithDB(ctx, sr.gormClient, &pool, time.Now())
}

// RegisterZFSPoolInventory atomically records a complete pool inventory for one host.
func (sr *scrutinyRepository) RegisterZFSPoolInventory(ctx context.Context, hostID string, pools []models.ZFSPool) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return errors.New("ZFS pool inventory requires a host ID")
	}

	now := time.Now()
	return sr.gormClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Advance inventory time for every previously known pool on this host. Pools
		// absent from this report then resolve to missing instead of stale.
		if err := tx.Model(&models.ZFSPool{}).Where("host_id = ?", hostID).
			Update("last_inventory_at", now).Error; err != nil {
			return err
		}

		for i := range pools {
			pool := pools[i]
			if pool.GUID == "" {
				continue
			}
			if pool.HostID != "" && strings.TrimSpace(pool.HostID) != hostID {
				return fmt.Errorf("pool %s belongs to host %q, inventory host is %q", pool.GUID, pool.HostID, hostID)
			}
			pool.HostID = hostID
			pool.LastSeenAt = now
			pool.LastInventoryAt = now
			if err := sr.registerZFSPoolWithDB(ctx, tx, &pool, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (sr *scrutinyRepository) registerZFSPoolWithDB(ctx context.Context, db *gorm.DB, pool *models.ZFSPool, now time.Time) error {
	pool.UpdatedAt = now

	// Check if pool already exists
	var existing models.ZFSPool
	result := db.WithContext(ctx).Where(queryGUID, pool.GUID).First(&existing)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// New pool - create it
		if err := db.WithContext(ctx).Create(pool).Error; err != nil {
			return err
		}
	} else if result.Error != nil {
		return result.Error
	} else {
		// Existing pool - update it
		updates := map[string]interface{}{
			"name":                   pool.Name,
			"status":                 pool.Status,
			"health":                 pool.Health,
			"size":                   pool.Size,
			"allocated":              pool.Allocated,
			"free":                   pool.Free,
			"fragmentation":          pool.Fragmentation,
			"capacity_percent":       pool.CapacityPercent,
			"usable_used":            pool.UsableUsed,
			"usable_free":            pool.UsableFree,
			"ashift":                 pool.Ashift,
			"scrub_state":            pool.ScrubState,
			"scrub_start_time":       pool.ScrubStartTime,
			"scrub_end_time":         pool.ScrubEndTime,
			"scrub_scanned_bytes":    pool.ScrubScannedBytes,
			"scrub_issued_bytes":     pool.ScrubIssuedBytes,
			"scrub_total_bytes":      pool.ScrubTotalBytes,
			"scrub_errors_count":     pool.ScrubErrorsCount,
			"scrub_percent_complete": pool.ScrubPercentComplete,
			"total_read_errors":      pool.TotalReadErrors,
			"total_write_errors":     pool.TotalWriteErrors,
			"total_checksum_errors":  pool.TotalChecksumErrors,
			"updated_at":             pool.UpdatedAt,
		}
		if strings.TrimSpace(pool.HostID) != "" {
			updates["host_id"] = strings.TrimSpace(pool.HostID)
		}
		if !pool.LastSeenAt.IsZero() {
			updates["last_seen_at"] = pool.LastSeenAt
		}
		if !pool.LastInventoryAt.IsZero() {
			updates["last_inventory_at"] = pool.LastInventoryAt
		}
		if err := db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
	}

	// Handle vdevs - delete existing and recreate
	if len(pool.Vdevs) > 0 {
		// Delete existing vdevs for this pool
		if err := db.WithContext(ctx).Where("pool_guid = ?", pool.GUID).Delete(&models.ZFSVdev{}).Error; err != nil {
			return err
		}

		// Insert new vdevs with hierarchy
		if err := sr.insertVdevsRecursive(ctx, db, pool.GUID, pool.Vdevs, nil); err != nil {
			return err
		}
	}

	return nil
}

// insertVdevsRecursive inserts vdevs and their children recursively
func (sr *scrutinyRepository) insertVdevsRecursive(ctx context.Context, db *gorm.DB, poolGUID string, vdevs []models.ZFSVdev, parentID *uint) error {
	for _, vdev := range vdevs {
		vdev.PoolGUID = poolGUID
		vdev.ParentID = parentID
		vdev.ID = 0 // Reset ID to let GORM auto-generate

		children := vdev.Children
		vdev.Children = nil // Don't try to insert children via association

		if err := db.WithContext(ctx).Create(&vdev).Error; err != nil {
			return err
		}

		// Recursively insert children
		if len(children) > 0 {
			if err := sr.insertVdevsRecursive(ctx, db, poolGUID, children, &vdev.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetZFSPools returns all non-archived ZFS pools
func (sr *scrutinyRepository) GetZFSPools(ctx context.Context) ([]models.ZFSPool, error) {
	return sr.getZFSPools(ctx, false)
}

func (sr *scrutinyRepository) getZFSPools(ctx context.Context, includeArchived bool) ([]models.ZFSPool, error) {
	pools := []models.ZFSPool{}
	query := sr.gormClient.WithContext(ctx)
	if !includeArchived {
		query = query.Where("archived = ?", false)
	}
	if err := query.Find(&pools).Error; err != nil {
		return nil, fmt.Errorf("could not get ZFS pools from DB: %v", err)
	}
	sr.resolveZFSPoolPresence(pools)
	return pools, nil
}

func (sr *scrutinyRepository) resolveZFSPoolPresence(pools []models.ZFSPool) {
	staleAfter := time.Hour
	if sr.appConfig != nil {
		minutes := sr.appConfig.GetInt(config.WebZFSPoolStaleAfterMinutesKey)
		if minutes > 0 {
			staleAfter = time.Duration(minutes) * time.Minute
		}
	}
	now := time.Now()
	for i := range pools {
		pools[i].Presence = pools[i].ResolvePresence(now, staleAfter)
	}
}

// GetZFSPoolDetails returns a single ZFS pool with its vdev hierarchy
func (sr *scrutinyRepository) GetZFSPoolDetails(ctx context.Context, guid string) (models.ZFSPool, error) {
	var pool models.ZFSPool

	if err := sr.gormClient.WithContext(ctx).Where(queryGUID, guid).First(&pool).Error; err != nil {
		return models.ZFSPool{}, err
	}

	// Load top-level vdevs (those without a parent)
	var vdevs []models.ZFSVdev
	if err := sr.gormClient.WithContext(ctx).Where("pool_guid = ? AND parent_id IS NULL", guid).Find(&vdevs).Error; err != nil {
		return pool, err
	}

	// Load children recursively for each vdev
	for i := range vdevs {
		if err := sr.loadVdevChildren(ctx, &vdevs[i]); err != nil {
			return pool, err
		}
	}

	pool.Vdevs = vdevs
	resolved := []models.ZFSPool{pool}
	sr.resolveZFSPoolPresence(resolved)
	pool.Presence = resolved[0].Presence
	return pool, nil
}

// loadVdevChildren recursively loads children for a vdev
func (sr *scrutinyRepository) loadVdevChildren(ctx context.Context, vdev *models.ZFSVdev) error {
	var children []models.ZFSVdev
	if err := sr.gormClient.WithContext(ctx).Where("parent_id = ?", vdev.ID).Find(&children).Error; err != nil {
		return err
	}

	for i := range children {
		if err := sr.loadVdevChildren(ctx, &children[i]); err != nil {
			return err
		}
	}

	vdev.Children = children
	return nil
}

// UpdateZFSPoolArchived updates the archived state of a ZFS pool
func (sr *scrutinyRepository) UpdateZFSPoolArchived(ctx context.Context, guid string, archived bool) error {
	var pool models.ZFSPool
	if err := sr.gormClient.WithContext(ctx).Where(queryGUID, guid).First(&pool).Error; err != nil {
		return fmt.Errorf(errZFSPoolNotFound, err)
	}

	return sr.gormClient.Model(&pool).Where(queryGUID, guid).Update("archived", archived).Error
}

// UpdateZFSPoolMuted updates the muted state of a ZFS pool
func (sr *scrutinyRepository) UpdateZFSPoolMuted(ctx context.Context, guid string, muted bool) error {
	var pool models.ZFSPool
	if err := sr.gormClient.WithContext(ctx).Where(queryGUID, guid).First(&pool).Error; err != nil {
		return fmt.Errorf(errZFSPoolNotFound, err)
	}

	return sr.gormClient.Model(&pool).Where(queryGUID, guid).Update("muted", muted).Error
}

// UpdateZFSPoolLabel updates the label of a ZFS pool
func (sr *scrutinyRepository) UpdateZFSPoolLabel(ctx context.Context, guid string, label string) error {
	var pool models.ZFSPool
	if err := sr.gormClient.WithContext(ctx).Where(queryGUID, guid).First(&pool).Error; err != nil {
		return fmt.Errorf(errZFSPoolNotFound, err)
	}

	return sr.gormClient.Model(&pool).Where(queryGUID, guid).Update("label", label).Error
}

// DeleteZFSPool deletes a ZFS pool and its associated data
func (sr *scrutinyRepository) DeleteZFSPool(ctx context.Context, guid string) error {
	// Validate GUID format before using in delete predicate (defense-in-depth, DeleteAPI doesn't support params)
	if err := validation.ValidateGUID(guid); err != nil {
		return fmt.Errorf("invalid GUID: %w", err)
	}

	// Delete vdevs first (foreign key constraint)
	if err := sr.gormClient.WithContext(ctx).Where("pool_guid = ?", guid).Delete(&models.ZFSVdev{}).Error; err != nil {
		return err
	}

	// Delete the pool
	if err := sr.gormClient.WithContext(ctx).Where(queryGUID, guid).Delete(&models.ZFSPool{}).Error; err != nil {
		return err
	}

	// Delete data from InfluxDB
	buckets := []string{
		sr.appConfig.GetString(cfgInfluxDBBucket),
		fmt.Sprintf("%s_weekly", sr.appConfig.GetString(cfgInfluxDBBucket)),
		fmt.Sprintf("%s_monthly", sr.appConfig.GetString(cfgInfluxDBBucket)),
		fmt.Sprintf("%s_yearly", sr.appConfig.GetString(cfgInfluxDBBucket)),
	}

	for _, bucket := range buckets {
		sr.logger.Infof("Deleting ZFS pool data for %s in bucket: %s", guid, bucket)
		if err := sr.influxClient.DeleteAPI().DeleteWithName(
			ctx,
			sr.appConfig.GetString(cfgInfluxDBOrg),
			bucket,
			time.Now().AddDate(-10, 0, 0),
			time.Now(),
			fmt.Sprintf(`pool_guid="%s"`, guid),
		); err != nil {
			return err
		}
	}

	return nil
}

// GetZFSPoolsSummary returns a summary of all non-archived ZFS pools
func (sr *scrutinyRepository) GetZFSPoolsSummary(ctx context.Context) (map[string]*models.ZFSPool, error) {
	pools, err := sr.GetZFSPools(ctx)
	if err != nil {
		return nil, err
	}
	return zfsPoolsSummary(pools), nil
}

// GetAllZFSPoolsSummary returns active and archived pools for user-facing views that can filter them.
func (sr *scrutinyRepository) GetAllZFSPoolsSummary(ctx context.Context) (map[string]*models.ZFSPool, error) {
	pools, err := sr.getZFSPools(ctx, true)
	if err != nil {
		return nil, err
	}
	return zfsPoolsSummary(pools), nil
}

func zfsPoolsSummary(pools []models.ZFSPool) map[string]*models.ZFSPool {
	summary := make(map[string]*models.ZFSPool)
	for i := range pools {
		summary[pools[i].GUID] = &pools[i]
	}

	return summary
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// ZFS Pool Metrics (InfluxDB)
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SaveZFSPoolMetrics saves ZFS pool metrics to InfluxDB
func (sr *scrutinyRepository) SaveZFSPoolMetrics(ctx context.Context, pool models.ZFSPool) error {
	// Create metrics from pool data
	metrics := measurements.ZFSPoolMetrics{
		Date:            time.Now(),
		PoolGUID:        pool.GUID,
		PoolName:        pool.Name,
		Size:            pool.Size,
		Allocated:       pool.Allocated,
		Free:            pool.Free,
		CapacityPercent: pool.CapacityPercent,
		Fragmentation:   pool.Fragmentation,
		Status:          string(pool.Status),
		ReadErrors:      pool.TotalReadErrors,
		WriteErrors:     pool.TotalWriteErrors,
		ChecksumErrors:  pool.TotalChecksumErrors,
		ScrubState:      string(pool.ScrubState),
		ScrubPercent:    pool.ScrubPercentComplete,
		ScrubErrors:     pool.ScrubErrorsCount,
	}

	tags, fields := metrics.Flatten()

	// Save to daily bucket
	return sr.saveDatapoint(
		sr.influxWriteApi,
		"zfs_pool",
		tags,
		fields,
		metrics.Date,
		ctx,
	)
}

// GetZFSPoolMetricsHistory retrieves historical metrics for a ZFS pool
// Note: GUID is validated at the handler level before reaching this function.
func (sr *scrutinyRepository) GetZFSPoolMetricsHistory(ctx context.Context, guid string, durationKey string) ([]measurements.ZFSPoolMetrics, error) {
	// Map duration key to actual duration and bucket
	bucketName := sr.lookupBucketName(durationKey)
	duration := sr.lookupDuration(durationKey)

	// Use parameterized query to prevent Flux injection
	queryStr := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["_measurement"] == "zfs_pool")
		|> filter(fn: (r) => r["pool_guid"] == params.guid)
		|> aggregateWindow(every: 1h, fn: last, createEmpty: false)
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> sort(columns: ["_time"], desc: false)
	`, bucketName, duration[0], duration[1])

	params := map[string]interface{}{
		"guid": guid,
	}

	result, err := sr.influxQueryApi.QueryWithParams(ctx, queryStr, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query ZFS pool metrics: %v", err)
	}
	defer result.Close()

	var metricsHistory []measurements.ZFSPoolMetrics
	for result.Next() {
		record := result.Record()
		values := record.Values()

		metrics, err := measurements.NewZFSPoolMetricsFromInfluxDB(values)
		if err != nil {
			sr.logger.Warnf("Failed to parse ZFS pool metrics: %v", err)
			continue
		}

		metricsHistory = append(metricsHistory, *metrics)
	}

	if result.Err() != nil {
		return nil, fmt.Errorf("query error: %v", result.Err())
	}

	return metricsHistory, nil
}
