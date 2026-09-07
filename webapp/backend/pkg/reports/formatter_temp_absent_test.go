package reports

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A device that reported no temperature must never be rendered as 0C, and must
// never be selected as the coldest device. Before TempCurrent became a pointer
// the two cases were indistinguishable, and tempExtremes carried a
// "TempCurrent == 0" workaround that made any device without a reading the
// coldest one.
func TestTempExtremesSkipsDevicesWithoutAReading(t *testing.T) {
	devices := []DeviceReport{
		{Name: "/dev/sda", TempCurrent: nil},
		{Name: "/dev/sdb", TempCurrent: tempPtr(41)},
		{Name: "/dev/sdc", TempCurrent: tempPtr(33)},
	}

	hottest, coldest := tempExtremes(devices)

	require.NotNil(t, hottest)
	require.NotNil(t, coldest)
	require.Equal(t, "/dev/sdb", hottest.Name)
	require.Equal(t, "/dev/sdc", coldest.Name, "a device with no reading must not be the coldest")
}

// A device genuinely reading 0C is a real measurement and must still win.
func TestTempExtremesKeepsAGenuineZeroReading(t *testing.T) {
	devices := []DeviceReport{
		{Name: "/dev/sda", TempCurrent: tempPtr(20)},
		{Name: "/dev/sdb", TempCurrent: tempPtr(0)},
	}

	hottest, coldest := tempExtremes(devices)

	require.Equal(t, "/dev/sda", hottest.Name)
	require.Equal(t, "/dev/sdb", coldest.Name)
}

func TestTempExtremesReturnsNilWhenNoDeviceReported(t *testing.T) {
	hottest, coldest := tempExtremes([]DeviceReport{{Name: "/dev/sda"}, {Name: "/dev/sdb"}})

	require.Nil(t, hottest)
	require.Nil(t, coldest)
}

func TestFormatTempRendersPlaceholderOnlyForAbsent(t *testing.T) {
	require.Equal(t, "--", formatTemp(nil))
	require.Equal(t, "0", formatTemp(tempPtr(0)))
	require.Equal(t, "37", formatTemp(tempPtr(37)))
	require.Equal(t, "-5", formatTemp(tempPtr(-5)))
}

// The rendered output is what the operator actually reads, so assert on it
// rather than only on the helper.
func TestHTMLDeviceTableRendersPlaceholderForAbsentReading(t *testing.T) {
	report := &ReportData{
		Devices: []DeviceReport{
			{Name: "/dev/sda", Model: "WDC WD40EFRX", TempCurrent: nil},
			{Name: "/dev/sdb", Model: "Samsung 860", TempCurrent: tempPtr(31)},
		},
	}

	var b strings.Builder
	writeDeviceHTMLTable(&b, report)
	html := b.String()

	require.Contains(t, html, ">--C<", "a device with no reading must not render as a number")
	require.Contains(t, html, ">31C<")
	require.NotContains(t, html, ">0C<")
}

// The text summary must not name a device that never reported a temperature.
func TestTextTempSummarySkipsDevicesWithoutAReading(t *testing.T) {
	devices := []DeviceReport{
		{Name: "/dev/sda", TempCurrent: nil},
		{Name: "/dev/sdb", TempCurrent: tempPtr(41), TempAvg: 40},
		{Name: "/dev/sdc", TempCurrent: tempPtr(33), TempAvg: 33},
	}

	text := strings.Join(appendTempSummary(nil, devices), "\n")

	require.Contains(t, text, "/dev/sdb")
	require.Contains(t, text, "/dev/sdc")
	require.NotContains(t, text, "/dev/sda")
}
