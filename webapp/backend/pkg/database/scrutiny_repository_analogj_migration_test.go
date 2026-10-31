package database

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAnalogJInfluxMigrationQueryRewritesOnlyLegacySmartData(t *testing.T) {
	query := analogJInfluxMigrationQuery(
		"metrics_weekly",
		"scrutiny",
		"9027cd65-fe19-562d-ac5d-0bb05818087f",
		"24e0a1be-6f0e-5c6f-9d4a-1b2c3d4e5f60",
		"nvme123",
		0,
	)

	require.Contains(t, query, `from(bucket: "metrics_weekly")`)
	require.Contains(t, query, `r["_measurement"] == "smart" or r["_measurement"] == "temp"`)
	require.Contains(t, query, `exists r["scrutiny_uuid"] and r["scrutiny_uuid"] == "9027cd65-fe19-562d-ac5d-0bb05818087f"`)
	require.Contains(t, query, `drop(columns: ["scrutiny_uuid"])`)
	// the device_wwn tag alone is not enough: deviceHistoryPredicate admits it only while
	// one device holds that WWN, so rewritten history must carry device_id too
	require.Contains(t, query, `set(key: "device_id", value: "24e0a1be-6f0e-5c6f-9d4a-1b2c3d4e5f60")`)
	require.Contains(t, query, `set(key: "device_wwn", value: "nvme123")`)
	require.Contains(t, query, `to(bucket: "metrics_weekly", org: "scrutiny")`)
	require.Equal(t, 1, strings.Count(query, `to(bucket:`))
}

func TestLegacyScrutinyUUIDMatchesAnalogJIdentity(t *testing.T) {
	require.Equal(t,
		"9027cd65-fe19-562d-ac5d-0bb05818087f",
		legacyScrutinyUUID("Samsung SSD 980 PRO 2TB", "NVME123", ""),
	)
	require.Equal(t,
		"7a14ef35-ed3a-552a-9980-46291bd986e3",
		legacyScrutinyUUID("WDC WD80EFZZ-68BTXN0", "WD-CA2XZ08L", "0x50014ee2c06ce3c3"),
	)
}

