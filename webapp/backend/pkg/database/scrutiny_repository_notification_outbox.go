package database

import (
	"context"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

// EnqueueNotification stores a notification for the leader replica to deliver.
func (sr *scrutinyRepository) EnqueueNotification(ctx context.Context, row *models.NotificationOutbox) error {
	return sr.gormClient.WithContext(ctx).Create(row).Error
}

// ListNotifications returns up to limit outbox rows in the given state, oldest first.
func (sr *scrutinyRepository) ListNotifications(ctx context.Context, state string, limit int) ([]models.NotificationOutbox, error) {
	var rows []models.NotificationOutbox
	err := sr.gormClient.WithContext(ctx).
		Where("state = ?", state).
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// TransitionNotification moves one row from one state to another. It returns false when the row
// is no longer in the from state, which is how two replicas avoid delivering the same row.
func (sr *scrutinyRepository) TransitionNotification(ctx context.Context, id uint, from string, to string) (bool, error) {
	result := sr.gormClient.WithContext(ctx).
		Model(&models.NotificationOutbox{}).
		Where("id = ? AND state = ?", id, from).
		Update("state", to)
	return result.RowsAffected == 1, result.Error
}

// DeleteNotifications removes delivered or discarded outbox rows.
func (sr *scrutinyRepository) DeleteNotifications(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return sr.gormClient.WithContext(ctx).Where("id IN ?", ids).Delete(&models.NotificationOutbox{}).Error
}

// DeleteStaleNotifications removes rows in the given states created before the cutoff and
// returns how many it removed.
func (sr *scrutinyRepository) DeleteStaleNotifications(ctx context.Context, states []string, createdBeforeUnixMs int64) (int64, error) {
	result := sr.gormClient.WithContext(ctx).
		Where("state IN ? AND created_at_unix_ms < ?", states, createdBeforeUnixMs).
		Delete(&models.NotificationOutbox{})
	return result.RowsAffected, result.Error
}
