package database

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClose_ReleasesSQLConnectionPool(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "close.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())

	repo := &scrutinyRepository{gormClient: db}
	require.NoError(t, repo.Close())

	require.ErrorContains(t, sqlDB.Ping(), "database is closed")
}

func TestClose_ToleratesPartiallyBuiltRepository(t *testing.T) {
	require.NoError(t, (&scrutinyRepository{}).Close())
}
