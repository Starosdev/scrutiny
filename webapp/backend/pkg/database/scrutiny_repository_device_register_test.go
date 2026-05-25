package database

import (
	"context"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/common"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createDeviceRegisterTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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
