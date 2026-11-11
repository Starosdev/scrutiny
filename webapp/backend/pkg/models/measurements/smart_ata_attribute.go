package measurements

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
)

var ataAttributeNameTokenPattern = regexp.MustCompile(`[^a-z0-9]+`)

var ataAttributeNameIgnoredTokens = map[string]struct{}{
	"ct":    {},
	"count": {},
	"total": {},
}

const ataSsdVendorSpecificThresholdSkipReason = "Skipped generic ATA observed-threshold scoring for vendor-specific SSD attribute"

const ataFailedInThePastReason = "Attribute has previously failed manufacturer SMART threshold"

type SmartAtaAttribute struct {
	AttributeId int    `json:"attribute_id"`
	Name        string `json:"name,omitempty"`
	Value       int64  `json:"value"`
	Threshold   int64  `json:"thresh"`
	Worst       int64  `json:"worst"`
	RawValue    int64  `json:"raw_value"`
	RawString   string `json:"raw_string"`
	WhenFailed  string `json:"when_failed"`

	//Generated data
	TransformedValue int64               `json:"transformed_value"`
	Status           pkg.AttributeStatus `json:"status"`
	StatusReason     string              `json:"status_reason,omitempty"`
	FailureRate      float64             `json:"failure_rate,omitempty"`
}

// FailedOnlyInThePast reports whether the attribute's only finding is smartctl's sticky
// when_failed "past" mark: no current failure and no threshold or override finding of its own.
// smartctl derives that mark from worst <= thresh, so Worst, not Value, is what can change it.
func (sa *SmartAtaAttribute) FailedOnlyInThePast() bool {
	return strings.EqualFold(sa.WhenFailed, pkg.AttributeWhenFailedInThePast) &&
		sa.Status == pkg.AttributeStatusWarningScrutiny &&
		sa.StatusReason == ataFailedInThePastReason
}

func (sa *SmartAtaAttribute) GetTransformedValue() int64 {
	return sa.TransformedValue
}

func (sa *SmartAtaAttribute) GetStatus() pkg.AttributeStatus {
	return sa.Status
}

func (sa *SmartAtaAttribute) Flatten() map[string]interface{} {

	idString := strconv.Itoa(sa.AttributeId)

	return map[string]interface{}{
		fmt.Sprintf("attr.%s.attribute_id", idString): idString,
		fmt.Sprintf("attr.%s.name", idString):         sa.Name,
		fmt.Sprintf("attr.%s.value", idString):        sa.Value,
		fmt.Sprintf("attr.%s.worst", idString):        sa.Worst,
		fmt.Sprintf("attr.%s.thresh", idString):       sa.Threshold,
		fmt.Sprintf("attr.%s.raw_value", idString):    sa.RawValue,
		fmt.Sprintf("attr.%s.raw_string", idString):   sa.RawString,
		fmt.Sprintf("attr.%s.when_failed", idString):  sa.WhenFailed,

		//Generated Data
		fmt.Sprintf("attr.%s.transformed_value", idString): sa.TransformedValue,
		fmt.Sprintf("attr.%s.status", idString):            int64(sa.Status),
		fmt.Sprintf("attr.%s.status_reason", idString):     sa.StatusReason,
		fmt.Sprintf("attr.%s.failure_rate", idString):      sa.FailureRate,
	}
}
func (sa *SmartAtaAttribute) Inflate(key string, val interface{}) {
	if val == nil {
		return
	}
	keyParts := strings.Split(key, ".")

	switch keyParts[2] {
	case "attribute_id":
		attrId, err := strconv.Atoi(val.(string))
		if err == nil {
			sa.AttributeId = attrId
		}
	case "name":
		sa.Name = val.(string)
	case "value":
		sa.Value = val.(int64)
	case "worst":
		sa.Worst = val.(int64)
	case "thresh":
		sa.Threshold = val.(int64)
	case "raw_value":
		sa.RawValue = val.(int64)
	case "raw_string":
		sa.RawString = val.(string)
	case "when_failed":
		sa.WhenFailed = val.(string)

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

// populate attribute status, using SMART Thresholds & Observed Metadata
// Chainable
func (sa *SmartAtaAttribute) PopulateAttributeStatus(profile *thresholds.ConsumerDriveProfile, isAtaSsd bool) *SmartAtaAttribute {
	if strings.EqualFold(sa.WhenFailed, pkg.AttributeWhenFailedFailingNow) {
		//this attribute has previously failed
		sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusFailedSmart)
		sa.StatusReason += "Attribute is failing manufacturer SMART threshold"
		//if the Smart Status is failed, we should exit early, no need to look at thresholds.
		return sa

	} else if strings.EqualFold(sa.WhenFailed, pkg.AttributeWhenFailedInThePast) {
		sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusWarningScrutiny)
		sa.StatusReason += ataFailedInThePastReason
	}

	if smartMetadata, ok := thresholds.AtaMetadata[sa.AttributeId]; ok {
		if sa.Status == pkg.AttributeStatusPassed && shouldSkipAtaSsdObservedThresholdScoring(isAtaSsd, &smartMetadata, sa.Name) {
			sa.StatusReason = ataSsdVendorSpecificThresholdSkipReason
			return sa
		}
		if profile != nil {
			if profileThresholds, ok := profile.AtaObservedThresholds[sa.AttributeId]; ok && len(profileThresholds) > 0 {
				smartMetadata.ObservedThresholds = profileThresholds
			}
		}
		sa.ValidateThreshold(smartMetadata)
	}

	return sa
}

