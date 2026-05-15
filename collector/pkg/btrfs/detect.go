package btrfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/sirupsen/logrus"
)

type Detect struct {
	Logger         *logrus.Entry
	Config         config.Interface
	ReadMountsFile func(string) ([]byte, error)
	LookPath       func(string) (string, error)
	RunCommand     func(name string, args ...string) ([]byte, error)
}

func (d *Detect) Start() ([]Filesystem, error) {
	if d.Logger == nil {
		d.Logger = logrus.NewEntry(logrus.New())
	}
	if d.ReadMountsFile == nil {
		d.ReadMountsFile = os.ReadFile
	}
	if d.LookPath == nil {
		d.LookPath = exec.LookPath
	}
	if d.RunCommand == nil {
		d.RunCommand = func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		}
	}

	btrfsPath, err := d.LookPath("btrfs")
	if err != nil {
		d.Logger.Warnf("btrfs command not found: %v", err)
		return nil, fmt.Errorf("btrfs command not found: %w", err)
	}
	d.Logger.Debugf("Found btrfs at: %s", btrfsPath)

	mountPoints, err := d.listBtrfsMountPoints()
	if err != nil {
		return nil, err
	}
	if len(mountPoints) == 0 {
		return nil, nil
	}

	filesystems := make([]Filesystem, 0, len(mountPoints))
	seen := make(map[string]struct{})
	for _, mountPoint := range mountPoints {
		fs, err := d.inspectFilesystem(mountPoint)
		if err != nil {
			d.Logger.Warnf("Failed to inspect Btrfs filesystem at %s: %v", mountPoint, err)
			continue
		}
		if fs.UUID == "" {
			d.Logger.Warnf("Skipping Btrfs filesystem at %s with empty UUID", mountPoint)
			continue
		}
		if _, ok := seen[fs.UUID]; ok {
			continue
		}
		seen[fs.UUID] = struct{}{}
		filesystems = append(filesystems, fs)
	}

	sort.Slice(filesystems, func(i, j int) bool {
		return filesystems[i].MountPoint < filesystems[j].MountPoint
	})

	return filesystems, nil
}

func (d *Detect) listBtrfsMountPoints() ([]string, error) {
	data, err := d.ReadMountsFile("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/mounts: %w", err)
	}

	seenMounts := make(map[string]struct{})
	seenSources := make(map[string]struct{})
	mountPoints := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[2] != "btrfs" {
			continue
		}
		source := unescapeMountField(fields[0])
		mountPoint := unescapeMountField(fields[1])
		if _, ok := seenMounts[mountPoint]; ok {
			continue
		}
		// Multiple mounted subvolumes commonly share the same source entry.
		if _, ok := seenSources[source]; ok {
			continue
		}
		seenMounts[mountPoint] = struct{}{}
		seenSources[source] = struct{}{}
		mountPoints = append(mountPoints, mountPoint)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan /proc/mounts: %w", err)
	}

	sort.Strings(mountPoints)
	return mountPoints, nil
}

func (d *Detect) inspectFilesystem(mountPoint string) (Filesystem, error) {
	fs := Filesystem{
		MountPoint: mountPoint,
		Status:     FilesystemStatusOnline,
		ScrubState: ScrubStateUnknown,
	}
	if d.Config != nil && d.Config.IsSet("host.id") {
		fs.HostID = d.Config.GetString("host.id")
	}

	showOutput, err := d.RunCommand("btrfs", "filesystem", "show", "--raw", mountPoint)
	if err != nil {
		return fs, fmt.Errorf("btrfs filesystem show failed: %w", err)
	}
	if err := parseFilesystemShow(&fs, string(showOutput)); err != nil {
		return fs, err
	}

	usageOutput, err := d.RunCommand("btrfs", "filesystem", "usage", "--raw", mountPoint)
	if err != nil {
		return fs, fmt.Errorf("btrfs filesystem usage failed: %w", err)
	}
	if err := parseFilesystemUsage(&fs, string(usageOutput)); err != nil {
		return fs, err
	}

	deviceStatsOutput, err := d.RunCommand("btrfs", "device", "stats", mountPoint)
	if err != nil {
		d.Logger.Warnf("btrfs device stats failed for %s: %v", mountPoint, err)
	} else {
		parseDeviceStats(&fs, string(deviceStatsOutput))
	}

	scrubOutput, err := d.RunCommand("btrfs", "scrub", "status", "--raw", mountPoint)
	if err != nil {
		d.Logger.Warnf("btrfs scrub status failed for %s: %v", mountPoint, err)
	} else {
		parseScrubStatus(&fs, string(scrubOutput))
	}

	if hasMissingDevice(fs.Devices) || fs.DeviceMissing > 0 {
		fs.Status = FilesystemStatusDegraded
	}

	return fs, nil
}

