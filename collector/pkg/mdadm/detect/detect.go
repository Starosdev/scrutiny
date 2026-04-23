package detect

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

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
	
	output, err := cmd.Output()
	if err != nil {
		return models.MDADMArray{}, models.MDADMMetrics{}, fmt.Errorf("failed to run mdadm --detail %s: %w", devicePath, err)
	}

	array, metrics, err := d.parseMdadmOutput(name, string(output))
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

		// Get filesystem-level used bytes if the array is mounted.
		usedBytes, statErr := d.getMountUsage(devicePath)
		if statErr != nil {
			d.Logger.Debugf("Could not get mount usage for %s (may not be mounted): %v", devicePath, statErr)
		} else {
			metrics.UsedBytes = usedBytes
		}
	}
	
	return array, metrics, err
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

// mountsPaths lists candidate locations for /proc/mounts, Docker-first.
var mountsPaths = []string{"/host/proc/mounts", "/proc/mounts"}

// getMountUsage finds the mount point for devicePath and returns the number
// of bytes currently used on the filesystem (total - free). Returns an error
// if the device is not mounted or statfs fails.
func (d *Detect) getMountUsage(devicePath string) (int64, error) {
	// Resolve symlinks so /dev/md0 and /dev/md/myarray match the same entry.
	resolved, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		resolved = devicePath // best-effort fallback
	}

	mountPoint := ""
	for _, path := range mountsPaths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}
			dev := fields[0]
			if dev == devicePath || dev == resolved {
				mountPoint = fields[1]
				break
			}
		}
		f.Close()
		if mountPoint != "" {
			break
		}
	}

	if mountPoint == "" {
		return 0, fmt.Errorf("device %s not found in /proc/mounts", devicePath)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &stat); err != nil {
		return 0, fmt.Errorf("statfs(%s): %w", mountPoint, err)
	}

	// Bfree = total free blocks (including reserved root blocks)
	// Bavail = free blocks available to unprivileged users
	// We use Bfree so the "used" figure matches what df reports.
	usedBlocks := stat.Blocks - stat.Bfree
	return int64(usedBlocks) * stat.Bsize, nil
}

// parseMdadmOutput extracts array metadata and metrics from mdadm output
func (d *Detect) parseMdadmOutput(name string, output string) (models.MDADMArray, models.MDADMMetrics, error) {
	array := models.MDADMArray{
		Name: name,
	}
	metrics := models.MDADMMetrics{}

	scanner := bufio.NewScanner(strings.NewReader(output))
	
	// Regex patterns for detail fields
	raidLevelPattern := regexp.MustCompile(`Raid Level\s*:\s*(.+)`)
	uuidPattern := regexp.MustCompile(`UUID\s*:\s*(.+)`)
	statePattern := regexp.MustCompile(`State\s*:\s*(.+)`)
	activePattern := regexp.MustCompile(`Active Devices\s*:\s*(\d+)`)
	workingPattern := regexp.MustCompile(`Working Devices\s*:\s*(\d+)`)
	failedPattern := regexp.MustCompile(`Failed Devices\s*:\s*(\d+)`)
	sparePattern := regexp.MustCompile(`Spare Devices\s*:\s*(\d+)`)
	rebuildPattern := regexp.MustCompile(`Rebuild Status\s*:\s*(\d+(?:\.\d+)?)%`)
	resyncPattern := regexp.MustCompile(`Resync Status\s*:\s*(\d+(?:\.\d+)?)%`)
	recoveryPattern := regexp.MustCompile(`Recovery Status\s*:\s*(\d+(?:\.\d+)?)%`)
	checkPattern := regexp.MustCompile(`check\s*=\s*(\d+(?:\.\d+)?)%`)
	// "Array Size : 209584128 (...)" — value is in KiB
	arraySizePattern := regexp.MustCompile(`Array Size\s*:\s*(\d+)`)

	// Device list starts after the header
	inDeviceList := false
	devicePattern := regexp.MustCompile(`\s+\d+\s+\d+\s+\d+\s+\d+\s+.+\s+(/dev/\S+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if m := raidLevelPattern.FindStringSubmatch(line); m != nil {
			array.Level = strings.TrimSpace(m[1])
		} else if m := uuidPattern.FindStringSubmatch(line); m != nil {
			array.UUID = strings.TrimSpace(m[1])
		} else if m := statePattern.FindStringSubmatch(line); m != nil {
			metrics.State = strings.TrimSpace(m[1])
		} else if m := activePattern.FindStringSubmatch(line); m != nil {
			metrics.ActiveDevices, _ = strconv.Atoi(m[1])
		} else if m := workingPattern.FindStringSubmatch(line); m != nil {
			metrics.WorkingDevices, _ = strconv.Atoi(m[1])
		} else if m := failedPattern.FindStringSubmatch(line); m != nil {
			metrics.FailedDevices, _ = strconv.Atoi(m[1])
		} else if m := sparePattern.FindStringSubmatch(line); m != nil {
			metrics.SpareDevices, _ = strconv.Atoi(m[1])
		} else if m := rebuildPattern.FindStringSubmatch(line); m != nil {
			progress, _ := strconv.ParseFloat(m[1], 64)
			metrics.SyncProgress = progress
		} else if m := resyncPattern.FindStringSubmatch(line); m != nil {
			progress, _ := strconv.ParseFloat(m[1], 64)
			metrics.SyncProgress = progress
		} else if m := recoveryPattern.FindStringSubmatch(line); m != nil {
			progress, _ := strconv.ParseFloat(m[1], 64)
			metrics.SyncProgress = progress
		} else if m := checkPattern.FindStringSubmatch(line); m != nil {
			progress, _ := strconv.ParseFloat(m[1], 64)
			metrics.SyncProgress = progress
		} else if m := arraySizePattern.FindStringSubmatch(line); m != nil {
			// mdadm reports size in KiB; convert to bytes
			kb, _ := strconv.ParseInt(m[1], 10, 64)
			metrics.ArraySize = kb * 1024
		}

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
