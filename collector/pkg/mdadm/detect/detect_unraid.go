package detect

// Unraid does not use Linux MD RAID. Its own md driver writes /proc/mdstat as
// key=value metadata instead of the "mdX : active raidN ..." lines parseMdstat
// expects, and Unraid ships no mdadm binary, so the "mdadm --detail" path in
// detect.go cannot run there either. An Unraid array is also a single
// parity-protected set rather than a collection of independent arrays, so the
// whole host maps onto one MDADMArray.

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/analogj/scrutiny/collector/pkg/mdadm/models"
)

const (
	unraidArrayName    = "unraid-array"
	unraidUUIDPrefix   = "unraid:"
	unraidStateKey     = "mdState"
	unraidStateStarted = "STARTED"

	// rdevStatus values. Unraid reports every one of its 30 array slots, so
	// most carry a DISK_NP* ("not present") status on a typical host.
	diskStatusOK           = "DISK_OK"
	diskStatusAbsentPrefix = "DISK_NP"
)

// unraidSlot holds the per-slot fields of a single Unraid array slot.
// Field order keeps the pointer-bearing strings first, for govet fieldalignment.
type unraidSlot struct {
	diskName   string // "mdNp1" for a data disk, empty for a parity disk
	deviceName string // backing block device, e.g. "sda"
	status     string // rdevStatus
	sizeBlocks int64  // diskSize, in 1K blocks
	index      int
}

// present reports whether a disk is actually assigned to this slot. Unraid
// marks an empty slot DISK_NP, and an empty slot that once held a disk
// DISK_NP_DSBL, which is why a prefix test is used rather than equality: the
// second parity slot of a single-parity array reports DISK_NP_DSBL and must not
// count as a failure.
func (s unraidSlot) present() bool {
	return s.status != "" && !strings.HasPrefix(s.status, diskStatusAbsentPrefix)
}

// healthy reports whether a present disk is in service. Anything other than
// DISK_OK (DISK_DSBL, DISK_INVALID, DISK_WRONG, ...) is treated as failed.
func (s unraidSlot) healthy() bool { return s.status == diskStatusOK }

// isData distinguishes a data disk from a parity disk. Unraid exports an md
// device for data disks only, so parity slots report an empty diskName.
func (s unraidSlot) isData() bool { return s.diskName != "" }

// isUnraidMdstat reports whether content is Unraid's key/value mdstat rather
// than the standard Linux format.
func isUnraidMdstat(content string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), unraidStateKey+"=") {
			return true
		}
	}
	return false
}

// readMdstat returns the full contents of the first available mdstat file.
func readMdstat() (string, error) {
	file, err := openMdstat()
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read mdstat: %w", err)
	}
	return string(content), nil
}

// detectUnraidArray maps an Unraid mdstat onto a single array. The bool result
// is false when mdstat is missing or in the standard Linux format, which leaves
// the Linux detection path in Start unchanged.
func (d *Detect) detectUnraidArray() (models.MDADMArray, models.MDADMMetrics, bool) {
	content, err := readMdstat()
	if err != nil {
		d.Logger.Debugf("Could not read mdstat for Unraid detection: %v", err)
		return models.MDADMArray{}, models.MDADMMetrics{}, false
	}
	if !isUnraidMdstat(content) {
		return models.MDADMArray{}, models.MDADMMetrics{}, false
	}

	d.Logger.Infoln("Detected Unraid-format /proc/mdstat")
	array, metrics := buildUnraidArray(content)

	if array.UUID == "" {
		array.UUID = d.syntheticArrayID(unraidArrayName)
		d.Logger.Warnf("Using synthetic MDADM identifier for the Unraid array because mdstat did not expose sbLabel")
	}

	return array, metrics, true
}

// buildUnraidArray converts Unraid mdstat contents into one array and its metrics.
func buildUnraidArray(content string) (models.MDADMArray, models.MDADMMetrics) {
	arrayKeys, slots := parseUnraidMdstat(content)

	array := models.MDADMArray{Name: unraidArrayName}
	if label := strings.TrimSpace(arrayKeys["sbLabel"]); label != "" {
		// sbLabel is the array GUID from the Unraid superblock, so it is stable
		// across reboots and disk replacements.
		array.UUID = unraidUUIDPrefix + label
	}

	metrics := models.MDADMMetrics{RawMdstat: unraidRawSummary(content)}

	parityDisks := 0
	for _, slot := range slots {
		if !slot.present() {
			continue
		}

		if slot.deviceName != "" {
			array.Devices = append(array.Devices, "/dev/"+slot.deviceName)
		}

		if slot.isData() {
			metrics.ArraySize += slot.sizeBlocks * 1024
		} else {
			parityDisks++
		}

		if slot.healthy() {
			metrics.ActiveDevices++
		} else {
			metrics.FailedDevices++
		}
	}

	// Unraid keeps no in-array spares, so every non-failed member is active.
	metrics.WorkingDevices = metrics.ActiveDevices
	array.Level = unraidLevel(parityDisks)
	metrics.SyncProgress = unraidSyncProgress(arrayKeys)
	metrics.State = unraidState(arrayKeys, metrics.FailedDevices)

	// UsedBytes is deliberately left at zero. Every Unraid data disk carries its
	// own filesystem, so there is no single mount point to stat for the array.

	return array, metrics
}

