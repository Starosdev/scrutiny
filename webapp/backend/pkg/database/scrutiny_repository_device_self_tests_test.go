package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
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
	require.NoError(t, db.AutoMigrate(&models.Device{}, &models.DeviceSelfTest{}, &models.AttributeOverride{}, &models.DeviceEnduranceOverride{}))

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

func TestSaveSmartAttributesPersistsScsiSelfTests(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()

	device := models.Device{
		DeviceID:       "device-1",
		WWN:            "wwn-1",
		DeviceProtocol: "SCSI",
	}
	require.NoError(t, repo.gormClient.WithContext(ctx).Create(&device).Error)

	smartInfo := loadSmartInfoFixture(t, filepath.Join("..", "models", "testdata", "smart-scsi-selftest.json"))
	require.Len(t, smartInfo.ScsiSelfTests, 3)

	_, err := repo.SaveSmartAttributes(ctx, device.WWN, smartInfo)
	require.NoError(t, err)

	var selfTests []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.WithContext(ctx).
		Order("lifetime_hours DESC, id DESC").
		Find(&selfTests).Error)

	require.Len(t, selfTests, 3)
	require.Equal(t, device.DeviceID, selfTests[0].DeviceID)
	require.Equal(t, device.WWN, selfTests[0].DeviceWWN)
	require.Equal(t, 48239, selfTests[0].LifetimeHours)
	require.Equal(t, "Background short", selfTests[0].TypeString)
	require.Equal(t, "Completed", selfTests[0].StatusString)
	require.True(t, selfTests[0].StatusPassed)
	require.Equal(t, 48234, selfTests[len(selfTests)-1].LifetimeHours)
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

	require.NoError(t, repo.syncDeviceSelfTests(ctx, &initialDevice, &initialPayload, 100))
	require.NoError(t, repo.syncDeviceSelfTests(ctx, &replacementDevice, &updatedPayload, 100))

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
	for lifetime := 25; lifetime >= 1; lifetime-- {
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

	require.NoError(t, repo.syncDeviceSelfTests(ctx, &device, &payload, 25))

	var selfTests []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.WithContext(ctx).
		Order("lifetime_hours DESC").
		Find(&selfTests).Error)

	require.Len(t, selfTests, 21)
	require.Equal(t, 25, selfTests[0].LifetimeHours)
	require.Equal(t, 5, selfTests[len(selfTests)-1].LifetimeHours)
}

func selfTestPayload(hours, observed int64, lifetimes ...int) collector.SmartInfo {
	payload := collector.SmartInfo{}
	payload.Device.Protocol = "ATA"
	payload.PowerOnTime.Hours = hours
	payload.LocalTime.TimeT = observed
	for _, lifetime := range lifetimes {
		entry := collector.AtaSmartSelfTestLogEntry{LifetimeHours: lifetime}
		entry.Type.Value = 1
		entry.Type.String = "Short offline"
		entry.Status.Passed = true
		payload.AtaSmartSelfTestLog.Standard.Table = append(payload.AtaSmartSelfTestLog.Standard.Table, entry)
	}
	return payload
}

func TestSelfTestLifetimeBounds(t *testing.T) {
	tests := []struct {
		name     string
		hours    int64
		raw      []int
		expected []int64 // -1 means ambiguous
	}{
		{"reported rollover", 68000, []int{2464, 39224}, []int64{68000, 39224}},
		{"before rollover", 65000, []int{64000, 100}, []int64{64000, 100}},
		{"rollover boundary", 65536, []int{0, 65535}, []int64{65536, 65535}},
		{"multiple possible epochs", 68000, []int{100}, []int64{-1}},
		{"multiple witnessed wraps", 131100, []int{28, 65535, 0, 65535}, []int64{131100, 131071, 65536, 65535}},
		{"missing hours", 0, []int{2464, 39224}, []int64{-1, -1}},
		{"negative hours", -1, []int{100}, []int64{-1}},
		{"inconsistent current counter", 100, []int{200, 100}, []int64{-1, -1}},
		{"invalid raw value", 68000, []int{-1, 100}, []int64{-1, -1}},
		{"outside ATA field", 68000, []int{65536}, []int64{-1}},
		{"same hour", 100, []int{100, 100}, []int64{100, 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := selfTestPayload(tt.hours, 1000, tt.raw...)
			minimum, maximum := selfTestLifetimeBounds(payload.AtaSmartSelfTestLog.Entries(), tt.hours)
			for i, want := range tt.expected {
				if want < 0 {
					require.True(t, maximum[i] < 0 || minimum[i] != maximum[i])
					continue
				}
				require.Equal(t, want, minimum[i])
				require.Equal(t, want, maximum[i])
			}
		})
	}
}

