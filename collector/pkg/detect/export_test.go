package detect

import "github.com/analogj/scrutiny/collector/pkg/models"

// SetBlockWWNFallback replaces the platform block device WWN lookup, so tests do not
// depend on which disks the host running them happens to have.
func (d *Detect) SetBlockWWNFallback(fallback func(*models.Device)) {
	d.blockWWNFallback = fallback
}

// InflateDevices exposes the post-scan identification, exclusion and filtering step.
func (d *Detect) InflateDevices(devices []models.Device) []models.Device {
	return d.inflateDevices(devices, nil)
}
