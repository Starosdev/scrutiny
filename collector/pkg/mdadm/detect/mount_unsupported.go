//go:build !linux
// +build !linux

package detect

import "fmt"

// getMountUsage uses device IDs (Major:Minor) to reliably connect the RAID device 
// (or its partitions) to a mount point in the container.
func (d *Detect) getMountUsage(devicePath string) (int64, error) {
	return 0, fmt.Errorf("mount usage detection is only supported on Linux")
}