func TestSelfTestChronologyAcrossCollections(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()
	device := models.Device{DeviceID: "device-1", WWN: "wwn-1", DeviceProtocol: "ATA"}
	require.NoError(t, repo.gormClient.Create(&device).Error)
	save := func(payload collector.SmartInfo) []models.DeviceSelfTest {
		t.Helper()
		require.NoError(t, repo.syncDeviceSelfTests(ctx, &device, &payload, payload.PowerOnTime.Hours))
		rows, err := repo.GetDeviceSelfTests(ctx, device.DeviceID)
		require.NoError(t, err)
		return rows
	}
	old := save(selfTestPayload(40000, 1000, 39224, 2464))
	require.Len(t, old, 2)
	oldID := old[1].ID
	payload := selfTestPayload(68000, 2000, 2464, 39224)
	rows := save(payload)
	require.Len(t, rows, 3)
	require.Equal(t, int64(68000), *rows[0].EffectiveLifetimeHours)
	require.Equal(t, 2464, rows[0].LifetimeHours)
	require.NotEqual(t, oldID, rows[0].ID)
	require.Equal(t, int64(39224), *rows[1].EffectiveLifetimeHours)
	require.Equal(t, oldID, rows[2].ID)
	require.Equal(t, int64(2464), *rows[2].EffectiveLifetimeHours)
	ids := []uint{rows[0].ID, rows[1].ID, rows[2].ID}
	payload.LocalTime.TimeT = 3000
	payload.AtaSmartSelfTestLog.Standard.Table[0].Status.String = "Updated status"
	rows = save(payload)
	require.Len(t, rows, 3)
	require.Equal(t, ids, []uint{rows[0].ID, rows[1].ID, rows[2].ID})
	require.Equal(t, "Updated status", rows[0].StatusString)
	rows = save(selfTestPayload(40000, 1500, 39224, 2464))
	require.Equal(t, ids, []uint{rows[0].ID, rows[1].ID, rows[2].ID})
	require.Equal(t, int64(3000), rows[0].ObservedAt)
}

func TestSelfTestRetentionUsesControllerOrder(t *testing.T) {
	for _, hours := range []int64{68000, 0} {
		t.Run(fmt.Sprint(hours), func(t *testing.T) {
			repo := createDeviceSelfTestRepository(t)
			ctx := context.Background()
			device := models.Device{DeviceID: "device-1"}
			payload := selfTestPayload(hours, 1000, 2464)
			for raw := 65000; raw > 64975; raw-- {
				entry := collector.AtaSmartSelfTestLogEntry{LifetimeHours: raw}
				payload.AtaSmartSelfTestLog.Standard.Table = append(payload.AtaSmartSelfTestLog.Standard.Table, entry)
			}
			require.NoError(t, repo.syncDeviceSelfTests(ctx, &device, &payload, hours))
			var rows []models.DeviceSelfTest
			require.NoError(t, repo.gormClient.Order(selfTestHistoryOrder).Find(&rows).Error)
			require.Len(t, rows, 21)
			require.Equal(t, 2464, rows[0].LifetimeHours)
			require.Equal(t, 64981, rows[20].LifetimeHours)
			if hours == 0 {
				require.Nil(t, rows[0].EffectiveLifetimeHours)
			}
		})
	}
}

func TestSelfTestAmbiguousOccurrenceDoesNotOverwriteEarlierEpoch(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()
	device := models.Device{DeviceID: "device-1"}
	before := selfTestPayload(65000, 1000, 100)
	require.NoError(t, repo.syncDeviceSelfTests(ctx, &device, &before, 65000))
	after := selfTestPayload(65636, 2000, 100)
	for i := 0; i < 2; i++ {
		require.NoError(t, repo.syncDeviceSelfTests(ctx, &device, &after, 65636))
	}
	var rows []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.Order(selfTestHistoryOrder).Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Nil(t, rows[0].EffectiveLifetimeHours)
	require.Equal(t, int64(100), *rows[1].EffectiveLifetimeHours)
}

