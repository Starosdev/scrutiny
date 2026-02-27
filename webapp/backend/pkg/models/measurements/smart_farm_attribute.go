package measurements

import (
	"fmt"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
)

// SmartFarmAttribute represents a Seagate FARM (Field Accessible Reliability Metrics)
// attribute. Like ATA Device Statistics, FARM attributes use string-based IDs
// with a "farm_" prefix (e.g., "farm_poh", "farm_reallocated_sectors").
type SmartFarmAttribute struct {
	AttributeId      string              `json:"attribute_id"`
	StatusReason     string              `json:"status_reason,omitempty"`
	Value            int64               `json:"value"`
	Threshold        int64               `json:"thresh"`
	TransformedValue int64               `json:"transformed_value"`
	FailureRate      float64             `json:"failure_rate,omitempty"`
	Status           pkg.AttributeStatus `json:"status"`
}

func (sa *SmartFarmAttribute) GetTransformedValue() int64 {
	return sa.TransformedValue
}

func (sa *SmartFarmAttribute) GetStatus() pkg.AttributeStatus {
	return sa.Status
}

func (sa *SmartFarmAttribute) Flatten() map[string]interface{} {
	return map[string]interface{}{
		fmt.Sprintf("attr.%s.attribute_id", sa.AttributeId): sa.AttributeId,
		fmt.Sprintf("attr.%s.value", sa.AttributeId):        sa.Value,
		fmt.Sprintf("attr.%s.thresh", sa.AttributeId):       sa.Threshold,

		// Generated Data
		fmt.Sprintf("attr.%s.transformed_value", sa.AttributeId): sa.TransformedValue,
		fmt.Sprintf("attr.%s.status", sa.AttributeId):            int64(sa.Status),
		fmt.Sprintf("attr.%s.status_reason", sa.AttributeId):     sa.StatusReason,
		fmt.Sprintf("attr.%s.failure_rate", sa.AttributeId):      sa.FailureRate,
	}
}

func (sa *SmartFarmAttribute) Inflate(key string, val interface{}) {
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

	// Generated
	case "transformed_value":
		sa.TransformedValue = val.(int64)
	case "status":
		sa.Status = pkg.AttributeStatus(val.(int64)) //nolint:gosec // status values are always within uint8 range
	case "status_reason":
		sa.StatusReason = val.(string)
	case "failure_rate":
		sa.FailureRate = val.(float64)
	}
}

// PopulateAttributeStatus sets the status based on FARM metadata thresholds.
func (sa *SmartFarmAttribute) PopulateAttributeStatus() *SmartFarmAttribute {
	sa.TransformedValue = sa.Value

	metadata, ok := thresholds.FarmMetadata[sa.AttributeId]
	if !ok {
		return sa
	}

	// Sanity check: reject impossibly high values for "ideal low" attributes
	if metadata.Ideal == thresholds.ObservedThresholdIdealLow && sa.Value > MaxReasonableFailureCount {
		sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusInvalidValue)
		sa.StatusReason = fmt.Sprintf("%s value %d exceeds reasonable maximum (%d), likely corrupted data",
			metadata.DisplayName, sa.Value, MaxReasonableFailureCount)
		return sa
	}

	threshold := metadata.Threshold
	if threshold == 0 && sa.Threshold > 0 {
		threshold = sa.Threshold
	}

	if threshold > 0 {
		if metadata.Ideal == thresholds.ObservedThresholdIdealLow && sa.Value >= threshold {
			sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusFailedScrutiny)
			sa.StatusReason = fmt.Sprintf("%s value %d exceeds threshold %d", metadata.DisplayName, sa.Value, threshold)
		} else if metadata.Ideal == thresholds.ObservedThresholdIdealHigh && sa.Value <= threshold {
			sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusFailedScrutiny)
			sa.StatusReason = fmt.Sprintf("%s value %d is below threshold %d", metadata.DisplayName, sa.Value, threshold)
		}
	} else if metadata.Critical && metadata.Ideal == thresholds.ObservedThresholdIdealLow && sa.Value > 0 {
		sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusWarningScrutiny)
		sa.StatusReason = fmt.Sprintf("%s has non-zero error count: value %d", metadata.DisplayName, sa.Value)
	}

	return sa
}
