package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// nvmeNamespacePath constructs the expected namespace block device path from a controller path.
// For example, "/dev/nvme0" returns "/dev/nvme0n1".
func nvmeNamespacePath(controllerPath string) string {
	return fmt.Sprintf("/dev/%sn1", filepath.Base(controllerPath))
}

// resolveNVMeBlockDevice attempts to find the namespace block device for an NVMe controller.
// smartctl reports controller character devices (/dev/nvme0), but fio needs a namespace
// block device (/dev/nvme0n1) that supports O_DIRECT.
func resolveNVMeBlockDevice(controllerPath string) (string, error) {
	namespacePath := nvmeNamespacePath(controllerPath)

	isBlock, err := isBlockDevice(namespacePath)
	if err != nil {
		return "", fmt.Errorf("namespace device %s not found: %w", namespacePath, err)
	}
	if !isBlock {
		return "", fmt.Errorf("%s exists but is not a block device", namespacePath)
	}

	return namespacePath, nil
}

// isBlockDevice checks whether the given path is a block device.
func isBlockDevice(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("could not get syscall.Stat_t for %s", path)
	}

	return stat.Mode&syscall.S_IFBLK != 0, nil
}
