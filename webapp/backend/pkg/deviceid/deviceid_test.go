package deviceid

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerate_Deterministic(t *testing.T) {
	id1 := Generate("QEMU HARDDISK", "QM00001", "0x5000cca264eb01d7")
	id2 := Generate("QEMU HARDDISK", "QM00001", "0x5000cca264eb01d7")
	require.Equal(t, id1, id2, "same inputs must produce the same UUID")
	require.Len(t, id1, 36, "output must be a standard UUID string")
}

func TestGenerate_DifferentSerials(t *testing.T) {
	id1 := Generate("Samsung SSD 870", "S1SERIAL01", "0x5000cca264eb01d7")
	id2 := Generate("Samsung SSD 870", "S1SERIAL02", "0x5000cca264eb01d7")
	require.NotEqual(t, id1, id2, "different serials with same WWN must produce different UUIDs")
}

func TestGenerate_DifferentModels(t *testing.T) {
	id1 := Generate("Samsung SSD 870", "S1SERIAL01", "0x5000cca264eb01d7")
	id2 := Generate("Samsung SSD 980", "S1SERIAL01", "0x5000cca264eb01d7")
	require.NotEqual(t, id1, id2, "different models must produce different UUIDs")
}

func TestGenerate_EmptyWWN(t *testing.T) {
	id := Generate("Samsung SSD 870", "S1SERIAL01", "")
	require.Len(t, id, 36, "empty WWN must still produce a valid UUID")
	require.NotEmpty(t, id)
}

func TestGenerate_EmptyAll(t *testing.T) {
	id := Generate("", "", "")
	require.Len(t, id, 36, "all-empty inputs must still produce a valid UUID")
}

func TestGenerate_CaseInsensitive(t *testing.T) {
	id1 := Generate("QEMU HARDDISK", "QM00001", "0x5000CCA264EB01D7")
	id2 := Generate("qemu harddisk", "qm00001", "0x5000cca264eb01d7")
	require.Equal(t, id1, id2, "inputs differing only in case must produce the same UUID")
}

func TestGenerate_WhitespaceTrimming(t *testing.T) {
	id1 := Generate("QEMU HARDDISK", "QM00001", "0x5000cca264eb01d7")
	id2 := Generate("  QEMU HARDDISK  ", "  QM00001  ", "  0x5000cca264eb01d7  ")
	require.Equal(t, id1, id2, "leading/trailing whitespace must be trimmed")
}

func TestGenerate_SameWWNDifferentSerial_UniqueIDs(t *testing.T) {
	// This is the core use case: two drives sharing a WWN but with different serials
	id1 := Generate("WDC WD40EFRX", "WD-ABC123", "0x5000cca264eb01d7")
	id2 := Generate("WDC WD40EFRX", "WD-XYZ789", "0x5000cca264eb01d7")
	require.NotEqual(t, id1, id2, "cloned drives with same WWN but different serials must be distinguishable")
}

func TestGenerate_NoWWN_DifferentSerials(t *testing.T) {
	// Drives with no WWN should be tracked separately by serial
	id1 := Generate("NVMe Drive", "SERIAL001", "")
	id2 := Generate("NVMe Drive", "SERIAL002", "")
	require.NotEqual(t, id1, id2)
}

func TestGenerate_ValidUUIDFormat(t *testing.T) {
	id := Generate("test", "serial", "wwn")
	// UUID format: 8-4-4-4-12 hex digits
	require.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}
