package models

// MissedPingStatusData contains the current status of the missed ping monitor
type MissedPingStatusData struct {
	// Configuration
	Enabled              bool `json:"enabled"`
	TimeoutMinutes       int  `json:"timeout_minutes"`
	CheckIntervalMinutes int  `json:"check_interval_minutes"`

	// Notification configuration
	NotifyConfigured    bool `json:"notify_configured"`
	NotifyEndpointCount int  `json:"notify_endpoint_count"`

	// Operational status
	LastCheckTime  string `json:"last_check_time"`  // RFC3339
	NextCheckTime  string `json:"next_check_time"`  // RFC3339
	MonitorRunning bool   `json:"monitor_running"`

	// Device tracking
	TotalDevices           int                  `json:"total_devices"`
	MonitoredDevices       int                  `json:"monitored_devices"`
	NotifiedDevices        []string             `json:"notified_devices"`
	NotifiedDevicesDetails []NotifiedDeviceInfo `json:"notified_devices_details"`

	// InfluxDB validation
	InfluxDBStatus InfluxDBStatusInfo `json:"influxdb_status"`

	// Errors
	LastError     string `json:"last_error,omitempty"`
	LastErrorTime string `json:"last_error_time,omitempty"`
}

// NotifiedDeviceInfo contains details about a device with an active notification
type NotifiedDeviceInfo struct {
	WWN              string `json:"wwn"`
	DeviceName       string `json:"device_name"`
	NotificationTime string `json:"notification_time"`
	LastSeenTime     string `json:"last_seen_time"`
}

// InfluxDBStatusInfo contains status of InfluxDB bucket validation
type InfluxDBStatusInfo struct {
	Available      bool     `json:"available"`
	BucketsFound   []string `json:"buckets_found"`
	BucketsMissing []string `json:"buckets_missing,omitempty"`
	Error          string   `json:"error,omitempty"`
}
