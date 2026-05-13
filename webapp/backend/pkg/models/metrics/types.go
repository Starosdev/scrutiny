package metrics

import (
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
)

// DeviceMetricsData stores metrics data for a single device
type DeviceMetricsData struct {
	UpdatedAt time.Time          `json:"updated_at"`
	SmartData measurements.Smart `json:"smart_data"`
	Device    models.Device      `json:"device"`
}

// ZFSPoolMetricsData stores metrics data for a single ZFS pool.
type ZFSPoolMetricsData struct {
	UpdatedAt time.Time      `json:"updated_at"`
	Pool      models.ZFSPool `json:"pool"`
}

// WorkloadMetricsData stores workload metrics data for a single device.
type WorkloadMetricsData struct {
	UpdatedAt time.Time              `json:"updated_at"`
	Insight   models.WorkloadInsight `json:"insight"`
}
