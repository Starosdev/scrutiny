package database

import (
	"context"
	"time"
)

// TryAcquireLease takes or renews the named lease for holder until now+ttl. It returns true when
// holder owns the lease after the call: the lease was free, expired, or already held by holder.
//
// A single upsert statement does the check and the write, so two replicas cannot both win, and it
// needs no session-level lock, which keeps it safe behind a transaction-pooling connection proxy.
func (sr *scrutinyRepository) TryAcquireLease(ctx context.Context, name string, holder string, ttl time.Duration) (bool, error) {
	now := time.Now()
	result := sr.gormClient.WithContext(ctx).Exec(
		`INSERT INTO scheduler_leases (name, holder, expires_at_unix_ms) VALUES (?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET holder = excluded.holder, expires_at_unix_ms = excluded.expires_at_unix_ms
		WHERE scheduler_leases.expires_at_unix_ms < ? OR scheduler_leases.holder = excluded.holder`,
		name, holder, now.Add(ttl).UnixMilli(), now.UnixMilli(),
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ReleaseLease gives up the named lease if holder still owns it, so another replica can take over
// without waiting for the lease to expire.
func (sr *scrutinyRepository) ReleaseLease(ctx context.Context, name string, holder string) error {
	return sr.gormClient.WithContext(ctx).Exec(
		"DELETE FROM scheduler_leases WHERE name = ? AND holder = ?", name, holder,
	).Error
}
