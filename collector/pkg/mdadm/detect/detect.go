package detect

import (
	"bufio"
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
	file, err := os.Open("/proc/mdstat")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open /proc/mdstat: %w", err)
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
	}
	
	return array, metrics, err
}

// getRawMdstat extracts the specific multi-line block for an array from /proc/mdstat
func (d *Detect) getRawMdstat(name string) (string, error) {
	file, err := os.Open("/proc/mdstat")
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
	
	// Regex patterns for detail fields
	raidLevelPattern := regexp.MustCompile(`Raid Level\s*:\s*(.+)`)
	uuidPattern := regexp.MustCompile(`UUID\s*:\s*(.+)`)
	statePattern := regexp.MustCompile(`State\s*:\s*(.+)`)
	activePattern := regexp.MustCompile(`Active Devices\s*:\s*(\d+)`)
	workingPattern := regexp.MustCompile(`Working Devices\s*:\s*(\d+)`)
	failedPattern := regexp.MustCompile(`Failed Devices\s*:\s*(\d+)`)
	sparePattern := regexp.MustCompile(`Spare Devices\s*:\s*(\d+)`)
	rebuildPattern := regexp.MustCompile(`Rebuild Status\s*:\s*(\d+)%`)
	resyncPattern := regexp.MustCompile(`Resync Status\s*:\s*(\d+)%`)

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