func shouldSkipAtaSsdObservedThresholdScoring(isAtaSsd bool, smartMetadata *thresholds.AtaAttributeMetadata, attributeName string) bool {
	if !isAtaSsd || smartMetadata == nil || len(smartMetadata.ObservedThresholds) == 0 {
		return false
	}
	return !ataAttributeNamesSemanticallyMatch(attributeName, smartMetadata.DisplayName)
}

func ataAttributeNamesSemanticallyMatch(reportedName string, metadataName string) bool {
	reportedTokens := ataAttributeNameTokens(reportedName)
	metadataTokens := ataAttributeNameTokens(metadataName)
	if len(reportedTokens) == 0 || len(metadataTokens) == 0 {
		return false
	}

	shorter := reportedTokens
	longer := metadataTokens
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}

	return orderedSubsequence(shorter, longer)
}

func ataAttributeNameTokens(name string) []string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return nil
	}

	rawTokens := ataAttributeNameTokenPattern.Split(normalized, -1)
	tokens := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		if token == "" {
			continue
		}
		if _, ignored := ataAttributeNameIgnoredTokens[token]; ignored {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func orderedSubsequence(shorter []string, longer []string) bool {
	idx := 0
	for _, token := range longer {
		if shorter[idx] != token {
			continue
		}
		idx++
		if idx == len(shorter) {
			return true
		}
	}
	return false
}

// compare the attribute (raw, normalized, transformed) value to observed thresholds, and update status if necessary
func (sa *SmartAtaAttribute) ValidateThreshold(smartMetadata thresholds.AtaAttributeMetadata) {
	// - if the attribute is critical
	//		- the failure rate is over 10 - set to failed
	// - if the attribute is not critical
	//		- if failure rate is above 20 - set to failed
	// 		- if failure rate is above 10 but below 20 - set to warn

	//update the smart attribute status based on Observed thresholds.
	value := sa.thresholdValue(smartMetadata.DisplayType)

	rate, found := observedFailureRateAt(&smartMetadata, value)
	if !found {
		// the attribute is better than every bucket, or carries no buckets at all: there is no
		// observed failure rate to score, and the manufacturer threshold has already been checked.
		return
	}

	sa.FailureRate = rate
	sa.applyFailureRateStatus(smartMetadata.Critical)
}

// observedFailureRateAt returns the failure rate to score for value, and whether any bucket
// applied.
//
// The buckets are annual failure rates measured over a mixed-drive population, not an ordinal
// severity scale. Sparse buckets are non-monotonic, and the covered range stops well short of the
// worst value a drive can report, so scoring only the bucket a value lands in let a worse value
// earn a milder status than a better one and let a value past the last bucket go unscored.
//
// When the metadata declares an Ideal direction, the rate is therefore the worst rate observed by
// the time the attribute has degraded to value, which never improves as the attribute worsens and
// carries the worst rate in the covered range out past the last bucket. Without an Ideal there is
// no worsening direction to accumulate along, so the bucket containing value is scored alone.
func observedFailureRateAt(smartMetadata *thresholds.AtaAttributeMetadata, value int64) (float64, bool) {
	switch smartMetadata.Ideal {
	case thresholds.ObservedThresholdIdealLow, thresholds.ObservedThresholdIdealHigh:
		return worstObservedFailureRate(smartMetadata.ObservedThresholds, smartMetadata.Ideal, value)
	}

	for _, obsThresh := range smartMetadata.ObservedThresholds {
		if observedThresholdContains(obsThresh, value) {
			return observedFailureRate(obsThresh), true
		}
	}
	return 0, false
}

// worstObservedFailureRate returns the highest failure rate among the buckets reached by value.
func worstObservedFailureRate(obsThresholds []thresholds.ObservedThreshold, ideal string, value int64) (float64, bool) {
	worst := 0.0
	found := false
	for _, obsThresh := range obsThresholds {
		if !observedThresholdReachedBy(obsThresh, ideal, value) {
			continue
		}
		if rate := observedFailureRate(obsThresh); !found || rate > worst {
			worst = rate
		}
		found = true
	}
	return worst, found
}

// observedThresholdReachedBy reports whether an attribute degrading to value has passed through
// this bucket: for an ideal-low attribute the value has climbed to at least the lowest value the
// bucket covers, and for an ideal-high one it has dropped to at most the highest.
//
// The edges match observedThresholdContains exactly, so no value changes which bucket it scores
// against; the envelope only adds the buckets already passed. A value no bucket covers, such as one
// sitting between a singleton zero bucket and a span that starts above it, carries the rate from
// below it, because the population the buckets came from holds no observation of that value.
func observedThresholdReachedBy(obsThresh thresholds.ObservedThreshold, ideal string, value int64) bool {
	if ideal == thresholds.ObservedThresholdIdealHigh {
		return value <= obsThresh.High
	}

	if obsThresh.Low == obsThresh.High {
		return value >= obsThresh.Low
	}
	// a span bucket's lowest covered value is Low+1, compared as a strict inequality so a bucket
	// declaring math.MaxInt64 as its floor does not overflow into one that every value reaches.
	// The bundled catalog already carries math.MaxInt64 as a ceiling.
	return value > obsThresh.Low
}

// thresholdValue returns the attribute value to compare against observed thresholds, selected by
// the metadata display type (normalized, transformed, or raw).
func (sa *SmartAtaAttribute) thresholdValue(displayType string) int64 {
	switch displayType {
	case thresholds.AtaSmartAttributeDisplayTypeNormalized:
		return int64(sa.Value)
	case thresholds.AtaSmartAttributeDisplayTypeTransformed:
		return sa.TransformedValue
	default:
		return sa.RawValue
	}
}

// DisplayValue returns the value the UI plots for this attribute, selected by its
// metadata display type. Callers that report a value beside a status-derived
// judgement use it so the number they show is the number the charts show.
// Unknown attributes fall back to the raw value.
func (sa *SmartAtaAttribute) DisplayValue() int64 {
	metadata, ok := thresholds.AtaMetadata[sa.AttributeId]
	if !ok {
		return sa.RawValue
	}
	return sa.thresholdValue(metadata.DisplayType)
}

// observedThresholdContains reports whether value falls within an observed-threshold bucket.
func observedThresholdContains(obsThresh thresholds.ObservedThreshold, value int64) bool {
	return ((obsThresh.Low == obsThresh.High) && value == obsThresh.Low) ||
		(obsThresh.Low < value && value <= obsThresh.High)
}

// observedFailureRate returns the bucket's annual failure rate, falling back to the midpoint of
// the error interval when the annual rate is 0 but the interval carries real values.
func observedFailureRate(obsThresh thresholds.ObservedThreshold) float64 {
	rate := obsThresh.AnnualFailureRate
	if rate == 0 && len(obsThresh.ErrorInterval) == 2 &&
		(obsThresh.ErrorInterval[0] != 0 || obsThresh.ErrorInterval[1] != 0) {
		rate = (obsThresh.ErrorInterval[0] + obsThresh.ErrorInterval[1]) / 2.0
	}
	return rate
}

// applyFailureRateStatus sets the attribute status/reason from its failure rate, using stricter
// thresholds for critical attributes (fail >= 10%) than non-critical ones (fail >= 20%, warn >= 10%).
func (sa *SmartAtaAttribute) applyFailureRateStatus(critical bool) {
	if critical {
		if sa.FailureRate >= 0.10 {
			sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusFailedScrutiny)
			sa.StatusReason += "Observed Failure Rate for Critical Attribute is greater than 10%"
		}
		return
	}

	if sa.FailureRate >= 0.20 {
		sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusFailedScrutiny)
		sa.StatusReason += "Observed Failure Rate for Non-Critical Attribute is greater than 20%"
	} else if sa.FailureRate >= 0.10 {
		sa.Status = pkg.AttributeStatusSet(sa.Status, pkg.AttributeStatusWarningScrutiny)
		sa.StatusReason += "Observed Failure Rate for Non-Critical Attribute is greater than 10%"
	}
}
