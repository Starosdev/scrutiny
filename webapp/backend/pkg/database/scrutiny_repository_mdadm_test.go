package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createMdadmTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.MDADMArray{}))

	return &scrutinyRepository{gormClient: db}
}

func TestRegisterMdadmArrayUpdatesExistingArrayDevices(t *testing.T) {
	repo := createMdadmTestRepository(t)
	ctx := context.Background()

	initial := models.MDADMArray{
		UUID:    "ad005595:61305895:4de01d37:669bf162",
		Name:    "md2",
		Level:   "raid5",
		Devices: []string{"/dev/sde", "/dev/sdd"},
	}
	require.NoError(t, repo.RegisterMdadmArray(ctx, initial))

	updated := models.MDADMArray{
		UUID:    initial.UUID,
		Name:    "md2",
		Level:   "raid5",
		Devices: []string{"/dev/sde", "/dev/sdd", "/dev/sdi", "/dev/sdg", "/dev/sdh", "/dev/sdf"},
	}
	require.NoError(t, repo.RegisterMdadmArray(ctx, updated))

	loaded, err := repo.GetMdadmArrayDetails(ctx, initial.UUID)
	require.NoError(t, err)
	require.Equal(t, updated.Devices, loaded.Devices)
}

func TestRegisterMdadmArrayUpdatesExistingArrayHostID(t *testing.T) {
	repo := createMdadmTestRepository(t)
	ctx := context.Background()

	initial := models.MDADMArray{
		UUID:    "ad005595:61305895:4de01d37:669bf162",
		Name:    "md2",
		Level:   "raid5",
		Devices: []string{"/dev/sde", "/dev/sdd"},
	}
	require.NoError(t, repo.RegisterMdadmArray(ctx, initial))

	updated := models.MDADMArray{
		UUID:    initial.UUID,
		Name:    initial.Name,
		Level:   initial.Level,
		Devices: initial.Devices,
		HostID:  "nas-01",
	}
	require.NoError(t, repo.RegisterMdadmArray(ctx, updated))

	loaded, err := repo.GetMdadmArrayDetails(ctx, initial.UUID)
	require.NoError(t, err)
	require.Equal(t, updated.HostID, loaded.HostID)
}

func TestGetMdadmArraysExcludesLegacyBlankUUIDRows(t *testing.T) {
	repo := createMdadmTestRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.gormClient.WithContext(ctx).Create(&models.MDADMArray{
		UUID:  "",
		Name:  "md0",
		Level: "raid1",
	}).Error)
	require.NoError(t, repo.gormClient.WithContext(ctx).Create(&models.MDADMArray{
		UUID:  "ad005595:61305895:4de01d37:669bf162",
		Name:  "md2",
		Level: "raid5",
	}).Error)

	arrays, err := repo.GetMdadmArrays(ctx)
	require.NoError(t, err)
	require.Len(t, arrays, 1)
	require.Equal(t, "md2", arrays[0].Name)
	require.Equal(t, "ad005595:61305895:4de01d37:669bf162", arrays[0].UUID)
}
