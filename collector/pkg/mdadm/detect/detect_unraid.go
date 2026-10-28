package detect

// Unraid does not use Linux MD RAID. Its own md driver writes /proc/mdstat as
// key=value metadata instead of the "mdX : active raidN ..." lines parseMdstat
// expects, and Unraid ships no mdadm binary, so the "mdadm --detail" path in
// detect.go cannot run there either. An Unraid array is also a single
// parity-protected set rather than a collection of independent arrays, so the
// whole host maps onto one MDADMArray.

import (
	"crypto/sha1" //nolint:gosec
	"encoding/hex"
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

	// Unraid's slot layout is fixed: slot 0 is parity, slot 29 is the optional
	// second parity disk, and the slots between them are data disks.
	unraidParitySlot  = 0
	unraidParity2Slot = 29
)

// unraidSlot holds the per-slot fields of a single Unraid array slot.
// Field order keeps the pointer-bearing strings first, for govet fieldalignment.
type unraidSlot struct {
	diskName   string // "mdNp1" while the array is started, else empty
	deviceName string // rdevName, the live backing block device, e.g. "sda"
	diskID     string // diskId, the serial recorded in the superblock
	status     string // rdevStatus
	sizeBlocks int64  // diskSize, in 1K blocks
	index      int
}

// assigned reports whether the superblock has a disk in this slot. Unraid keeps
// the recorded identity even when the disk itself is gone, which is what
// separates a configured-but-absent disk from an empty slot. An unused slot
// reports an empty diskId and a zero diskSize.
func (s unraidSlot) assigned() bool {
	return s.diskID != "" || s.sizeBlocks > 0
}

// attached reports whether the disk is physically there. Unraid prefixes every
// absent status with DISK_NP, and its own webgui tests presence the same way
// (ArrayOperation.page uses strpos(status, "DISK_NP") === 0).
func (s unraidSlot) attached() bool {
	return s.status != "" && !strings.HasPrefix(s.status, diskStatusAbsentPrefix)
}

// missing reports an assigned disk that is not attached. Unraid calls this
// DISK_NP_MISSING and renders it as "Missing"; it is a failure, not an empty
// slot, and it must never be skipped the way an unassigned slot is. An empty
// second parity slot reports DISK_NP_DSBL with no recorded identity, so it is
// absent rather than missing.
func (s unraidSlot) missing() bool {
	return s.assigned() && !s.attached()
}

// healthy reports whether an attached disk is in service. Anything other than
// DISK_OK (DISK_DSBL, DISK_DSBL_NEW, DISK_INVALID, DISK_WRONG, DISK_TOO_SMALL,
// DISK_NEW) is treated as failed.
func (s unraidSlot) healthy() bool { return s.status == diskStatusOK }

// isParity reports whether this slot holds a parity disk. The slot index is the
// authority rather than diskName, because Unraid exports the md device for a
// data disk only while the array is started, so diskName is empty for every
// slot on a stopped array.
func (s unraidSlot) isParity() bool {
	return s.index == unraidParitySlot || s.index == unraidParity2Slot
}

// unraidLines iterates the trimmed lines of an mdstat already held in memory.
// bufio.Scanner is deliberately avoided: it caps a token at 64KiB and reports
// that through an error the callers here have no way to return, so an
// unexpectedly long line would truncate the parse silently.
func unraidLines(content string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for _, line := range strings.Split(content, "\n") {
			if !yield(strings.TrimSpace(line)) {
				return
			}
		}
	}
}

// isUnraidMdstat reports whether content is Unraid's key/value mdstat rather
// than the standard Linux format.
func isUnraidMdstat(content string) bool {
	for line := range unraidLines(content) {
		if strings.HasPrefix(line, unraidStateKey+"=") {
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
		// across reboots and disk replacements. It is derived from the flash
		// drive GUID, which is the Unraid license identifier, and the UUID is
		// rendered in the web UI and included in notification bodies, so the
		// digest is stored rather than the value itself.
		sum := sha1.Sum([]byte(label)) //nolint:gosec // digest is for a stable non-identifying key, not security
		array.UUID = unraidUUIDPrefix + hex.EncodeToString(sum[:])
	}

	metrics := models.MDADMMetrics{RawMdstat: unraidRawSummary(content)}

	parityDisks := 0
	missingDisks := 0
	for _, slot := range slots {
		if !slot.assigned() && !slot.attached() {
			// An unused slot. Unraid reports all 30 of them on every host.
			continue
		}

		if slot.deviceName != "" {
			array.Devices = append(array.Devices, "/dev/"+slot.deviceName)
		}

		if slot.isParity() {
			parityDisks++
		} else {
			metrics.ArraySize += slot.sizeBlocks * 1024
		}

		switch {
		case slot.missing():
			missingDisks++
			metrics.FailedDevices++
		case slot.healthy():
			metrics.ActiveDevices++
		default:
			metrics.FailedDevices++
		}
	}

	// Cross-check against Unraid's own counter. If a future Unraid release stops
	// recording the identity of a missing disk, the per-slot scan above would
	// see an empty slot and report the array healthy, which is the one outcome
	// this collector must never produce.
	if reported := int(unraidInt(arrayKeys, "mdNumMissing")); reported > missingDisks {
		metrics.FailedDevices += reported - missingDisks
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

	for line := range unraidLines(content) {
		key, value, found := strings.Cut(line, "=")
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
		case "diskId":
			slot.diskID = value
		case "rdevStatus":
			slot.status = value
		case "diskSize":
			// A negative or unparseable size contributes nothing rather than
			// shrinking the reported array capacity.
			if size, err := strconv.ParseInt(value, 10, 64); err == nil && size > 0 {
				slot.sizeBlocks = size
			}
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
	// "nocorrect" is a parity check that reports errors without writing
	// corrections, so it is a check rather than a resync.
	case strings.Contains(action, "check"), strings.Contains(action, "nocorrect"):
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

	for line := range unraidLines(content) {
		key, _, found := strings.Cut(line, "=")
		if !found || strings.Contains(key, ".") {
			continue
		}
		// sbLabel is withheld: it is the Unraid license identifier, and this
		// block is rendered verbatim on the array details page.
		if key == "sbLabel" {
			continue
		}
		kept = append(kept, line)
	}

	return strings.Join(kept, "\n")
}
