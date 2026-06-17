package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/glebarez/sqlite"
	"github.com/golang/mock/gomock"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type stubWriteAPI struct{}

func (s *stubWriteAPI) WriteRecord(ctx context.Context, line ...string) error { return nil }
func (s *stubWriteAPI) WritePoint(ctx context.Context, point ...*write.Point) error {
	return nil
}
func (s *stubWriteAPI) EnableBatching()                 {}
func (s *stubWriteAPI) Flush(ctx context.Context) error { return nil }

var _ influxapi.WriteAPIBlocking = (*stubWriteAPI)(nil)

type stubQueryAPI struct{}

func (s *stubQueryAPI) QueryRaw(ctx context.Context, query string, dialect *domain.Dialect) (string, error) {
	return "", errors.New("not implemented in tests")
}

func (s *stubQueryAPI) QueryRawWithParams(ctx context.Context, query string, dialect *domain.Dialect, params interface{}) (string, error) {
	return "", errors.New("not implemented in tests")
}

func (s *stubQueryAPI) Query(ctx context.Context, query string) (*influxapi.QueryTableResult, error) {
	return nil, errors.New("not implemented in tests")
}

func (s *stubQueryAPI) QueryWithParams(ctx context.Context, query string, params interface{}) (*influxapi.QueryTableResult, error) {
	return nil, errors.New("not implemented in tests")
}

var _ influxapi.QueryAPI = (*stubQueryAPI)(nil)

func createDeviceSelfTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().Get(gomock.Any()).Return(nil).AnyTimes()
	fakeConfig.EXPECT().GetBool(gomock.Any()).Return(false).AnyTimes()
	fakeConfig.EXPECT().GetInt(gomock.Any()).Return(0).AnyTimes()
	fakeConfig.EXPECT().GetIntSlice(gomock.Any()).Return(nil).AnyTimes()
	fakeConfig.EXPECT().GetString(gomock.Any()).Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetStringSlice(gomock.Any()).Return(nil).AnyTimes()
	fakeConfig.EXPECT().IsSet(gomock.Any()).Return(false).AnyTimes()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Device{}, &models.DeviceSelfTest{}, &models.AttributeOverride{}))

	return &scrutinyRepository{
		appConfig:      fakeConfig,
		gormClient:     db,
		logger:         logrus.New(),
		influxWriteApi: &stubWriteAPI{},
		influxQueryApi: &stubQueryAPI{},
	}
}

func loadSmartInfoFixture(t *testing.T, fixturePath string) collector.SmartInfo {
	t.Helper()

	payload, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	var smartInfo collector.SmartInfo
	require.NoError(t, json.Unmarshal(payload, &smartInfo))
	return smartInfo
}

func TestSaveSmartAttributesPersistsAtaSelfTests(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()

	device := models.Device{
		DeviceID:       "device-1",
		WWN:            "wwn-1",
		DeviceProtocol: "ATA",
	}
	require.NoError(t, repo.gormClient.WithContext(ctx).Create(&device).Error)

	smartInfo := loadSmartInfoFixture(t, filepath.Join("..", "web", "testdata", "upload-device-metrics-req.json"))

	_, err := repo.SaveSmartAttributes(ctx, device.WWN, smartInfo)
	require.NoError(t, err)

	var selfTests []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.WithContext(ctx).
		Order("lifetime_hours DESC, id DESC").
		Find(&selfTests).Error)

	require.Len(t, selfTests, 21)
	require.Equal(t, device.DeviceID, selfTests[0].DeviceID)
	require.Equal(t, device.WWN, selfTests[0].DeviceWWN)
	require.Equal(t, 1708, selfTests[0].LifetimeHours)
	require.Equal(t, "Short offline", selfTests[0].TypeString)
	require.Equal(t, "Completed without error", selfTests[0].StatusString)
	require.True(t, selfTests[0].StatusPassed)
	require.Equal(t, 1157, selfTests[len(selfTests)-1].LifetimeHours)
}