func TestAnalogJHistoryRewritePreservesSourceOnFailureAndRetries(t *testing.T) {
	for _, failure := range []string{"bucket lookup", "HTTP rejection", "streamed retention", "streamed field conflict", "delete"} {
		t.Run(failure, func(t *testing.T) {
			var fail atomic.Bool
			fail.Store(true)
			var deletes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/buckets":
					if fail.Load() && failure == "bucket lookup" {
						http.Error(w, "bucket access denied", http.StatusForbidden)
						return
					}
					if r.URL.Query().Get("org") == "" {
						// the lookup has to be organization-scoped: a query by bucket name alone
						// matches every organization the token can read
						http.Error(w, "organization-scoped bucket lookup required", http.StatusForbidden)
						return
					}
					// the lookup is organization-scoped, so the response is the organization's
					// bucket list rather than one bucket echoed back by name
					w.Header().Set("Content-Type", "application/json")
					listed := make([]domain.Bucket, 0, 4)
					for _, name := range []string{"metrics", "metrics_weekly", "metrics_monthly", "metrics_yearly"} {
						listed = append(listed, domain.Bucket{Name: name, RetentionRules: domain.RetentionRules{{EverySeconds: 5443200}}})
					}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"buckets": listed})
				case "/api/v2/query":
					if fail.Load() && failure == "HTTP rejection" {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"code":"unprocessable entity","message":"field type conflict"}`))
						return
					}
					w.Header().Set("Content-Type", "text/csv")
					if fail.Load() && strings.HasPrefix(failure, "streamed") {
						message := "partial write: field type conflict"
						if failure == "streamed retention" {
							message = "partial write: dropped 268 points outside retention policy of duration 1512h0m0s"
						}
						writer := csv.NewWriter(w)
						_ = writer.WriteAll([][]string{{"#datatype", "string", "string"}, {"", "error", "reference"}, {"", message, ""}})
					}
				case "/api/v2/delete":
					if fail.Load() && failure == "delete" {
						http.Error(w, "delete access denied", http.StatusForbidden)
						return
					}
					deletes.Add(1)
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			client := influxdb2.NewClient(server.URL, "test-token")
			t.Cleanup(client.Close)
			mockCtrl := gomock.NewController(t)
			fakeConfig := mock_config.NewMockInterface(mockCtrl)
			fakeConfig.EXPECT().GetString(cfgInfluxDBBucket).Return("metrics").AnyTimes()
			fakeConfig.EXPECT().GetString(cfgInfluxDBOrg).Return("scrutiny").AnyTimes()
			repo := &scrutinyRepository{
				appConfig: fakeConfig, influxClient: client,
				influxQueryApi: client.QueryAPI("scrutiny"), logger: logrus.New(),
			}
			mapping := map[string]analogJHistoryTarget{
				"legacy-uuid": {DeviceID: "current-device-id", WWN: "current-wwn"},
			}
			err := repo.rewriteAnalogJInfluxHistory(context.Background(), mapping)
			require.Error(t, err)
			require.Zero(t, deletes.Load(), "failed rewrites must not delete source history")
			fail.Store(false)
			require.NoError(t, repo.rewriteAnalogJInfluxHistory(context.Background(), mapping))
			require.EqualValues(t, 4, deletes.Load(), "retry must complete all four buckets")
		})
	}
}

func TestPrepareAnalogJDeviceRepairRestoresNVMeIdentityAndMergesDuplicate(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	legacyCreatedAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	model := "Samsung SSD 980 PRO 2TB"
	serial := "NVME123"
	legacyID := deviceid.Generate(model, serial, "")
	canonicalWWN := "nvme123"
	canonicalID := deviceid.Generate(model, serial, canonicalWWN)

	require.NoError(t, db.Create(&models.Device{
		DeviceID:                  legacyID,
		WWN:                       "",
		DeviceName:                "nvme0",
		ModelName:                 model,
		SerialNumber:              serial,
		DeviceProtocol:            pkg.DeviceProtocolNvme,
		Label:                     "Archive Bay",
		Archived:                  true,
		Muted:                     true,
		MissedPingTimeoutOverride: 90,
		SmartDisplayMode:          "raw",
		CreatedAt:                 legacyCreatedAt,
	}).Error)
	require.NoError(t, db.Create(&models.Device{
		DeviceID:       canonicalID,
		WWN:            canonicalWWN,
		DeviceName:     "nvme0",
		ModelName:      model,
		SerialNumber:   serial,
		DeviceProtocol: pkg.DeviceProtocolNvme,
		DeviceStatus:   pkg.DeviceStatusFailedScrutiny,
	}).Error)

	sataModel := "WDC WD80EFZZ-68BTXN0"
	sataSerial := "WD-CA2XZ08L"
	sataWWN := "0x50014ee2c06ce3c3"
	require.NoError(t, db.Create(&models.Device{
		DeviceID:     deviceid.Generate(sataModel, sataSerial, sataWWN),
		WWN:          sataWWN,
		DeviceName:   "sda",
		ModelName:    sataModel,
		SerialNumber: sataSerial,
	}).Error)

	mapping, err := repo.prepareAnalogJDeviceRepair(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, map[string]analogJHistoryTarget{
		"9027cd65-fe19-562d-ac5d-0bb05818087f": {DeviceID: canonicalID, WWN: canonicalWWN},
		"7a14ef35-ed3a-552a-9980-46291bd986e3": {
			DeviceID: deviceid.Generate(sataModel, sataSerial, sataWWN),
			WWN:      sataWWN,
		},
	}, mapping)

	var devices []models.Device
	require.NoError(t, db.Order("device_name ASC").Find(&devices).Error)
	require.Len(t, devices, 2)

	var repaired models.Device
	require.NoError(t, db.Where(queryDeviceID, canonicalID).First(&repaired).Error)
	require.Equal(t, canonicalWWN, repaired.WWN)
	require.Equal(t, "Archive Bay", repaired.Label)
	require.True(t, repaired.Archived)
	require.True(t, repaired.Muted)
	require.Equal(t, 90, repaired.MissedPingTimeoutOverride)
	require.Equal(t, "raw", repaired.SmartDisplayMode)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, repaired.DeviceStatus)
	require.Equal(t, legacyCreatedAt, repaired.CreatedAt.UTC())

	secondMapping, err := repo.prepareAnalogJDeviceRepair(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, mapping, secondMapping)
	require.NoError(t, db.Find(&devices).Error)
	require.Len(t, devices, 2)
}

// A drive hit by both 0.9.x bugs has its WWN cleared on both rows and its model name
// blanked on one of them. The row that keeps its model name has to survive: the repair
// merges every later row for a serial into the first one it processes, and
// isLegacyIdentityCandidate requires equal model names, so a blank-model survivor leaves
// the next collector run creating a fresh duplicate instead of reconciling.
func TestPrepareAnalogJDeviceRepairKeepsTheModelBearingRowWhenBothWWNsWereCleared(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	model := "Samsung SSD 980 PRO 2TB"
	serial := "NVME123"
	staleCreatedAt := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
	// the blank-model row is the older one, so created_at order alone would keep it
	require.NoError(t, db.Create(&models.Device{
		DeviceID:     deviceid.Generate("", serial, ""),
		SerialNumber: serial,
		DeviceName:   "nvme0",
		HostId:       "nas-1",
		Label:        "Cache",
		CreatedAt:    staleCreatedAt,
	}).Error)
	require.NoError(t, db.Create(&models.Device{
		DeviceID:     deviceid.Generate(model, serial, ""),
		ModelName:    model,
		SerialNumber: serial,
		DeviceName:   "nvme0",
		HostId:       "nas-1",
		CreatedAt:    time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
	}).Error)

	mapping, err := repo.prepareAnalogJDeviceRepair(context.Background(), db)
	require.NoError(t, err)

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 1)
	require.Equal(t, model, devices[0].ModelName)
	require.Equal(t, strings.ToLower(serial), devices[0].WWN)
	require.Equal(t, "Cache", devices[0].Label)
	require.Equal(t, staleCreatedAt, devices[0].CreatedAt.UTC())

	// both 0.9.x identities have to reach the surviving row, or the blank-model row's
	// history is orphaned
	survivor := analogJHistoryTarget{DeviceID: devices[0].DeviceID, WWN: devices[0].WWN}
	require.Equal(t, survivor, mapping[legacyScrutinyUUID("", serial, "")])
	require.Equal(t, survivor, mapping[legacyScrutinyUUID(model, serial, "")])
}

// An ambiguous serial must not abort the migration. Two cleared-WWN rows reporting
// different hosts each keep their own row, because analogJRepairCandidates excludes a
// candidate on a host disagreement, and a third row then matches both. Returning an error
// there rolled the whole chain back and left the web service restarting every second,
// which is the #1056 symptom reached by another route.
func TestPrepareAnalogJDeviceRepairLeavesAnAmbiguousSerialUnrepaired(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	serial := "S4000ABC"
	for _, row := range []struct {
		model string
		host  string
	}{
		{"ST4000VN008", "nas-1"},
		{"ST4000VN008-2DR166", "nas-2"},
		{"", ""},
	} {
		require.NoError(t, db.Create(&models.Device{
			DeviceID:     deviceid.Generate(row.model, serial, ""),
			SerialNumber: serial,
			ModelName:    row.model,
			HostId:       row.host,
		}).Error)
	}

	_, err := repo.prepareAnalogJDeviceRepair(context.Background(), db)
	require.NoError(t, err)

	// the two model-bearing rows each recovered a serial-fallback WWN; the ambiguous row is
	// left exactly as it was, for /api/device/{id}/merge_into to resolve
	var devices []models.Device
	require.NoError(t, db.Unscoped().Order("model_name ASC").Find(&devices).Error)
	require.Len(t, devices, 3)
	require.Empty(t, devices[0].ModelName)
	require.Empty(t, devices[0].WWN)
	for _, device := range devices[1:] {
		require.Equal(t, strings.ToLower(serial), device.WWN)
	}
}

func TestPrepareAnalogJDeviceRepairUsesCollectorDiscoveredWWN(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	model := "Micron NVMe"
	canonicalModel := model + " Gen4"
	serial := "MTFD123"
	legacyID := deviceid.Generate(model, serial, "")
	canonicalWWN := "eui.0025385b11aa2233"
	canonicalID := deviceid.Generate(canonicalModel, serial, canonicalWWN)
	deletedAt := time.Now().UTC()
	require.NoError(t, db.Create(&models.Device{
		DeviceID:     legacyID,
		ModelName:    model,
		SerialNumber: serial,
		HostId:       "nas-1",
		Label:        "Cache",
	}).Error)
	require.NoError(t, db.Create(&models.Device{
		DeviceID:     canonicalID,
		WWN:          canonicalWWN,
		ModelName:    canonicalModel,
		SerialNumber: serial,
		HostId:       "nas-1",
		DeletedAt:    &deletedAt,
	}).Error)

	mapping, err := repo.prepareAnalogJDeviceRepair(context.Background(), db)
	require.NoError(t, err)
	require.Equal(
		t,
		analogJHistoryTarget{DeviceID: canonicalID, WWN: canonicalWWN},
		mapping[legacyScrutinyUUID(model, serial, "")],
	)

	var devices []models.Device
	require.NoError(t, db.Find(&devices).Error)
	require.Len(t, devices, 1)
	require.Equal(t, canonicalID, devices[0].DeviceID)
	require.Equal(t, "Cache", devices[0].Label)
}

func newWWNDuplicateTestRepository(t *testing.T) (*scrutinyRepository, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Device{}))

	return &scrutinyRepository{gormClient: db, logger: logrus.New()}, db
}

// fixes #1056: AnalogJ/scrutiny 0.9.x forked one drive into two rows when its reported
// model name changed, because 0.9.x keyed devices by a UUID over model + serial + WWN.
// Both rows carry the same WWN and serial, so the drive appeared twice with its history
// split across the two device IDs.
func TestMergeAnalogJWWNDuplicatesCollapsesOneDrive(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	serial := "48L0A0ZXFVNE"
	wwn := "0x5000039xxxxxxxx1"
	model := "TOSHIBA AL13SEB300"
	staleCreatedAt := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
	staleUpdatedAt := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	staleID := deviceid.Generate("", serial, wwn)
	canonicalID := deviceid.Generate(model, serial, wwn)

	require.NoError(t, db.Create(&models.Device{
		DeviceID:                  staleID,
		WWN:                       wwn,
		SerialNumber:              serial,
		HostId:                    "truenas",
		Label:                     "Array Bay 3",
		Archived:                  true,
		Muted:                     true,
		MissedPingTimeoutOverride: 90,
		SmartDisplayMode:          "raw",
		CreatedAt:                 staleCreatedAt,
		UpdatedAt:                 staleUpdatedAt,
	}).Error)
	require.NoError(t, db.Create(&models.Device{
		DeviceID:     canonicalID,
		WWN:          wwn,
		ModelName:    model,
		SerialNumber: serial,
		HostId:       "truenas",
		DeviceStatus: pkg.DeviceStatusFailedScrutiny,
		CreatedAt:    time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC),
	}).Error)

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 1)
	require.Equal(t, canonicalID, devices[0].DeviceID)
	require.Equal(t, model, devices[0].ModelName)
	require.Equal(t, pkg.DeviceStatusFailedScrutiny, devices[0].DeviceStatus)
	require.Equal(t, staleCreatedAt, devices[0].CreatedAt.UTC())
	require.Equal(t, "Array Bay 3", devices[0].Label)
	require.True(t, devices[0].Archived)
	require.True(t, devices[0].Muted)
	require.Equal(t, 90, devices[0].MissedPingTimeoutOverride)
	require.Equal(t, "raw", devices[0].SmartDisplayMode)

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 1)
}

// The legacy scrutiny_uuid mapping is built before the merge, so an entry naming the
// deleted row has to be repointed or its 0.9.x history is rewritten onto a device_id that
// no longer exists. Self-test history keyed by the deleted device_id has to move too.
func TestMergeAnalogJWWNDuplicatesRepointsHistoryAndSelfTests(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)
	require.NoError(t, db.AutoMigrate(&models.DeviceSelfTest{}))

	serial := "48L0A0ZXFVNE"
	wwn := "0x5000039xxxxxxxx1"
	model := "TOSHIBA AL13SEB300"
	staleID := deviceid.Generate("", serial, wwn)
	canonicalID := deviceid.Generate(model, serial, wwn)

	require.NoError(t, db.Create(&models.Device{
		DeviceID: staleID, WWN: wwn, SerialNumber: serial,
		UpdatedAt: time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC),
	}).Error)
	require.NoError(t, db.Create(&models.Device{
		DeviceID: canonicalID, WWN: wwn, ModelName: model, SerialNumber: serial,
		UpdatedAt: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC),
	}).Error)
	require.NoError(t, db.Create(&models.DeviceSelfTest{
		DeviceID: staleID, DeviceIdentity: staleID, DeviceWWN: wwn, TypeString: "short",
	}).Error)

	staleUUID := legacyScrutinyUUID("", serial, wwn)
	canonicalUUID := legacyScrutinyUUID(model, serial, wwn)
	mapping := map[string]analogJHistoryTarget{
		staleUUID:     {DeviceID: staleID, WWN: wwn},
		canonicalUUID: {DeviceID: canonicalID, WWN: wwn},
	}

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, mapping))

	require.Equal(t, map[string]analogJHistoryTarget{
		staleUUID:     {DeviceID: canonicalID, WWN: wwn},
		canonicalUUID: {DeviceID: canonicalID, WWN: wwn},
	}, mapping)

	var selfTests []models.DeviceSelfTest
	require.NoError(t, db.Find(&selfTests).Error)
	require.Len(t, selfTests, 1)
	require.Equal(t, canonicalID, selfTests[0].DeviceID)
	require.Equal(t, canonicalID, selfTests[0].DeviceIdentity)
}

// Two rows that each report a model name are two drives as far as anything here can
// tell. 0.9.x blanked the model name on the row that stopped updating, so a blank on
// exactly one row is what identifies the pair; without it the hard delete is a guess.
func TestMergeAnalogJWWNDuplicatesKeepsRowsThatBothReportAModel(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	serial := "00000000000000000000"
	wwn := "0x0000000000000000"
	for _, model := range []string{"WDC WD40EFZX", "WDC WD40EPFX"} {
		require.NoError(t, db.Create(&models.Device{
			DeviceID:     deviceid.Generate(model, serial, wwn),
			WWN:          wwn,
			ModelName:    model,
			SerialNumber: serial,
			Capacity:     4000787030016,
		}).Error)
	}

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 2)
}

// Enclosure firmware that fabricates a WWN can fabricate a serial too, so a capacity
// disagreement is treated as two drives rather than one duplicated row.
// Two blank-model rows cannot come from deviceid.Generate, which would collide on the
// primary key, but RegisterDevice honours a caller-supplied device_id. Nothing then says
// which row is the live one, so refuse rather than hard-delete an arbitrary half.
func TestMergeAnalogJWWNDuplicatesKeepsRowsWhenNeitherReportsAModel(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	serial := "48L0A0ZXFVNE"
	wwn := "0x5000039xxxxxxxx1"
	for _, deviceID := range []string{"caller-supplied-a", "caller-supplied-b"} {
		require.NoError(t, db.Create(&models.Device{
			DeviceID:     deviceID,
			WWN:          wwn,
			SerialNumber: serial,
		}).Error)
	}

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 2)
}

func TestMergeAnalogJWWNDuplicatesKeepsDifferentCapacities(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	serial := "00000000000000000000"
	wwn := "0x0000000000000000"
	for _, row := range []struct {
		model    string
		capacity int64
	}{{"", 4000787030016}, {"ST8000VN004", 8001563222016}} {
		require.NoError(t, db.Create(&models.Device{
			DeviceID:     deviceid.Generate(row.model, serial, wwn),
			WWN:          wwn,
			ModelName:    row.model,
			SerialNumber: serial,
			Capacity:     row.capacity,
		}).Error)
	}

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 2)
}

// #851: drives behind one controller, or reporting a vendor-default WWN, share the WWN
// and are told apart by their serials. Those are two drives, not a duplicated row.
func TestMergeAnalogJWWNDuplicatesKeepsDistinctSerials(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	wwn := "0x600508b1001039343720202020200016"
	model := "HITACHI HUC106060CSS600"
	// one row carries the 0.9.x blank model name, so the differing serials are the only
	// thing keeping these two drives apart
	for i, serial := range []string{"PPKJA1ZB", "PPK4ZTUB"} {
		rowModel := model
		if i == 0 {
			rowModel = ""
		}
		require.NoError(t, db.Create(&models.Device{
			DeviceID:     deviceid.Generate(rowModel, serial, wwn),
			WWN:          wwn,
			ModelName:    rowModel,
			SerialNumber: serial,
			HostId:       "nas-1",
		}).Error)
	}

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 2)
}

// A drive whose reported model string changed twice leaves three rows in one group.
// Nothing says which model-bearing row the blank one belongs to, so refuse the group
// rather than merging an arbitrary pair of it.
func TestMergeAnalogJWWNDuplicatesKeepsGroupsLargerThanAPair(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	serial := "48L0A0ZXFVNE"
	wwn := "0x5000039xxxxxxxx1"
	for _, model := range []string{"", "TOSHIBA AL13SEB300", "TOSHIBA AL13SEB300A"} {
		require.NoError(t, db.Create(&models.Device{
			DeviceID:     deviceid.Generate(model, serial, wwn),
			WWN:          wwn,
			ModelName:    model,
			SerialNumber: serial,
		}).Error)
	}

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 3)
}

// Two collectors reporting one drive is not a duplicated row, so the migration leaves
// the choice to the /api/device/{id}/merge_into endpoint.
func TestMergeAnalogJWWNDuplicatesKeepsConflictingHosts(t *testing.T) {
	repo, db := newWWNDuplicateTestRepository(t)

	serial := "WD-CA2XZ08L"
	wwn := "0x50014ee2c06ce3c3"
	model := "WDC WD80EFZZ-68BTXN0"
	// Two collectors register one drive under distinct device ids because each adds its
	// own host to the identity; deviceid.Generate ignores the host when a WWN exists.
	for _, row := range []struct {
		model string
		host  string
	}{{"", "nas-1"}, {model, "nas-2"}} {
		require.NoError(t, db.Create(&models.Device{
			DeviceID:     deviceid.Generate(row.model, serial, wwn+"-"+row.host),
			WWN:          wwn,
			ModelName:    row.model,
			SerialNumber: serial,
			DeviceName:   "sda",
			HostId:       row.host,
		}).Error)
	}

	require.NoError(t, repo.mergeAnalogJWWNDuplicates(context.Background(), db, map[string]analogJHistoryTarget{}))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 2)
}

// The merge is reached only through the m20260216155600 lineage gate, and only after
// prepareAnalogJDeviceRepair has built the history mapping, so exercise it through
// migrateM20260803000000 rather than calling the merge directly.
func TestMigrateM20260803000000MergesDuplicatesAndRewritesHistoryOnce(t *testing.T) {
	var rewritten []string
	var deletes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/buckets":
			w.Header().Set("Content-Type", "application/json")
			listed := make([]domain.Bucket, 0, 4)
			for _, name := range []string{"metrics", "metrics_weekly", "metrics_monthly", "metrics_yearly"} {
				listed = append(listed, domain.Bucket{Name: name})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"buckets": listed})
		case "/api/v2/query":
			// the client posts the Flux as a JSON field, so compare the decoded query
			var payload struct {
				Query string `json:"query"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			rewritten = append(rewritten, payload.Query)
			w.Header().Set("Content-Type", "text/csv")
		case "/api/v2/delete":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			deletes = append(deletes, string(body))
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := influxdb2.NewClient(server.URL, "test-token")
	t.Cleanup(client.Close)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString(cfgInfluxDBBucket).Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString(cfgInfluxDBOrg).Return("scrutiny").AnyTimes()

	_, db := newWWNDuplicateTestRepository(t)
	require.NoError(t, db.Exec(`CREATE TABLE migrations (id TEXT NOT NULL PRIMARY KEY)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO migrations (id) VALUES (?)`, analogJScrutinyUUIDMigrationID).Error)

	serial := "48L0A0ZXFVNE"
	wwn := "0x5000039xxxxxxxx1"
	model := "TOSHIBA AL13SEB300"
	staleID := deviceid.Generate("", serial, wwn)
	canonicalID := deviceid.Generate(model, serial, wwn)
	require.NoError(t, db.Create(&models.Device{
		DeviceID: staleID, WWN: wwn, SerialNumber: serial, Label: "Bay 3",
		CreatedAt: time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC),
	}).Error)
	require.NoError(t, db.Create(&models.Device{
		DeviceID: canonicalID, WWN: wwn, ModelName: model, SerialNumber: serial,
		CreatedAt: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC),
	}).Error)

	repo := &scrutinyRepository{
		appConfig: fakeConfig, influxClient: client,
		influxQueryApi: client.QueryAPI("scrutiny"), gormClient: db, logger: logrus.New(),
	}
	require.NoError(t, repo.migrateM20260803000000(context.Background(), db))

	var devices []models.Device
	require.NoError(t, db.Unscoped().Find(&devices).Error)
	require.Len(t, devices, 1)
	require.Equal(t, canonicalID, devices[0].DeviceID)
	require.Equal(t, "Bay 3", devices[0].Label)
	require.Equal(t, time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC), devices[0].CreatedAt.UTC())

	// Two legacy identities across four buckets, and every rewrite lands on the surviving
	// device_id rather than the deleted one.
	require.Len(t, rewritten, 8)
	require.Len(t, deletes, 8)
	for _, query := range rewritten {
		require.Contains(t, query, `set(key: "device_id", value: "`+canonicalID+`")`)
		require.NotContains(t, query, staleID)
	}
	for _, uuid := range []string{legacyScrutinyUUID("", serial, wwn), legacyScrutinyUUID(model, serial, wwn)} {
		found := false
		for _, query := range rewritten {
			if strings.Contains(query, uuid) {
				found = true
			}
		}
		require.True(t, found, "no rewrite covered legacy scrutiny_uuid %s", uuid)
	}
}