func TestSelfTestSameHourOccurrencesAndExtendedFallback(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()
	device := models.Device{DeviceID: "device-1"}
	payload := selfTestPayload(100, 1000, 100, 100)
	payload.AtaSmartSelfTestLog.Extended.Table = payload.AtaSmartSelfTestLog.Standard.Table
	payload.AtaSmartSelfTestLog.Standard.Table = nil
	for i := 0; i < 2; i++ {
		require.NoError(t, repo.syncDeviceSelfTests(ctx, &device, &payload, 100))
	}
	var rows []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.Find(&rows).Error)
	require.Len(t, rows, 2)
	payload.Device.Protocol = "NVMe"
	payload.AtaSmartSelfTestLog.Extended.Table[0].LifetimeHours = 99
	require.NoError(t, repo.syncDeviceSelfTests(ctx, &device, &payload, 100))
	require.NoError(t, repo.gormClient.Find(&rows).Error)
	require.Len(t, rows, 2)
}

func TestGetLatestDeviceSelfTestUsesObservedOrder(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()
	device := models.Device{DeviceID: "device-1", WWN: "wwn-1", DeviceProtocol: "ATA"}
	require.NoError(t, repo.gormClient.Create(&device).Error)
	require.NoError(t, repo.gormClient.Create(&models.DeviceSelfTest{
		DeviceID: device.DeviceID, DeviceWWN: device.WWN, DeviceIdentity: device.WWN,
		StatusPassed: true, ObservedAt: 100, LogIndex: 0,
	}).Error)
	require.NoError(t, repo.gormClient.Create(&models.DeviceSelfTest{
		DeviceID: device.DeviceID, DeviceWWN: device.WWN, DeviceIdentity: device.WWN,
		StatusPassed: false, ObservedAt: 200, LogIndex: 0,
	}).Error)

	latest, err := repo.GetLatestDeviceSelfTest(ctx, device.DeviceID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.False(t, latest.StatusPassed)
	require.Equal(t, int64(200), latest.ObservedAt)
}

func TestGetLatestDeviceSelfTestReturnsNilWithoutHistory(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()
	device := models.Device{DeviceID: "device-1", WWN: "wwn-1", DeviceProtocol: "ATA"}
	require.NoError(t, repo.gormClient.Create(&device).Error)

	latest, err := repo.GetLatestDeviceSelfTest(ctx, device.DeviceID)
	require.NoError(t, err)
	require.Nil(t, latest)
}

func TestSelfTestRejectsUnreliablePowerOnContext(t *testing.T) {
	current := measurements.Smart{PowerOnHours: 68000}
	require.Equal(t, int64(68000), selfTestPowerOnHours(&current, nil))
	previous := measurements.Smart{PowerOnHours: 69000}
	require.Zero(t, selfTestPowerOnHours(&current, &previous))
	current.Attributes = map[string]measurements.SmartAttribute{"9": &measurements.SmartAtaAttribute{Status: pkg.AttributeStatusWarningScrutiny}}
	require.Zero(t, selfTestPowerOnHours(&current, nil))
}

func TestSaveSmartAttributesResolvesSelfTestRollover(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	ctx := context.Background()
	device := models.Device{DeviceID: "device-1", WWN: "wwn-1", DeviceProtocol: "ATA"}
	require.NoError(t, repo.gormClient.Create(&device).Error)
	payload := loadSmartInfoFixture(t, filepath.Join("..", "web", "testdata", "upload-device-metrics-req.json"))
	payload.PowerOnTime.Hours = 68000
	payload.AtaSmartSelfTestLog = selfTestPayload(68000, 1000, 2464, 39224).AtaSmartSelfTestLog
	for i := range payload.AtaSmartAttributes.Table {
		attribute := &payload.AtaSmartAttributes.Table[i]
		if attribute.ID == 9 {
			attribute.Raw.Value = 68000
			attribute.Raw.String = "68000"
		}
	}
	_, err := repo.SaveSmartAttributes(ctx, device.WWN, payload)
	require.NoError(t, err)
	rows, err := repo.GetDeviceSelfTests(ctx, device.DeviceID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].EffectiveLifetimeHours)
	require.Equal(t, int64(68000), *rows[0].EffectiveLifetimeHours)
	require.Equal(t, int64(39224), *rows[1].EffectiveLifetimeHours)
}
