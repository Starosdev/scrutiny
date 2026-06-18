package measurements

import "time"

//nolint:govet // Keep measurement fields ordered to match the serialized metric payload.
type BtrfsMetrics struct {
	FilesystemUUID    string    `json:"filesystem_uuid"`
	HostID            string    `json:"host_id"`
	Label             string    `json:"label"`
	Date              time.Time `json:"date"`
	DeviceSize        int64     `json:"device_size"`
	DeviceAllocated   int64     `json:"device_allocated"`
	DeviceUnallocated int64     `json:"device_unallocated"`
	DeviceMissing     int64     `json:"device_missing"`
	Used              int64     `json:"used"`
	FreeEstimated     int64     `json:"free_estimated"`
	FreeStatfs        int64     `json:"free_statfs"`
	ScrubReadErrors   int64     `json:"scrub_read_errors"`
	ScrubCsumErrors   int64     `json:"scrub_csum_errors"`
	ScrubVerifyErrors int64     `json:"scrub_verify_errors"`
	ScrubSuperErrors  int64     `json:"scrub_super_errors"`
	DataRatio         float64   `json:"data_ratio"`
	MetadataRatio     float64   `json:"metadata_ratio"`
	Status            string    `json:"status"`
	ScrubState        string    `json:"scrub_state"`
}

func (m *BtrfsMetrics) Flatten() (map[string]string, map[string]interface{}) {
	return map[string]string{
			"filesystem_uuid": m.FilesystemUUID,
			"host_id":         m.HostID,
			"label":           m.Label,
		}, map[string]interface{}{
			"device_size":         m.DeviceSize,
			"device_allocated":    m.DeviceAllocated,
			"device_unallocated":  m.DeviceUnallocated,
			"device_missing":      m.DeviceMissing,
			"used":                m.Used,
			"free_estimated":      m.FreeEstimated,
			"free_statfs":         m.FreeStatfs,
			"data_ratio":          m.DataRatio,
			"metadata_ratio":      m.MetadataRatio,
			"status":              m.Status,
			"scrub_state":         m.ScrubState,
			"scrub_read_errors":   m.ScrubReadErrors,
			"scrub_csum_errors":   m.ScrubCsumErrors,
			"scrub_verify_errors": m.ScrubVerifyErrors,
			"scrub_super_errors":  m.ScrubSuperErrors,
		}
}

func NewBtrfsMetricsFromInfluxDB(attrs map[string]interface{}) (*BtrfsMetrics, error) {
	return &BtrfsMetrics{
		Date:              attrs["_time"].(time.Time),
		FilesystemUUID:    attrs["filesystem_uuid"].(string),
		HostID:            influxString(attrs, "host_id"),
		Label:             influxString(attrs, "label"),
		DeviceSize:        influxInt64(attrs, "device_size"),
		DeviceAllocated:   influxInt64(attrs, "device_allocated"),
		DeviceUnallocated: influxInt64(attrs, "device_unallocated"),
		DeviceMissing:     influxInt64(attrs, "device_missing"),
		Used:              influxInt64(attrs, "used"),
		FreeEstimated:     influxInt64(attrs, "free_estimated"),
		FreeStatfs:        influxInt64(attrs, "free_statfs"),
		DataRatio:         influxFloat64(attrs, "data_ratio"),
		MetadataRatio:     influxFloat64(attrs, "metadata_ratio"),
		Status:            influxString(attrs, "status"),
		ScrubState:        influxString(attrs, "scrub_state"),
		ScrubReadErrors:   influxInt64(attrs, "scrub_read_errors"),
		ScrubCsumErrors:   influxInt64(attrs, "scrub_csum_errors"),
		ScrubVerifyErrors: influxInt64(attrs, "scrub_verify_errors"),
		ScrubSuperErrors:  influxInt64(attrs, "scrub_super_errors"),
	}, nil
}
