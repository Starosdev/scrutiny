package models

// MissedPingStatusData contains the current status of the missed ping monitor
type MissedPingStatusData struct {
	NextCheckTime          string               `json:"next_check_time"`
	LastErrorTime          string               `json:"last_error_time,omitempty"`
	LastError              string               `json:"last_error,omitempty"`
	LastCheckTime          string               `json:"last_check_time"`
	NotifiedDevicesDetails []NotifiedDeviceInfo `json:"notified_devices_details"`
	NotifiedDevices        []string             `json:"notified_devices"`
	InfluxDBStatus         InfluxDBStatusInfo   `json:"influxdb_status"`
	MonitoredDevices       int                  `json:"monitored_devices"`
	TotalDevices           int                  `json:"total_devices"`
	NotifyEndpointCount    int                  `json:"notify_endpoint_count"`
	CheckIntervalMinutes   int                  `json:"check_interval_minutes"`
	TimeoutMinutes         int                  `json:"timeout_minutes"`
	Enabled                bool                 `json:"enabled"`
	MonitorRunning         bool                 `json:"monitor_running"`
	NotifyConfigured       bool                 `json:"notify_configured"`
}

// NotifiedDeviceInfo contains details about a device with an active notification
type NotifiedDeviceInfo struct {
	DeviceID         string `json:"device_id"`
	WWN              string `json:"wwn"`
	DeviceName       string `json:"device_name"`
	NotificationTime string `json:"notification_time"`
	LastSeenTime     string `json:"last_seen_time"`
}

// InfluxDBStatusInfo contains status of InfluxDB bucket validation
type InfluxDBStatusInfo struct {
	Error          string   `json:"error,omitempty"`
	BucketsFound   []string `json:"buckets_found"`
	BucketsMissing []string `json:"buckets_missing,omitempty"`
	Available      bool     `json:"available"`
}
