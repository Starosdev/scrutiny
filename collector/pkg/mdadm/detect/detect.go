package detect

import (
	"bufio"
	"crypto/sha1" //nolint:gosec
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/mdadm/models"
	"github.com/sirupsen/logrus"
)

// Detect handles MDADM RAID array detection
type Detect struct {
	Logger *logrus.Entry
	Config config.Interface
}

// mdstatPaths lists the paths to check for mdstat, in priority order.
// /host/proc/mdstat is used when running in Docker (bind-mounted from host).
// /proc/mdstat is the native path on bare metal.
var mdstatPaths = []string{"/host/proc/mdstat", "/proc/mdstat"}

// openMdstat opens the first available mdstat file.
func openMdstat() (*os.File, error) {
	for _, path := range mdstatPaths {
		if f, err := os.Open(path); err == nil {
			return f, nil
		}
	}
	return nil, fmt.Errorf("mdstat not found at any of: %v", mdstatPaths)
}

// Start detects all MDADM arrays on the system
func (d *Detect) Start() ([]models.MDADMArray, []models.MDADMMetrics, error) {
	// 1. Discover arrays from /proc/mdstat
	arrayNames, err := d.parseMdstat()
	if err != nil {
		return nil, nil, err
	}

	if len(arrayNames) == 0 {
		d.Logger.Infoln("No MDADM arrays found in /proc/mdstat")
		return nil, nil, nil
	}

	var arrays []models.MDADMArray
	var metrics []models.MDADMMetrics

	// 2. Get details for each array
	for _, name := range arrayNames {
		array, metric, err := d.getArrayDetail(name)
		if err != nil {
			d.Logger.Warnf("Failed to get details for array %s: %v", name, err)
			continue
		}
		arrays = append(arrays, array)
		metrics = append(metrics, metric)
	}

	return arrays, metrics, nil
}

// parseMdstat parses /proc/mdstat to discover active arrays
func (d *Detect) parseMdstat() ([]string, error) {
	file, err := openMdstat()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open mdstat: %w", err)
	}
	defer file.Close()

	var arrays []string
	scanner := bufio.NewScanner(file)
	// Example line: "md0 : active raid1 sdb[1] sda[0]"
	mdPattern := regexp.MustCompile(`^(md\d+)\s*:\s*active`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := mdPattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			arrays = append(arrays, matches[1])
		}
	}

	return arrays, scanner.Err()
}

// getArrayDetail runs mdadm --detail and parses its output
func (d *Detect) getArrayDetail(name string) (models.MDADMArray, models.MDADMMetrics, error) {
	devicePath := fmt.Sprintf("/dev/%s", name)

	var cmd *exec.Cmd
	if os.Getuid() == 0 {
		cmd = exec.Command("mdadm", "--detail", devicePath)
	} else {
		cmd = exec.Command("sudo", "mdadm", "--detail", devicePath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return models.MDADMArray{}, models.MDADMMetrics{}, fmt.Errorf("failed to run mdadm --detail %s: %w", devicePath, err)
	}

	array, metrics, err := d.parseMdadmOutput(name, string(output))
	if err == nil && strings.TrimSpace(array.UUID) == "" {
		exportUUID, exportErr := d.getArrayUUIDFromExport(devicePath)
		if exportErr != nil {
			d.Logger.Debugf("Could not determine UUID for %s via mdadm --detail --export: %v", devicePath, exportErr)
		} else {
			array.UUID = exportUUID
		}
	}
	if err == nil && strings.TrimSpace(array.UUID) == "" {
		array.UUID = d.syntheticArrayID(name)
		d.Logger.Warnf("Using synthetic MDADM identifier for %s because mdadm did not expose a UUID", devicePath)
	}
	if err == nil {
		rawMdstat, _ := d.getRawMdstat(name)
		metrics.RawMdstat = rawMdstat

		// Parse sync/check/rebuild/recovery progress from /proc/mdstat if not already set
		// by mdadm --detail. The "check = X%" line only appears in /proc/mdstat.
		if metrics.SyncProgress == 0 && rawMdstat != "" {
			mdstatProgressPattern := regexp.MustCompile(`(?:check|resync|recovery|rebuild)\s*=\s*(\d+(?:\.\d+)?)%`)
			if m := mdstatProgressPattern.FindStringSubmatch(rawMdstat); m != nil {
				metrics.SyncProgress, _ = strconv.ParseFloat(m[1], 64)
			}
		}

		// Get filesystem-level used bytes if the array is mounted in the container.
		usedBytes, statErr := d.getMountUsage(devicePath)
		if statErr != nil {
			d.Logger.Debugf("Could not get mount usage for %s (may not be mounted in container): %v", devicePath, statErr)
		} else {
			metrics.UsedBytes = usedBytes
		}
	}

	return array, metrics, err
}

func (d *Detect) getArrayUUIDFromExport(devicePath string) (string, error) {
	var cmd *exec.Cmd
	if os.Getuid() == 0 {
		cmd = exec.Command("mdadm", "--detail", "--export", devicePath)
	} else {
		cmd = exec.Command("sudo", "mdadm", "--detail", "--export", devicePath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run mdadm --detail --export %s: %w", devicePath, err)
	}

	uuid := parseMdadmExportUUID(string(output))
	if uuid == "" {
		return "", fmt.Errorf("mdadm export output did not include MD_UUID")
	}
	return uuid, nil
}

// getRawMdstat extracts the specific multi-line block for an array from /proc/mdstat
func (d *Detect) getRawMdstat(name string) (string, error) {
	file, err := openMdstat()
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var block []string
	inBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, name+" :") {
			inBlock = true
		} else if inBlock && (!strings.HasPrefix(line, " ") && len(strings.TrimSpace(line)) > 0) {
			// A new array block starts with a non-space character (like md1 : ...)
			// or Personalities line.
			break
		}

		if inBlock {
			block = append(block, line)
		}
	}

	return strings.Join(block, "\n"), scanner.Err()
}