func TestSyncDeviceSelfTestsDedupesByDeviceIdentity(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()

	initialDevice := models.Device{
		DeviceID:       "device-1",
		WWN:            "wwn-1",
		DeviceProtocol: "ATA",
	}
	replacementDevice := models.Device{
		DeviceID:       "device-2",
		WWN:            "wwn-1",
		DeviceProtocol: "ATA",
	}

	initialPayload := collector.SmartInfo{}
	initialPayload.Device.Protocol = "ATA"
	initialPayload.AtaSmartSelfTestLog.Standard.Table = []collector.AtaSmartSelfTestLogEntry{
		{
			LifetimeHours: 100,
		},
	}
	initialPayload.AtaSmartSelfTestLog.Standard.Table[0].Type.Value = 1
	initialPayload.AtaSmartSelfTestLog.Standard.Table[0].Type.String = "Short offline"
	initialPayload.AtaSmartSelfTestLog.Standard.Table[0].Status.Value = 1
	initialPayload.AtaSmartSelfTestLog.Standard.Table[0].Status.String = "Aborted by host"
	initialPayload.AtaSmartSelfTestLog.Standard.Table[0].Status.Passed = false

	updatedPayload := collector.SmartInfo{}
	updatedPayload.Device.Protocol = "ATA"
	updatedPayload.AtaSmartSelfTestLog.Standard.Table = []collector.AtaSmartSelfTestLogEntry{
		{
			LifetimeHours: 100,
		},
	}
	updatedPayload.AtaSmartSelfTestLog.Standard.Table[0].Type.Value = 1
	updatedPayload.AtaSmartSelfTestLog.Standard.Table[0].Type.String = "Short offline"
	updatedPayload.AtaSmartSelfTestLog.Standard.Table[0].Status.Value = 0
	updatedPayload.AtaSmartSelfTestLog.Standard.Table[0].Status.String = "Completed without error"
	updatedPayload.AtaSmartSelfTestLog.Standard.Table[0].Status.Passed = true

	require.NoError(t, repo.syncDeviceSelfTests(ctx, initialDevice, initialPayload))
	require.NoError(t, repo.syncDeviceSelfTests(ctx, replacementDevice, updatedPayload))

	var selfTests []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.WithContext(ctx).Find(&selfTests).Error)
	require.Len(t, selfTests, 1)
	require.Equal(t, "device-2", selfTests[0].DeviceID)
	require.Equal(t, "wwn-1", selfTests[0].DeviceWWN)
	require.Equal(t, "Completed without error", selfTests[0].StatusString)
	require.True(t, selfTests[0].StatusPassed)
}

func TestSyncDeviceSelfTestsPrunesOldEntries(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()

	device := models.Device{
		DeviceID:       "device-1",
		WWN:            "wwn-1",
		DeviceProtocol: "ATA",
	}

	payload := collector.SmartInfo{}
	payload.Device.Protocol = "ATA"
	for lifetime := 1; lifetime <= 25; lifetime++ {
		entry := collector.AtaSmartSelfTestLogEntry{
			LifetimeHours: lifetime,
		}
		entry.Type.Value = 1
		entry.Type.String = "Short offline"
		entry.Status.Value = 0
		entry.Status.String = "Completed without error"
		entry.Status.Passed = true
		payload.AtaSmartSelfTestLog.Standard.Table = append(payload.AtaSmartSelfTestLog.Standard.Table, entry)
	}

	require.NoError(t, repo.syncDeviceSelfTests(ctx, device, payload))

	var selfTests []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.WithContext(ctx).
		Order("lifetime_hours DESC").
		Find(&selfTests).Error)

	require.Len(t, selfTests, 21)
	require.Equal(t, 25, selfTests[0].LifetimeHours)
	require.Equal(t, 5, selfTests[len(selfTests)-1].LifetimeHours)
}