func parseFilesystemShow(fs *Filesystem, output string) error {
	scanner := bufio.NewScanner(strings.NewReader(output))
	headerPattern := regexp.MustCompile(`(?i)^Label:\s*(?:(?:'([^']*)')|none)\s+uuid:\s*([a-f0-9-]+)$`)
	devicePattern := regexp.MustCompile(`^\s*devid\s+(\d+)\s+size\s+(\d+)\s+used\s+\d+\s+path\s+(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			fs.Label = matches[1]
			fs.UUID = matches[2]
			continue
		}

		if matches := devicePattern.FindStringSubmatch(line); matches != nil {
			id, _ := strconv.Atoi(matches[1])
			size, _ := strconv.ParseInt(matches[2], 10, 64)
			path := strings.TrimSpace(matches[3])
			missing := path == "missing"
			fs.Devices = append(fs.Devices, Device{
				ID:      id,
				Size:    size,
				Path:    path,
				Missing: missing,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan btrfs filesystem show output: %w", err)
	}
	fs.DeviceCount = len(fs.Devices)
	if fs.UUID == "" {
		return fmt.Errorf("btrfs filesystem show output missing UUID")
	}
	return nil
}

func parseFilesystemUsage(fs *Filesystem, output string) error {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentSection string
	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "WARNING:") {
			continue
		}

		switch {
		case strings.HasSuffix(line, ":") && !strings.Contains(line, ","):
			currentSection = strings.TrimSuffix(line, ":")
		case currentSection == "Overall" && strings.Contains(line, ":") && !strings.Contains(line, ","):
			key, value := splitUsageKV(line)
			assignOverallUsage(fs, key, value)
		case strings.Contains(line, ","):
			assignAllocationUsage(fs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan btrfs filesystem usage output: %w", err)
	}
	return nil
}

func splitUsageKV(line string) (string, string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func assignOverallUsage(fs *Filesystem, key, value string) {
	switch key {
	case "Device size":
		fs.DeviceSize = parseLeadingInt(value)
	case "Device allocated":
		fs.DeviceAllocated = parseLeadingInt(value)
	case "Device unallocated":
		fs.DeviceUnallocated = parseLeadingInt(value)
	case "Device missing":
		fs.DeviceMissing = parseLeadingInt(value)
	case "Used":
		fs.Used = parseLeadingInt(value)
	case "Free (estimated)":
		fs.FreeEstimated, fs.FreeMin = parseFreeEstimated(value)
	case "Free (statfs, df)":
		fs.FreeStatfs = parseLeadingInt(value)
	case "Data ratio":
		fs.DataRatio = parseLeadingFloat(value)
	case "Metadata ratio":
		fs.MetadataRatio = parseLeadingFloat(value)
	case "Multiple profiles":
		fs.MultipleProfiles = strings.EqualFold(value, "yes")
	}
}

func assignAllocationUsage(fs *Filesystem, line string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}
	label := strings.TrimSpace(parts[0])
	body := strings.TrimSpace(parts[1])
	labelParts := strings.SplitN(label, ",", 2)
	if len(labelParts) != 2 {
		return
	}

	section := strings.TrimSpace(labelParts[0])
	profile := strings.TrimSpace(labelParts[1])
	totalMatch := regexp.MustCompile(`total=(\d+)`).FindStringSubmatch(body)
	usedMatch := regexp.MustCompile(`used=(\d+)`).FindStringSubmatch(body)
	total := int64(0)
	used := int64(0)
	if len(totalMatch) == 2 {
		total, _ = strconv.ParseInt(totalMatch[1], 10, 64)
	}
	if len(usedMatch) == 2 {
		used, _ = strconv.ParseInt(usedMatch[1], 10, 64)
	}

	switch section {
	case "Data":
		fs.DataProfile = profile
		fs.DataTotal = total
		fs.DataUsed = used
	case "Metadata":
		fs.MetadataProfile = profile
		fs.MetadataTotal = total
		fs.MetadataUsed = used
	case "System":
		fs.SystemProfile = profile
		fs.SystemTotal = total
		fs.SystemUsed = used
	}
}

func parseDeviceStats(fs *Filesystem, output string) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	statPattern := regexp.MustCompile(`^\[(.+)\]\.(\w+)\s+(\d+)$`)
	for scanner.Scan() {
		matches := statPattern.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if matches == nil {
			continue
		}

		path := matches[1]
		statKey := matches[2]
		value, _ := strconv.ParseInt(matches[3], 10, 64)

		device := findDeviceByPath(fs.Devices, path)
		if device == nil {
			continue
		}
		switch statKey {
		case "write_io_errs":
			device.WriteIOErrors = value
		case "read_io_errs":
			device.ReadIOErrors = value
		case "flush_io_errs":
			device.FlushIOErrors = value
		case "corruption_errs":
			device.CorruptionErrors = value
		case "generation_errs":
			device.GenerationErrors = value
		}
	}
}

func parseScrubStatus(fs *Filesystem, output string) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		key, value := splitUsageKV(line)
		switch key {
		case "UUID":
			if fs.UUID == "" {
				fs.UUID = value
			}
		case "Scrub started":
			if ts, err := parseBtrfsTime(value); err == nil {
				fs.ScrubStartedAt = &ts
			}
		case "Status":
			fs.ScrubState = parseScrubState(value)
		case "Duration":
			fs.ScrubDuration = value
		case "Total to scrub":
			fs.ScrubTotalBytes = parseLeadingInt(value)
		case "Bytes scrubbed":
			fs.ScrubScrubbedBytes = parseLeadingInt(value)
		case "Error summary":
			fs.ScrubErrorSummary = value
			parseScrubErrorSummary(fs, value)
		}
	}
	if fs.ScrubState == ScrubStateFinished && fs.ScrubStartedAt != nil && fs.ScrubDuration != "" {
		if duration, err := parseClockDuration(fs.ScrubDuration); err == nil {
			finished := fs.ScrubStartedAt.Add(duration)
			fs.ScrubFinishedAt = &finished
		}
	}
}

func parseScrubErrorSummary(fs *Filesystem, value string) {
	if strings.EqualFold(value, "no errors found") {
		return
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' '
	})
	for _, part := range parts {
		if !strings.Contains(part, "=") {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		count, _ := strconv.ParseInt(kv[1], 10, 64)
		switch kv[0] {
		case "read":
			fs.ScrubReadErrors = count
		case "csum":
			fs.ScrubCsumErrors = count
		case "verify":
			fs.ScrubVerifyErrors = count
		case "super":
			fs.ScrubSuperErrors = count
		}
	}
}

func parseScrubState(value string) ScrubState {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "running":
		return ScrubStateRunning
	case "finished":
		return ScrubStateFinished
	case "aborted", "cancelled", "canceled":
		return ScrubStateAborted
	case "not running", "idle":
		return ScrubStateIdle
	default:
		return ScrubStateUnknown
	}
}

func findDeviceByPath(devices []Device, path string) *Device {
	for i := range devices {
		if devices[i].Path == path {
			return &devices[i]
		}
	}
	return nil
}

func hasMissingDevice(devices []Device) bool {
	for _, device := range devices {
		if device.Missing {
			return true
		}
	}
	return false
}

func parseLeadingInt(value string) int64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	clean := strings.TrimSuffix(fields[0], ".")
	parsed, _ := strconv.ParseInt(clean, 10, 64)
	return parsed
}

func parseLeadingFloat(value string) float64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	parsed, _ := strconv.ParseFloat(strings.TrimSuffix(fields[0], "."), 64)
	return parsed
}

func parseFreeEstimated(value string) (int64, int64) {
	mainValue := parseLeadingInt(value)
	minPattern := regexp.MustCompile(`min:\s*(\d+)`)
	matches := minPattern.FindStringSubmatch(value)
	if len(matches) != 2 {
		return mainValue, 0
	}
	minValue, _ := strconv.ParseInt(matches[1], 10, 64)
	return mainValue, minValue
}

func parseBtrfsTime(value string) (time.Time, error) {
	layouts := []string{
		time.ANSIC,
		"Mon Jan _2 15:04:05 2006",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", value)
}

func parseClockDuration(value string) (time.Duration, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("unsupported duration format: %s", value)
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second, nil
}

func unescapeMountField(value string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return replacer.Replace(value)
}