// parseUnraidMdstat splits Unraid's mdstat into array-level keys and per-slot
// records. Slot keys carry a ".<index>" suffix, such as "rdevStatus.3".
func parseUnraidMdstat(content string) (map[string]string, []unraidSlot) {
	arrayKeys := map[string]string{}
	slotsByIndex := map[int]*unraidSlot{}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		key, value, found := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !found {
			continue
		}

		name, indexStr, hasIndex := strings.Cut(key, ".")
		if !hasIndex {
			arrayKeys[name] = value
			continue
		}

		index, err := strconv.Atoi(indexStr)
		if err != nil {
			continue
		}

		slot, ok := slotsByIndex[index]
		if !ok {
			slot = &unraidSlot{index: index}
			slotsByIndex[index] = slot
		}

		switch name {
		case "diskName":
			slot.diskName = value
		case "rdevName":
			slot.deviceName = value
		case "rdevStatus":
			slot.status = value
		case "diskSize":
			slot.sizeBlocks, _ = strconv.ParseInt(value, 10, 64)
		}
	}

	slots := make([]unraidSlot, 0, len(slotsByIndex))
	for _, slot := range slotsByIndex {
		slots = append(slots, *slot)
	}
	// Sort by slot index so device order matches the Unraid disk assignment.
	sort.Slice(slots, func(i, j int) bool { return slots[i].index < slots[j].index })

	return arrayKeys, slots
}

// unraidLevel describes the array's redundancy. Unraid has no RAID level, so
// the parity disk count is the equivalent piece of information.
func unraidLevel(parityDisks int) string {
	if parityDisks <= 0 {
		return "unraid-noparity"
	}
	return fmt.Sprintf("unraid-parity%d", parityDisks)
}

// unraidSyncProgress converts Unraid's resync position into a percentage, the
// same way Unraid's own webgui does: mdResyncPos over mdResyncSize, both in 1K
// blocks. mdResyncSize is always populated with the parity disk size, so
// mdResyncPos is what indicates an operation.
func unraidSyncProgress(arrayKeys map[string]string) float64 {
	position := unraidInt(arrayKeys, "mdResyncPos")
	if position <= 0 {
		return 0
	}

	total := unraidInt(arrayKeys, "mdResyncSize")
	if total <= 0 {
		return 0
	}

	progress := float64(position) / float64(total) * 100
	if progress > 100 {
		return 100
	}
	return progress
}

func unraidInt(arrayKeys map[string]string, key string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(arrayKeys[key]), 10, 64)
	return value
}

// unraidState renders Unraid's array state using the vocabulary the rest of
// Scrutiny already understands ("active", "degraded", "inactive", "checking",
// "resync", "recovery"), so no backend or frontend mapping is needed.
func unraidState(arrayKeys map[string]string, failedDevices int) string {
	state := strings.TrimSpace(arrayKeys[unraidStateKey])
	if !strings.EqualFold(state, unraidStateStarted) {
		if state == "" {
			state = "unknown"
		}
		return "inactive (" + strings.ToLower(state) + ")"
	}

	parts := []string{"active"}
	if failedDevices > 0 {
		parts = append(parts, "degraded")
	}
	if action := unraidSyncAction(arrayKeys); action != "" {
		parts = append(parts, action)
		if unraidSyncPaused(arrayKeys) {
			parts = append(parts, "paused")
		}
	}

	return strings.Join(parts, ", ")
}

// unraidSyncAction names the parity operation in progress, or "" when idle.
// mdResyncAction keeps the last operation's name after it finishes, so
// mdResyncPos decides whether one is actually in progress.
func unraidSyncAction(arrayKeys map[string]string) string {
	if unraidInt(arrayKeys, "mdResyncPos") <= 0 {
		return ""
	}

	action := strings.ToLower(arrayKeys["mdResyncAction"])
	switch {
	case strings.Contains(action, "recon"):
		return "recovery"
	case strings.Contains(action, "clear"):
		return "resync"
	case strings.Contains(action, "check"):
		return "checking"
	default:
		return "resync"
	}
}

// unraidSyncPaused reports whether an in-progress operation is paused. Unraid
// zeroes mdResync while paused but keeps mdResyncPos, which is how its own
// Array Operation page decides between a Pause and a Resume button.
func unraidSyncPaused(arrayKeys map[string]string) bool {
	return unraidInt(arrayKeys, "mdResyncPos") > 0 && unraidInt(arrayKeys, "mdResync") <= 0
}

// unraidRawSummary keeps the array-level keys and drops the per-slot ones.
// Unraid reports all 30 slots, which makes mdstat roughly 450 lines, and
// raw_mdstat is stored on every metrics point. The per-slot data is already
// exposed through the array's device list and the device counters.
func unraidRawSummary(content string) string {
	var kept []string

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		key, _, found := strings.Cut(line, "=")
		if !found || strings.Contains(key, ".") {
			continue
		}
		kept = append(kept, line)
	}

	return strings.Join(kept, "\n")
}
