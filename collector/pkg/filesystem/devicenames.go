package filesystem

import "strings"

// translateDeviceNames rewrites kernel block device names into the names
// Scrutiny registers drives under, dropping any that cannot be translated.
//
// The sysfs walk in collector/pkg/blockdev answers a kernel question: which
// block devices lie under this mount. Attribution needs a different answer,
// which drive the UI shows, and the two disagree in a way sysfs cannot settle:
//
//   - An NVMe filesystem resolves to a namespace, nvme0n1, while the SMART
//     collector registers the controller, nvme0. Several namespaces on one
//     controller are still one physical drive (#1057).
//   - An Unraid array device, md1p1, has no holder link to the disk holding it,
//     so the pairing comes from Unraid's mdstat instead (#1058).
//
// A name that survives translation is still only a claim: the server attributes
// it to a drive only when a registered device carries that name.
func translateDeviceNames(names []string, unraidArrayDisks map[string]string) []string {
	if len(names) == 0 {
		return nil
	}

	translated := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if disk, isArrayDevice := unraidArrayDisks[name]; isArrayDevice {
			name = disk
		}
		name = nvmeControllerName(name)
		if name == "" {
			continue
		}
		// Two array devices can share a disk, and two namespaces share a
		// controller, so translation can collapse distinct names into one.
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		translated = append(translated, name)
	}

	if len(translated) == 0 {
		return nil
	}
	return translated
}

// nvmeControllerName reduces an NVMe namespace to its controller, which is the
// name Scrutiny registers the drive under. Any other name is returned unchanged.
//
// Only the exact nvme<digits>n<digits> shape is reduced. A partition such as
// nvme0n1p2 never reaches here, because the walk resolves it to its namespace
// first, and refusing to match it keeps this from mangling a name it does not
// understand.
func nvmeControllerName(name string) string {
	if !strings.HasPrefix(name, "nvme") {
		return name
	}
	controller, namespace, found := strings.Cut(name[len("nvme"):], "n")
	if !found || !allDigits(controller) || !allDigits(namespace) {
		return name
	}
	return "nvme" + controller
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
