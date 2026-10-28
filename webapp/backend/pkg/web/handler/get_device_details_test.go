package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var assertFilesystemReadError = errors.New("filesystem_devices table is unreadable")

type deviceDetailsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Device             models.Device                `json:"device"`
		Filesystems        []models.FilesystemCapacity  `json:"filesystems"`
		FilesystemStatus   *models.FilesystemHostStatus `json:"filesystem_status"`
		CollectorReporting bool                         `json:"filesystem_collector_reporting"`
	} `json:"data"`
}

// deviceDetailsRequest serves GET /api/device/device-1/details against a repo
// whose filesystem summary is the one supplied, and returns the decoded body.
func deviceDetailsRequest(
	t *testing.T,
	device models.Device,
	filesystems map[string][]models.FilesystemCapacity,
	hostStatuses map[string]*models.FilesystemHostStatus,
	summaryErr error,
) (*httptest.ResponseRecorder, deviceDetailsResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	fakeRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	fakeRepo.EXPECT().GetDeviceDetails(gomock.Any(), "device-1").Return(device, nil)
	fakeRepo.EXPECT().
		GetSmartAttributeHistory(gomock.Any(), "device-1", "forever", 0, 0, nil).
		Return([]measurements.Smart{}, nil)
	fakeRepo.EXPECT().GetMergedOverrides(gomock.Any()).Return([]overrides.AttributeOverride{})
	fakeRepo.EXPECT().GetFilesystemSummary(gomock.Any()).Return(filesystems, hostStatuses, summaryErr)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", fakeRepo)
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Next()
	})
	r.GET("/api/device/:id/details", handler.GetDeviceDetails)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/device/device-1/details", nil)
	r.ServeHTTP(w, req)

	var response deviceDetailsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	return w, response
}

func testDevice() models.Device {
	return models.Device{
		DeviceID:       "device-1",
		WWN:            testDeviceWWN,
		DeviceName:     "sda",
		HostId:         "atlas",
		DeviceProtocol: pkg.DeviceProtocolAta,
	}
}

func TestGetDeviceDetailsReturnsOnlyThisDrivesFilesystems(t *testing.T) {
	filesystems := map[string][]models.FilesystemCapacity{
		"atlas": {
			{HostID: "atlas", MountPoint: "/", SourceDevice: "/dev/sda1", AvailableBytes: 40, BackingDevices: []string{"sda"}},
			{HostID: "atlas", MountPoint: "/data", SourceDevice: "/dev/sdb1", AvailableBytes: 90, BackingDevices: []string{"sdb"}},
			{HostID: "atlas", MountPoint: "/array", SourceDevice: "/dev/md0", AvailableBytes: 10, BackingDevices: []string{"sda", "sdb"}},
			{HostID: "atlas", MountPoint: "/remote", SourceDevice: "nas:/export", AvailableBytes: 5},
		},
		// Another host's drive of the same name must not leak in.
		"hermes": {
			{HostID: "hermes", MountPoint: "/", SourceDevice: "/dev/sda1", AvailableBytes: 1, BackingDevices: []string{"sda"}},
		},
	}
	hostStatuses := map[string]*models.FilesystemHostStatus{
		"atlas": {HostID: "atlas", Status: models.FilesystemHostStatusAvailable, FilesystemCount: 4},
	}

	w, response := deviceDetailsRequest(t, testDevice(), filesystems, hostStatuses, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, response.Success)
	require.Len(t, response.Data.Filesystems, 2)
	require.Equal(t, "/", response.Data.Filesystems[0].MountPoint)
	// The array spans this drive too, so it belongs here with its own mount.
	require.Equal(t, "/array", response.Data.Filesystems[1].MountPoint)
	require.Equal(t, []string{"sda", "sdb"}, response.Data.Filesystems[1].BackingDevices)
	require.Equal(t, models.FilesystemHostStatusAvailable, response.Data.FilesystemStatus.Status)
}

func TestGetDeviceDetailsCollapsesRepeatedMountsOfOneFilesystem(t *testing.T) {
	// Two btrfs subvolume mounts of one filesystem on one partition.
	filesystems := map[string][]models.FilesystemCapacity{
		"atlas": {
			{HostID: "atlas", MountPoint: "/", SourceDevice: "/dev/sda2", FilesystemType: "btrfs", AvailableBytes: 500, BackingDevices: []string{"sda"}},
			{HostID: "atlas", MountPoint: "/home", SourceDevice: "/dev/sda2", FilesystemType: "btrfs", AvailableBytes: 500, BackingDevices: []string{"sda"}},
		},
	}

	_, response := deviceDetailsRequest(t, testDevice(), filesystems, map[string]*models.FilesystemHostStatus{}, nil)

	require.Len(t, response.Data.Filesystems, 1)
	require.Equal(t, "/", response.Data.Filesystems[0].MountPoint)
}

func TestGetDeviceDetailsOmitsFilesystemStatusWhenHostNeverReported(t *testing.T) {
	w, response := deviceDetailsRequest(t, testDevice(), map[string][]models.FilesystemCapacity{}, map[string]*models.FilesystemHostStatus{}, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, response.Data.Filesystems)
	// Omitted rather than null: an absent key and an empty list are different
	// facts, and the detail page's panel gate reads both.
	require.Nil(t, response.Data.FilesystemStatus)
	require.NotContains(t, w.Body.String(), `"filesystem_status"`)
	require.Contains(t, w.Body.String(), `"filesystems":[]`)
	// Nothing anywhere: the page renders no panel at all rather than a card
	// explaining a mismatch that does not exist.
	require.False(t, response.Data.CollectorReporting)
}

func TestGetDeviceDetailsReportsCollectorRunningUnderAnotherHostID(t *testing.T) {
	// The collector is enabled and reporting, but under "default" rather than
	// the empty host id this drive's host carries, so nothing attributes. The
	// detail page needs to be able to say so, which it cannot do from an empty
	// filesystems list alone.
	filesystems := map[string][]models.FilesystemCapacity{
		"default": {
			{HostID: "default", MountPoint: "/", SourceDevice: "/dev/sda1", AvailableBytes: 40, BackingDevices: []string{"sda"}},
		},
	}
	hostStatuses := map[string]*models.FilesystemHostStatus{
		"default": {HostID: "default", Status: models.FilesystemHostStatusAvailable, FilesystemCount: 1},
	}

	device := testDevice()
	device.HostId = ""

	_, response := deviceDetailsRequest(t, device, filesystems, hostStatuses, nil)

	require.Empty(t, response.Data.Filesystems)
	require.Nil(t, response.Data.FilesystemStatus)
	require.True(t, response.Data.CollectorReporting)
}

func TestGetDeviceDetailsStillServesSmartDataWhenFilesystemReadFails(t *testing.T) {
	w, response := deviceDetailsRequest(t, testDevice(), nil, nil, assertFilesystemReadError)

	// The page exists to show SMART data; an optional collector's table being
	// unreadable must not take the whole response down.
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, response.Success)
	require.Equal(t, "device-1", response.Data.Device.DeviceID)
	require.Empty(t, response.Data.Filesystems)
	require.Nil(t, response.Data.FilesystemStatus)
}
