package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/stretchr/testify/require"
)

func TestMissedPingMonitorFlushesQuietQueueIndependently(t *testing.T) {
	for _, scenario := range []string{"disabled", "enabled", "device query fails", "last seen query fails", "settings missing", "settings query fails", "empty queue"} {
		t.Run(scenario, func(t *testing.T) {
			var deliveries atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				deliveries.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			ae, ctrl := createTestAppEngine(t)
			gate := notify.NewNotificationGate(ae.Logger)
			ae.NotificationGate = gate
			monitor := NewMissedPingMonitor(ae)
			defer monitor.cancel()
			repo := mock_database.NewMockDeviceRepo(ctrl)
			monitor.deviceRepo = repo
			now := time.Now()
			quiet := &models.Settings{}
			quiet.Metrics.NotificationQuietStart = now.Add(-time.Hour).Format("15:04")
			quiet.Metrics.NotificationQuietEnd = now.Add(time.Hour).Format("15:04")
			const threshold = 55
			send := func(since time.Time) bool {
				n := notify.NewTemperatureNotify(ae.Logger, ae.Config, &models.Device{DeviceName: "/dev/sda"}, threshold, threshold, since, now, "celsius")
				return gate.TrySend(&n, quiet, false)
			}
			if scenario != "empty queue" {
				gate.Temperature().Evaluate("drive", threshold, threshold, 0, now, nil, send)
				require.Equal(t, 1, gate.QueueLength())
			}
			settings := &models.Settings{}
			settings.Metrics.NotifyOnMissedPing = scenario == "enabled" || scenario == "device query fails" || scenario == "last seen query fails"
			queryError := errors.New("query failed")
			var settingsError error
			switch scenario {
			case "settings missing":
				settings = nil
			case "settings query fails":
				settingsError = queryError
				repo.EXPECT().Close().Return(nil)
			case "device query fails":
				repo.EXPECT().GetDevices(monitor.ctx).Return(nil, queryError)
				repo.EXPECT().Close().Return(nil)
			case "last seen query fails":
				repo.EXPECT().GetDevices(monitor.ctx).Return(nil, nil)
				repo.EXPECT().GetDevicesLastSeenTimes(monitor.ctx).Return(nil, queryError)
				repo.EXPECT().Close().Return(nil)
			case "enabled":
				repo.EXPECT().GetDevices(monitor.ctx).Return(nil, nil)
				repo.EXPECT().GetDevicesLastSeenTimes(monitor.ctx).Return(nil, nil)
			}
			repo.EXPECT().LoadSettings(monitor.ctx).Return(settings, settingsError)
			canFlush := settings != nil && settingsError == nil && scenario != "empty queue"
			if canFlush {
				repo.EXPECT().GetNotifyUrls(monitor.ctx).Return([]models.NotifyUrl{{URL: server.URL}}, nil)
			}
			monitor.checkMissedPings()
			if canFlush {
				require.EqualValues(t, 1, deliveries.Load())
				require.Zero(t, gate.QueueLength())
				gate.Temperature().Evaluate("drive", threshold, threshold, 0, now.Add(time.Minute), nil, send)
				require.Zero(t, gate.QueueLength(), "a delivered excursion must not be queued again")
			} else {
				require.Zero(t, deliveries.Load())
				if scenario != "empty queue" {
					require.Equal(t, 1, gate.QueueLength())
				}
			}
		})
	}
}
