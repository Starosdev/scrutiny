package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCompareAndSetSettingValue(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SettingEntry{}))
	repo := &scrutinyRepository{gormClient: db}
	t.Cleanup(func() { _ = repo.Close() })
	require.NoError(t, db.Create(&models.SettingEntry{SettingKeyName: "k", SettingDataType: "string"}).Error)

	ok, err := repo.CompareAndSetSettingValue(ctx, "k", "", "t1")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = repo.CompareAndSetSettingValue(ctx, "k", "", "t2")
	require.NoError(t, err)
	require.False(t, ok, "a stale expected value loses")

	ok, err = repo.CompareAndSetSettingValue(ctx, "k", "t1", "t2")
	require.NoError(t, err)
	require.True(t, ok)
	value, err := repo.GetSettingValue(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, "t2", value)

	ok, err = repo.CompareAndSetSettingValue(ctx, "missing", "", "t1")
	require.NoError(t, err)
	require.True(t, ok, "a missing setting counts as empty and is created")
	ok, err = repo.CompareAndSetSettingValue(ctx, "missing", "", "t2")
	require.NoError(t, err)
	require.False(t, ok)
}
