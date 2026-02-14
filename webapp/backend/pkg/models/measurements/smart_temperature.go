package measurements

import (
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
)

type SmartTemperature struct {
	Date time.Time `json:"date"`
	Temp int64     `json:"temp"`
}

func (st *SmartTemperature) Flatten() (tags map[string]string, fields map[string]interface{}) {
	fields = map[string]interface{}{
		"temp": st.Temp,
	}
	tags = map[string]string{}

	return tags, fields
}

func (st *SmartTemperature) Inflate(key string, val interface{}) {
	if val == nil {
		return
	}

	if key == "temp" {
		switch t := val.(type) {
		case int64:
			st.Temp = t
		case float64:
			st.Temp = int64(t)
		}
	}
}

// CorrectedTemperature extracts a corrected temperature from smartctl data.
// Some drives (especially ATA SSDs behind USB bridges) report incorrect values
// in the standard temperature.current field. This function applies fallbacks:
//   - SCSI/SAS: falls back to scsi_environmental_reports.temperature_1.current
//   - ATA: falls back to attribute 194 raw value (lowest byte via 0xFF bitmask)
func CorrectedTemperature(info collector.SmartInfo) int64 {
	temp := info.Temperature.Current

	// For SCSI/SAS drives, if standard temperature field is 0, check scsi_environmental_reports
	if temp == 0 && len(info.ScsiEnvironmentalReports) > 0 {
		if scsiTemp, ok := info.ScsiEnvironmentalReports["temperature_1"]; ok {
			temp = scsiTemp.Current
		}
	}

	// For ATA drives, if standard temperature is unreasonable, check attribute 194
	if (temp <= 0 || temp > 150) && info.Device.Protocol == pkg.DeviceProtocolAta {
		if fallback := ataAttr194Temperature(info.AtaSmartAttributes.Table); fallback > 0 {
			temp = fallback
		}
	}

	return temp
}

// ataAttr194Temperature extracts temperature from ATA attribute 194's raw value.
// The lowest byte (via 0xFF bitmask) contains the temperature in Celsius.
// Returns 0 if attribute 194 is not found or the value is out of range.
func ataAttr194Temperature(table []collector.AtaSmartAttributesTableItem) int64 {
	for _, attr := range table {
		if attr.ID == 194 && attr.Raw.Value > 0 {
			extractedTemp := attr.Raw.Value & 0xFF
			if extractedTemp > 0 && extractedTemp < 100 {
				return extractedTemp
			}
			return 0
		}
	}
	return 0
}
