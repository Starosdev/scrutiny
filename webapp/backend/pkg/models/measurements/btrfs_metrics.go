package measurements

import "time"

type BtrfsMetrics struct {
	Date           time.Time `json:"date"`
	FilesystemUUID string    `json:"filesystem_uuid"`
	HostID         string    `json:"host_id"`
	Label          string    `json:"label"`

	DeviceSize        int64   `json:"device_size"`
	DeviceAllocated   int64   `json:"device_allocated"`
	DeviceUnallocated int64   `json:"device_unallocated"`
	DeviceMissing     int64   `json:"device_missing"`
	Used              int64   `json:"used"`
	FreeEstimated     int64   `json:"free_estimated"`
	FreeStatfs        int64   `json:"free_statfs"`
	DataRatio         float64 `json:"data_ratio"`
	MetadataRatio     float64 `json:"metadata_ratio"`
	Status            string  `json:"status"`
	ScrubState        string  `json:"scrub_state"`
	ScrubReadErrors   int64   `json:"scrub_read_errors"`
	ScrubCsumErrors   int64   `json:"scrub_csum_errors"`
	ScrubVerifyErrors int64   `json:"scrub_verify_errors"`
	ScrubSuperErrors  int64   `json:"scrub_super_errors"`
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
	m := BtrfsMetrics{
		Date:           attrs["_time"].(time.Time),
		FilesystemUUID: attrs["filesystem_uuid"].(string),
	}
	if val, ok := attrs["host_id"]; ok && val != nil {
		m.HostID = val.(string)
	}
	if val, ok := attrs["label"]; ok && val != nil {
		m.Label = val.(string)
	}
	if val, ok := attrs["device_size"]; ok && val != nil {
		m.DeviceSize = val.(int64)
	}
	if val, ok := attrs["device_allocated"]; ok && val != nil {
		m.DeviceAllocated = val.(int64)
	}
	if val, ok := attrs["device_unallocated"]; ok && val != nil {
		m.DeviceUnallocated = val.(int64)
	}
	if val, ok := attrs["device_missing"]; ok && val != nil {
		m.DeviceMissing = val.(int64)
	}
	if val, ok := attrs["used"]; ok && val != nil {
		m.Used = val.(int64)
	}
	if val, ok := attrs["free_estimated"]; ok && val != nil {
		m.FreeEstimated = val.(int64)
	}
	if val, ok := attrs["free_statfs"]; ok && val != nil {
		m.FreeStatfs = val.(int64)
	}
	if val, ok := attrs["data_ratio"]; ok && val != nil {
		m.DataRatio = val.(float64)
	}
	if val, ok := attrs["metadata_ratio"]; ok && val != nil {
		m.MetadataRatio = val.(float64)
	}
	if val, ok := attrs["status"]; ok && val != nil {
		m.Status = val.(string)
	}
	if val, ok := attrs["scrub_state"]; ok && val != nil {
		m.ScrubState = val.(string)
	}
	if val, ok := attrs["scrub_read_errors"]; ok && val != nil {
		m.ScrubReadErrors = val.(int64)
	}
	if val, ok := attrs["scrub_csum_errors"]; ok && val != nil {
		m.ScrubCsumErrors = val.(int64)
	}
	if val, ok := attrs["scrub_verify_errors"]; ok && val != nil {
		m.ScrubVerifyErrors = val.(int64)
	}
	if val, ok := attrs["scrub_super_errors"]; ok && val != nil {
		m.ScrubSuperErrors = val.(int64)
	}
	return &m, nil
}
