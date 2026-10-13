package collector

import (
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mock_shell "github.com/analogj/scrutiny/collector/pkg/common/shell/mock"
	"github.com/analogj/scrutiny/collector/pkg/detect"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// runCollectAll collects five devices and returns the highest number of concurrent
// smartctl calls and the order in which devices were started.
func runCollectAll(t *testing.T, parallelism int) (int32, []string) {
	t.Helper()
	ctrl := gomock.NewController(t)
	var inFlight, peak int32
	var mu sync.Mutex
	var arrived int32
	gate := make(chan struct{})
	var order []string

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl", gomock.Any(), "", gomock.Any()).
		DoAndReturn(func(_ any, _ any, _ string, args []string, _ string, _ []string) (string, error) {
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if cur <= p || atomic.CompareAndSwapInt32(&peak, p, cur) {
					break
				}
			}
			mu.Lock()
			order = append(order, args[len(args)-1])
			mu.Unlock()
			// Hold the first `parallelism` callers until all of them are in flight, which
			// proves they ran concurrently; later callers pass straight through.
			if atomic.AddInt32(&arrived, 1) <= int32(parallelism) {
				if atomic.LoadInt32(&arrived) == int32(parallelism) {
					close(gate)
				}
				select {
				case <-gate:
				case <-time.After(5 * time.Second):
					t.Error("timed out waiting for concurrent callers")
				}
			}
			atomic.AddInt32(&inFlight, -1)
			return someSmartPayload, nil
		}).
		Times(5)

	mc := newTestCollectCollector(t, fakeShell, nil)
	mc.config.Set(configKeyMetricsParallelism, parallelism)

	devices := []models.Device{}
	for _, n := range []string{"sda", "sdb", "sdc", "sdd", "sde"} {
		devices = append(devices, models.Device{DeviceID: "id-" + n, DeviceName: n, DeviceType: "sat"})
	}
	mc.collectAll(devices)
	return atomic.LoadInt32(&peak), order
}

func TestMetricsCollector_CollectAll_DefaultIsSequentialAndOrdered(t *testing.T) {
	peak, order := runCollectAll(t, 1)
	require.Equal(t, int32(1), peak)
	require.Equal(t, []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd", "/dev/sde"}, order)
}

func TestMetricsCollector_CollectAll_BoundedParallelism(t *testing.T) {
	peak, _ := runCollectAll(t, 3)
	require.Equal(t, int32(3), peak)
}

func TestMetricsCollector_ReportSkipped_OneScanErrorPerDevice(t *testing.T) {
	var posts int32
	var body string
	mc := newTestCollectCollector(t, nil, func(req *http.Request) {
		atomic.AddInt32(&posts, 1)
		b, _ := io.ReadAll(req.Body)
		body = string(b)
		require.Equal(t, "/api/collector/scan-error", req.URL.Path)
	})
	mc.reportSkipped([]detect.SkippedDevice{{DeviceName: "sdb", Reason: "smartctl --info failed for sdb: exit status 2"}})
	require.Equal(t, int32(1), atomic.LoadInt32(&posts))
	require.Contains(t, body, `"error_type":"info"`)
	require.Contains(t, body, `"device_name":"sdb"`)
}

func TestMetricsCollector_CollectAll_InvalidParallelismFallsBackToOne(t *testing.T) {
	peak, _ := runCollectAll(t, 0)
	require.Equal(t, int32(1), peak)
}
