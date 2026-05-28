package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/common"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createDeviceRegisterTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Device{}))

	return &scrutinyRepository{
		gormClient: db,
		logger:     logrus.New(),
	}
}

func TestRegisterDeviceRefreshesMetadataOnConflict(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()

	deviceID := "device-1"
	initial := models.Device{
		DeviceID:       deviceID,
		WWN:            "wwn-1",
		DeviceName:     "sda",
		ModelName:      "Samsung SSD 870 EVO 4TB",
		SerialNumber:   "",
		Capacity:       0,
		Firmware:       "",
		InterfaceType:  "",
		InterfaceSpeed: "",
		FormFactor:     "",
		RotationSpeed:  0,
	}
	require.NoError(t, repo.RegisterDevice(ctx, initial))

	refreshed := models.Device{
		DeviceID:        deviceID,
		WWN:             "wwn-1",
		DeviceName:      "sda",
		ModelName:       "Samsung SSD 870 EVO 4TB",
		Manufacturer:    "Samsung",
		SerialNumber:    "S7HDNF0Y548663E",
		Capacity:        4000787030016,
		Firmware:        "SVT02B6Q",
		InterfaceType:   "SATA",
		InterfaceSpeed:  "6.0 Gb/s",
		FormFactor:      "2.5 inches",
		RotationSpeed:   0,
		DeviceProtocol:  "ATA",
		SmartSupport:    common.SmartSupport{Available: true},
		CollectorVersion: "1.61.0",
	}
	require.NoError(t, repo.RegisterDevice(ctx, refreshed))

	var stored models.Device
	require.NoError(t, repo.gormClient.WithContext(ctx).Where(queryDeviceID, deviceID).First(&stored).Error)
	require.Equal(t, "S7HDNF0Y548663E", stored.SerialNumber)
	require.Equal(t, int64(4000787030016), stored.Capacity)
	require.Equal(t, "SVT02B6Q", stored.Firmware)
	require.Equal(t, "SATA", stored.InterfaceType)
	require.Equal(t, "6.0 Gb/s", stored.InterfaceSpeed)
	require.Equal(t, "2.5 inches", stored.FormFactor)
	require.Equal(t, "Samsung", stored.Manufacturer)
}

func TestRegisterDeviceRekeysLegacyRowWhenSerialAppearsForStableWWN(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()

	legacyID := deviceid.Generate("WDC WD80EFZZ-68BTXN0", "", "0x50014ee2c06ce3c3")
	refreshedID := deviceid.Generate("WDC WD80EFZZ-68BTXN0", "WD-CA2XZ08L", "0x50014ee2c06ce3c3")

	require.NoError(t, repo.RegisterDevice(ctx, models.Device{
		DeviceID:     legacyID,
		WWN:          "0x50014ee2c06ce3c3",
		HostId:       "host-1",
		DeviceName:   "sdb",
		ModelName:    "WDC WD80EFZZ-68BTXN0",
		SerialNumber: "",
		Capacity:     0,
	}))

	require.NoError(t, repo.RegisterDevice(ctx, models.Device{
		DeviceID:        refreshedID,
		WWN:             "0x50014ee2c06ce3c3",
		HostId:          "host-1",
		DeviceName:      "sdb",
		ModelName:       "WDC WD80EFZZ-68BTXN0",
		Manufacturer:    "Western Digital",
		SerialNumber:    "WD-CA2XZ08L",
		Capacity:        8001563222016,
		DeviceProtocol:  "ATA",
		CollectorVersion: "1.61.0",
	}))

	var devices []models.Device
	require.NoError(t, repo.gormClient.WithContext(ctx).Order("created_at asc").Find(&devices).Error)
	require.Len(t, devices, 1)
	require.Equal(t, refreshedID, devices[0].DeviceID)
	require.Equal(t, "WD-CA2XZ08L", devices[0].SerialNumber)
	require.Equal(t, int64(8001563222016), devices[0].Capacity)
	require.Equal(t, "Western Digital", devices[0].Manufacturer)
}

func TestRegisterDeviceDeletesLegacyDuplicateOnceCanonicalRowExists(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()

	legacyID := deviceid.Generate("WDC WD80EFZZ-68BTXN0", "", "0x50014ee2c06ce3c3")
	canonicalID := deviceid.Generate("WDC WD80EFZZ-68BTXN0", "WD-CA2XZ08L", "0x50014ee2c06ce3c3")

	legacyCreatedAt := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, repo.RegisterDevice(ctx, models.Device{
		DeviceID:     legacyID,
		WWN:          "0x50014ee2c06ce3c3",
		HostId:       "host-1",
		DeviceName:   "sdb",
		ModelName:    "WDC WD80EFZZ-68BTXN0",
		SerialNumber: "",
		Capacity:     0,
	}))
	require.NoError(t, repo.gormClient.WithContext(ctx).
		Model(&models.Device{}).
		Where(queryDeviceID, legacyID).
		Update("created_at", legacyCreatedAt).Error)

	require.NoError(t, repo.RegisterDevice(ctx, models.Device{
		DeviceID:        canonicalID,
		WWN:             "0x50014ee2c06ce3c3",
		HostId:          "host-1",
		DeviceName:      "sdb",
		ModelName:       "WDC WD80EFZZ-68BTXN0",
		Manufacturer:    "Western Digital",
		SerialNumber:    "WD-CA2XZ08L",
		Capacity:        8001563222016,
		DeviceProtocol:  "ATA",
		CollectorVersion: "1.61.0",
	}))

	require.NoError(t, repo.RegisterDevice(ctx, models.Device{
		DeviceID:        canonicalID,
		WWN:             "0x50014ee2c06ce3c3",
		HostId:          "host-1",
		DeviceName:      "sdb",
		ModelName:       "WDC WD80EFZZ-68BTXN0",
		Manufacturer:    "Western Digital",
		SerialNumber:    "WD-CA2XZ08L",
		Capacity:        8001563222016,
		DeviceProtocol:  "ATA",
		CollectorVersion: "1.61.0",
	}))

	var devices []models.Device
	require.NoError(t, repo.gormClient.WithContext(ctx).Order("created_at asc").Find(&devices).Error)
	require.Len(t, devices, 1)
	require.Equal(t, canonicalID, devices[0].DeviceID)
	require.Equal(t, legacyCreatedAt, devices[0].CreatedAt.UTC().Truncate(time.Second))
}

func TestRegisterDeviceRejectsBlankDevice(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()

	err := repo.RegisterDevice(ctx, models.Device{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no identifying information")

	var count int64
	require.NoError(t, repo.gormClient.WithContext(ctx).Model(&models.Device{}).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func TestRegisterDeviceAcceptsDeviceWithOnlyDeviceName(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()

	err := repo.RegisterDevice(ctx, models.Device{
		DeviceName: "sda",
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, repo.gormClient.WithContext(ctx).Model(&models.Device{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestRegisterDeviceAcceptsDeviceWithOnlySerialNumber(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()

	err := repo.RegisterDevice(ctx, models.Device{
		SerialNumber: "SN123456",
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, repo.gormClient.WithContext(ctx).Model(&models.Device{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
