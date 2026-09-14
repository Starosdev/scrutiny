package detect

import "github.com/analogj/scrutiny/collector/pkg/models"

// SetBlockWWNFallback replaces the platform block device WWN lookup, so tests do not
// depend on which disks the host running them happens to have.
func (d *Detect) SetBlockWWNFallback(fallback func(*models.Device)) {
	d.blockWWNFallback = fallback
}
