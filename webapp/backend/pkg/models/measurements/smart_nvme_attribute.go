package measurements

import (
	"fmt"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
)

type SmartNvmeAttribute struct {
	AttributeId string `json:"attribute_id"` //json string from smartctl
	Value       int64  `json:"value"`
	Threshold   int64  `json:"thresh"`

	TransformedValue int64               `json:"transformed_value"`
	Status           pkg.AttributeStatus `json:"status"`
	StatusReason     string              `json:"status_reason,omitempty"`
	FailureRate      float64             `json:"failure_rate,omitempty"`
}

func (sa *SmartNvmeAttribute) GetTransformedValue() int64 {
	return sa.TransformedValue
}

func (sa *SmartNvmeAttribute) GetStatus() pkg.AttributeStatus {
	return sa.Status
}

func (sa *SmartNvmeAttribute) Flatten() map[string]interface{} {
	return map[string]interface{}{
		fmt.Sprintf("attr.%s.attribute_id", sa.AttributeId): sa.AttributeId,
		fmt.Sprintf("attr.%s.value", sa.AttributeId):        sa.Value,
		fmt.Sprintf("attr.%s.thresh", sa.AttributeId):       sa.Threshold,

		//Generated Data
		fmt.Sprintf("attr.%s.transformed_value", sa.AttributeId): sa.TransformedValue,
		fmt.Sprintf("attr.%s.status", sa.AttributeId):            int64(sa.Status),
		fmt.Sprintf("attr.%s.status_reason", sa.AttributeId):     sa.StatusReason,
		fmt.Sprintf("attr.%s.failure_rate", sa.AttributeId):      sa.FailureRate,
	}
}
func (sa *SmartNvmeAttribute) Inflate(key string, val interface{}) {
	if val == nil {
		return
	}

	keyParts := strings.Split(key, ".")

	switch keyParts[2] {
	case "attribute_id":
		sa.AttributeId = val.(string)
	case "value":
		sa.Value = val.(int64)
	case "thresh":
		sa.Threshold = val.(int64)

	//generated
	case "transformed_value":
		sa.TransformedValue = val.(int64)
	case "status":
		sa.Status = pkg.AttributeStatus(val.(int64))
	case "status_reason":
		sa.StatusReason = val.(string)
	case "failure_rate":
		sa.FailureRate = val.(float64)
	}
}

// nvmeCriticalWarningBits lists the NVMe SMART / Health critical warning bits (NVMe base spec,
// Get Log Page 02h, byte 0).
var nvmeCriticalWarningBits = []struct {
	reason string
	mask   int64
}{
	{"available spare capacity is below the spare threshold", 1 << 0},
	{"temperature is above or below a critical threshold", 1 << 1},
	{"NVM subsystem reliability is degraded", 1 << 2},
	{"media has been placed in read-only mode", 1 << 3},
	{"volatile memory backup device has failed", 1 << 4},
	{"persistent memory region has become read-only", 1 << 5},
}

// decodeNvmeCriticalWarning returns a human readable reason for every bit set in value.
func decodeNvmeCriticalWarning(value int64) string {
	var reasons []string
	known := int64(0)
	for _, bit := range nvmeCriticalWarningBits {
		known |= bit.mask
		if value&bit.mask != 0 {
			reasons = append(reasons, bit.reason)
		}
	}
	if unknown := value &^ known; unknown != 0 {
		reasons = append(reasons, fmt.Sprintf("unrecognized critical warning bits set (0x%02x)", unknown))
	}
	return "Controller critical warning: " + strings.Join(reasons, "; ")
}

// PopulateAttributeStatus sets the status using SMART thresholds and observed metadata.
// Chainable
func (sa *SmartNvmeAttribute) PopulateAttributeStatus() *SmartNvmeAttribute {
	// Any set critical_warning bit is a manufacturer-reported failure, not only a Scrutiny one.
	if sa.AttributeId == "critical_warning" {
		if sa.Value != 0 {
			sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusFailedSmart|pkg.AttributeStatusFailedScrutiny)
			sa.StatusReason += decodeNvmeCriticalWarning(sa.Value)
		}
		return sa
	}

	smartMetadata, ok := thresholds.NmveMetadata[sa.AttributeId]
	if !ok {
		return sa
	}

	//-1 is a special number meaning no threshold.
	if sa.Threshold != -1 {
		// check what the ideal is. Ideal tells us if we our recorded value needs to be above, or below the threshold
		if (smartMetadata.Ideal == "low" && sa.Value > sa.Threshold) ||
			(smartMetadata.Ideal == "high" && sa.Value < sa.Threshold) {
			sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusFailedScrutiny)
			sa.StatusReason += "Attribute is failing recommended SMART threshold"
			return sa
		}
	}
	if smartMetadata.WarnAtOrAbove > 0 && sa.Value >= smartMetadata.WarnAtOrAbove {
		sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusWarningScrutiny)
		sa.StatusReason += fmt.Sprintf("Attribute has reached the recommended warning level (%d)", smartMetadata.WarnAtOrAbove)
	}

	return sa
}
