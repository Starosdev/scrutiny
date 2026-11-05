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

func TestMdadmArrayActionsPersistEachFlag(t *testing.T) {
	repo := createMdadmTestRepository(t)
	ctx := context.Background()

	array := models.MDADMArray{
		UUID:    "synthetic:4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		Name:    "md4",
		Level:   "raid1",
		Devices: []string{"/dev/sda", "/dev/sdb"},
	}
	require.NoError(t, repo.RegisterMdadmArray(ctx, array))

	require.NoError(t, repo.UpdateMdadmArrayArchived(ctx, array.UUID, true))
	require.NoError(t, repo.UpdateMdadmArrayMuted(ctx, array.UUID, true))
	require.NoError(t, repo.UpdateMdadmArrayLabel(ctx, array.UUID, "Media array"))

	loaded, err := repo.GetMdadmArrayDetails(ctx, array.UUID)
	require.NoError(t, err)
	require.True(t, loaded.Archived)
	require.True(t, loaded.Muted)
	require.Equal(t, "Media array", loaded.Label)

	// Archiving takes the array out of the summary, which is why the arrays
	// page cannot offer an unarchive.
	summary, err := repo.GetMdadmArraysSummary(ctx)
	require.NoError(t, err)
	require.NotContains(t, summary, array.UUID)

	require.NoError(t, repo.UpdateMdadmArrayArchived(ctx, array.UUID, false))
	summary, err = repo.GetMdadmArraysSummary(ctx)
	require.NoError(t, err)
	require.Contains(t, summary, array.UUID)
}

// The identifier reaches an InfluxDB delete predicate, so the repository
// rejects an unusable one itself rather than relying on its caller.
func TestDeleteMdadmArrayRejectsAnInvalidIdentifier(t *testing.T) {
	repo := createMdadmTestRepository(t)

	require.Error(t, repo.DeleteMdadmArray(context.Background(), `a" or array_uuid!="a`))
	require.Error(t, repo.DeleteMdadmArray(context.Background(), ""))
}
