package detect

import "strings"

// UnraidArrayDeviceMap maps each Unraid array device name to the physical disk
// behind it, for example "md1p1" to "sdf".
//
// Unraid's array devices are neither Linux md arrays nor partitions: sysfs
// registers md1p1 as a top-level virtual block device with no partition file and
// an empty slaves directory, so the kernel's holder tree says nothing about
// which disk holds it (#1058). Unraid records the pairing in its own mdstat, as
// diskName.N against rdevName.N, which is the only place it exists.
//
// The map is empty rather than an error for every non-Unraid host, an
// unreadable mdstat, and a stopped array. Unraid publishes diskName only while
// the array is started, which is why the slot parser deliberately ignores that
// key: assignment and parity classification must not depend on a value that
// disappears. A name mapping may, because an absent diskName yields no entry and
// the caller then attributes nothing instead of attributing something wrong.
func UnraidArrayDeviceMap() map[string]string {
	content, err := readMdstat()
	if err != nil || !isUnraidMdstat(content) {
		return nil
	}
	return unraidArrayDeviceMap(content)
}

// unraidArrayDeviceMap pairs diskName with rdevName by slot index.
//
// Both keys are read in one pass rather than through parseUnraidMdstat, so the
// unraidSlot contract that excludes diskName stays as it is.
func unraidArrayDeviceMap(content string) map[string]string {
	arrayDevices := map[string]string{}
	physicalDisks := map[string]string{}

	for line := range unraidLines(content) {
		key, value, found := strings.Cut(line, "=")
		if !found || value == "" {
			continue
		}
		name, index, hasIndex := strings.Cut(key, ".")
		if !hasIndex || index == "" {
			continue
		}

		switch name {
		case "diskName":
			arrayDevices[index] = value
		case "rdevName":
			physicalDisks[index] = value
		}
	}

	deviceMap := make(map[string]string, len(arrayDevices))
	for index, arrayDevice := range arrayDevices {
		// A slot with no live backing device, which is how Unraid reports a
		// disabled or missing disk, maps to nothing.
		if disk := physicalDisks[index]; disk != "" {
			deviceMap[arrayDevice] = disk
		}
	}
	return deviceMap
}