func TestAnalogJMigrationIntegration(t *testing.T) {
	ResetMigrationGuardForTests()

	influxHost := integrationInfluxHost(t)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.database.location").Return(filepath.Join(t.TempDir(), "scrutiny.db")).AnyTimes()
	fakeConfig.EXPECT().GetString("web.database.journal_mode").Return("WAL").AnyTimes()
	fakeConfig.EXPECT().GetString("web.database.type").Return("sqlite").AnyTimes()
	fakeConfig.EXPECT().GetString("log.level").Return("INFO").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.scheme").Return("http").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.host").Return(influxHost).AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.port").Return("8086").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.token").Return("my-super-secret-auth-token").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.init_username").Return("admin").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.init_password").Return("password12345").AnyTimes()
	fakeConfig.EXPECT().GetBool("web.influxdb.tls.insecure_skip_verify").Return(false).AnyTimes()
	fakeConfig.EXPECT().GetBool("web.influxdb.retention_policy").Return(false).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention_policy.default_retention_period_days").Return(0).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention_policy.monthly_retention_period_months").Return(0).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention_policy.yearly_retention_period_months").Return(0).AnyTimes()
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().GetStringSlice("failures.ignored.devstat").Return([]string{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	repoInterface, err := NewScrutinyRepository(fakeConfig, logrus.WithField("test", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repoInterface.Close()) })
	repo := repoInterface.(*scrutinyRepository)
	ctx := context.Background()

	model := "Issue 694 NVMe"
	serial := "ISSUE694NVME"
	legacyUUID := legacyScrutinyUUID(model, serial, "")
	targetWWN := strings.ToLower(serial)
	legacyDeviceID := deviceid.Generate(model, serial, "")
	currentDeviceID := deviceid.Generate(model, serial, targetWWN)
	require.NoError(t, repo.gormClient.Create(&models.Device{
		DeviceID:       legacyDeviceID,
		DeviceName:     "nvme694",
		ModelName:      model,
		SerialNumber:   serial,
		DeviceProtocol: pkg.DeviceProtocolNvme,
	}).Error)
	require.NoError(t, repo.gormClient.Exec(
		"INSERT OR IGNORE INTO migrations (id) VALUES (?)",
		analogJScrutinyUUIDMigrationID,
	).Error)

	buckets := []string{"metrics", "metrics_weekly", "metrics_monthly", "metrics_yearly"}
	pointTime := time.Now().UTC().Add(-2 * time.Hour)
	for _, bucket := range buckets {
		writeAPI := repo.influxClient.WriteAPIBlocking("scrutiny", bucket)
		tags := map[string]string{"scrutiny_uuid": legacyUUID, "device_protocol": pkg.DeviceProtocolNvme}
		// Seed expired history while retention is unlimited, then shorten the
		// window. InfluxDB can still read these points until shard cleanup (#776).
		for _, timestamp := range []time.Time{pointTime.Add(-64 * 24 * time.Hour), pointTime} {
			require.NoError(t, writeAPI.WritePoint(ctx, influxdb2.NewPoint(
				"smart",
				tags,
				map[string]interface{}{"temp": int64(41), "power_on_hours": int64(694)},
				timestamp,
			)))
			require.NoError(t, writeAPI.WritePoint(ctx, influxdb2.NewPoint(
				"temp",
				tags,
				map[string]interface{}{"temp": int64(41)},
				timestamp,
			)))
		}
		if bucket != "metrics_yearly" {
			bucketInfo, err := repo.influxClient.BucketsAPI().FindBucketByName(ctx, bucket)
			require.NoError(t, err)
			originalRules := bucketInfo.RetentionRules
			t.Cleanup(func() {
				bucketInfo.RetentionRules = originalRules
				_, err := repo.influxClient.BucketsAPI().UpdateBucket(ctx, bucketInfo)
				require.NoError(t, err)
			})
			bucketInfo.RetentionRules = domain.RetentionRules{{EverySeconds: 63 * 24 * 60 * 60}}
			_, err = repo.influxClient.BucketsAPI().UpdateBucket(ctx, bucketInfo)
			require.NoError(t, err)
		}
		require.Equal(t, 6, influxRecordCount(t, repo, bucket, "scrutiny_uuid", legacyUUID))
	}

	deleteStart := time.Unix(0, 0).UTC()
	deleteStop := time.Now().UTC().Add(24 * time.Hour)
	t.Cleanup(func() {
		for _, bucket := range buckets {
			_ = repo.influxClient.DeleteAPI().DeleteWithName(ctx, "scrutiny", bucket, deleteStart, deleteStop, fmt.Sprintf("device_wwn=%q", targetWWN))
			_ = repo.influxClient.DeleteAPI().DeleteWithName(ctx, "scrutiny", bucket, deleteStart, deleteStop, fmt.Sprintf("scrutiny_uuid=%q", legacyUUID))
		}
	})

	require.NoError(t, repo.gormClient.Transaction(func(tx *gorm.DB) error {
		return repo.migrateM20260803000000(ctx, tx)
	}))

	var repaired models.Device
	require.NoError(t, repo.gormClient.Where(queryDeviceID, currentDeviceID).First(&repaired).Error)
	require.Equal(t, targetWWN, repaired.WWN)
	for _, bucket := range buckets {
		expectedFields := 3
		if bucket == "metrics_yearly" {
			expectedFields = 6
		}
		require.Equal(t, expectedFields, influxRecordCount(t, repo, bucket, "device_wwn", targetWWN))
		// device_wwn alone is unreadable once a second device holds that WWN, so the
		// rewrite has to land device_id as a filterable tag too
		require.Equal(t, expectedFields, influxRecordCount(t, repo, bucket, "device_id", currentDeviceID))
		require.Zero(t, influxRecordCount(t, repo, bucket, "scrutiny_uuid", legacyUUID))
	}

	summary, err := repo.GetSummary(ctx)
	require.NoError(t, err)
	require.NotNil(t, summary[currentDeviceID].SmartResults)
	require.Equal(t, int64(694), summary[currentDeviceID].SmartResults.PowerOnHours)

	require.NoError(t, repo.gormClient.Transaction(func(tx *gorm.DB) error {
		return repo.migrateM20260803000000(ctx, tx)
	}))
	require.Equal(t, 3, influxRecordCount(t, repo, "metrics", "device_wwn", targetWWN))
}

func influxRecordCount(t *testing.T, repo *scrutinyRepository, bucket, tag, value string) int {
	t.Helper()
	query := fmt.Sprintf(`from(bucket: %q)
|> range(start: 0)
|> filter(fn: (r) => r[%q] == %q)
|> count()`, bucket, tag, value)
	result, err := repo.influxQueryApi.Query(context.Background(), query)
	require.NoError(t, err)
	defer result.Close()
	count := 0
	for result.Next() {
		count += int(result.Record().Value().(int64))
	}
	require.NoError(t, result.Err())
	return count
}