// parseMdadmOutput extracts array metadata and metrics from mdadm output
func (d *Detect) parseMdadmOutput(name string, output string) (models.MDADMArray, models.MDADMMetrics, error) {
	array := models.MDADMArray{
		Name: name,
	}
	metrics := models.MDADMMetrics{}

	scanner := bufio.NewScanner(strings.NewReader(output))
	patterns := newMDADMOutputPatterns()

	// Device list starts after the header
	inDeviceList := false
	devicePattern := regexp.MustCompile(`\s+\d+\s+\d+\s+\d+\s+\d+\s+.+\s+(/dev/\S+)`)

	for scanner.Scan() {
		line := scanner.Text()
		updateMDADMDetailLine(line, &patterns, &array, &metrics)

		if strings.Contains(line, "Number   Major   Minor   RaidDevice State") {
			inDeviceList = true
			continue
		}

		if inDeviceList {
			if m := devicePattern.FindStringSubmatch(line); m != nil {
				array.Devices = append(array.Devices, m[1])
			}
		}
	}

	return array, metrics, nil
}

type mdadmOutputPatterns struct {
	raidLevel *regexp.Regexp
	uuid      *regexp.Regexp
	state     *regexp.Regexp
	active    *regexp.Regexp
	working   *regexp.Regexp
	failed    *regexp.Regexp
	spare     *regexp.Regexp
	arraySize *regexp.Regexp
	progress  []*regexp.Regexp
}

func newMDADMOutputPatterns() mdadmOutputPatterns {
	return mdadmOutputPatterns{
		raidLevel: regexp.MustCompile(`Raid Level\s*:\s*(.+)`),
		uuid:      regexp.MustCompile(`UUID\s*:\s*(.+)`),
		state:     regexp.MustCompile(`State\s*:\s*(.+)`),
		active:    regexp.MustCompile(`Active Devices\s*:\s*(\d+)`),
		working:   regexp.MustCompile(`Working Devices\s*:\s*(\d+)`),
		failed:    regexp.MustCompile(`Failed Devices\s*:\s*(\d+)`),
		spare:     regexp.MustCompile(`Spare Devices\s*:\s*(\d+)`),
		progress: []*regexp.Regexp{
			regexp.MustCompile(`Rebuild Status\s*:\s*(\d+(?:\.\d+)?)%`),
			regexp.MustCompile(`Resync Status\s*:\s*(\d+(?:\.\d+)?)%`),
			regexp.MustCompile(`Recovery Status\s*:\s*(\d+(?:\.\d+)?)%`),
			regexp.MustCompile(`check\s*=\s*(\d+(?:\.\d+)?)%`),
		},
		arraySize: regexp.MustCompile(`Array Size\s*:\s*(\d+)`),
	}
}

func updateMDADMDetailLine(line string, patterns *mdadmOutputPatterns, array *models.MDADMArray, metrics *models.MDADMMetrics) {
	switch {
	case matchStringField(line, patterns.raidLevel, &array.Level):
	case matchStringField(line, patterns.uuid, &array.UUID):
	case matchStringField(line, patterns.state, &metrics.State):
	case matchIntField(line, patterns.active, &metrics.ActiveDevices):
	case matchIntField(line, patterns.working, &metrics.WorkingDevices):
	case matchIntField(line, patterns.failed, &metrics.FailedDevices):
	case matchIntField(line, patterns.spare, &metrics.SpareDevices):
	case matchProgress(line, patterns.progress, &metrics.SyncProgress):
	case matchArraySize(line, patterns.arraySize, &metrics.ArraySize):
	}
}

func matchStringField(line string, pattern *regexp.Regexp, target *string) bool {
	if m := pattern.FindStringSubmatch(line); m != nil {
		*target = strings.TrimSpace(m[1])
		return true
	}
	return false
}

func matchIntField(line string, pattern *regexp.Regexp, target *int) bool {
	if m := pattern.FindStringSubmatch(line); m != nil {
		*target, _ = strconv.Atoi(m[1])
		return true
	}
	return false
}

func matchProgress(line string, patterns []*regexp.Regexp, target *float64) bool {
	for _, pattern := range patterns {
		if m := pattern.FindStringSubmatch(line); m != nil {
			*target, _ = strconv.ParseFloat(m[1], 64)
			return true
		}
	}
	return false
}

func matchArraySize(line string, pattern *regexp.Regexp, target *int64) bool {
	if m := pattern.FindStringSubmatch(line); m != nil {
		kb, _ := strconv.ParseInt(m[1], 10, 64)
		*target = kb * 1024
		return true
	}
	return false
}

func parseMdadmExportUUID(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "MD_UUID=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "MD_UUID="))
		}
	}
	return ""
}

func (d *Detect) syntheticArrayID(name string) string {
	hostID := ""
	if d.Config != nil {
		hostID = strings.TrimSpace(d.Config.GetString("host.id"))
	}
	if hostID == "" {
		hostID = "unknown-host"
	}

	sum := sha1.Sum([]byte(hostID + "\n" + name)) //nolint:gosec // SHA1 used for deterministic ID generation, not cryptographic security
	return "synthetic:" + hex.EncodeToString(sum[:])
}
