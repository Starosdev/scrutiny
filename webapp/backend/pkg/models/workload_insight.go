package models

// WorkloadInsight contains computed workload metrics for a single device
type WorkloadInsight struct {
	DeviceWWN      string `json:"device_wwn"`
	DeviceProtocol string `json:"device_protocol"`

	// Data quality
	DataPoints    int     `json:"data_points"`
	TimeSpanHours float64 `json:"time_span_hours"`

	// Computed rates (bytes per day)
	DailyWriteBytes int64 `json:"daily_write_bytes"`
	DailyReadBytes  int64 `json:"daily_read_bytes"`

	// Cumulative totals within the queried duration (bytes)
	TotalWriteBytes int64 `json:"total_write_bytes"`
	TotalReadBytes  int64 `json:"total_read_bytes"`

	// Ratio: reads / writes (0 if no writes)
	ReadWriteRatio float64 `json:"read_write_ratio"`

	// Classification: "heavy", "medium", "light", "idle", "unknown"
	Intensity string `json:"intensity"`

	// SSD endurance (nil for HDDs or when data unavailable)
	Endurance *EnduranceEstimate `json:"endurance,omitempty"`

	// Spike detection (nil when no spike or insufficient data)
	Spike *ActivitySpike `json:"spike,omitempty"`
}

// EnduranceEstimate projects SSD remaining lifespan
type EnduranceEstimate struct {
	Available             bool    `json:"available"`
	PercentageUsed        int64   `json:"percentage_used"`
	EstimatedLifespanDays int64   `json:"estimated_lifespan_days,omitempty"`
	TBWrittenSoFar        float64 `json:"tbw_so_far"`
}

// ActivitySpike indicates unusual write activity compared to baseline
type ActivitySpike struct {
	Detected                bool    `json:"detected"`
	RecentDailyWriteBytes   int64   `json:"recent_daily_write_bytes"`
	BaselineDailyWriteBytes int64   `json:"baseline_daily_write_bytes"`
	SpikeFactor             float64 `json:"spike_factor"`
	Description             string  `json:"description"`
}
