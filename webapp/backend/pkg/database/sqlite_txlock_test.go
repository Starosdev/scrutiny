package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

func openTxLockTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path+sqliteDSNQuery("WAL")), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// A read-then-write transaction must not fail with SQLITE_BUSY when another connection commits a
// write between its read and its write (seen in production when the filesystem collector posted
// while RegisterDevice ran). With immediate transactions the second writer waits instead.
func TestSQLiteReadThenWriteTransactionSurvivesConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "txlock.db")
	first := openTxLockTestDB(t, path)
	second := openTxLockTestDB(t, path)
	require.NoError(t, first.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)").Error)

	readDone := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		<-readDone
		writerDone <- second.Exec("INSERT INTO items (name) VALUES ('from-second')").Error
	}()

	err := first.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Raw("SELECT count(*) FROM items").Scan(&count).Error; err != nil {
			return err
		}
		close(readDone)
		// Give the other connection time to attempt its write while this transaction is open.
		time.Sleep(200 * time.Millisecond)
		return tx.Exec("INSERT INTO items (name) VALUES ('from-first')").Error
	})
	require.NoError(t, err)
	require.NoError(t, <-writerDone)

	var total int64
	require.NoError(t, first.Raw("SELECT count(*) FROM items").Scan(&total).Error)
	require.Equal(t, int64(2), total)
}

func TestSQLiteDSNQueryUsesImmediateTransactions(t *testing.T) {
	require.Contains(t, sqliteDSNQuery("WAL"), "_txlock=immediate")
	require.Contains(t, sqliteDSNQuery("WAL"), "busy_timeout%3D30000")
}
