package validation

import (
	"errors"
	"regexp"
)

var (
	// wwnRegex validates WWN format: 0x followed by exactly 16 hexadecimal characters
	// Example: 0x5000cca264eb01d7
	wwnRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{16}$`)

	// guidRegex validates ZFS pool GUID format: either decimal (up to 20 digits) or hex (0x prefix)
	// Examples: 12345678901234567890, 0xABCD1234
	guidRegex = regexp.MustCompile(`^(0x[0-9a-fA-F]{1,16}|[0-9]{1,20})$`)

	// ErrInvalidWWN is returned when WWN format validation fails
	ErrInvalidWWN = errors.New("invalid WWN format: must be 0x followed by 16 hexadecimal characters")

	// ErrInvalidGUID is returned when GUID format validation fails
	ErrInvalidGUID = errors.New("invalid GUID format: must be a decimal number or hexadecimal with 0x prefix")
)

// ValidateWWN validates that a WWN (World Wide Name) follows the expected format.
// Valid format: 0x followed by exactly 16 hexadecimal characters (case-insensitive).
// This validation prevents Flux query injection attacks by ensuring only safe characters.
func ValidateWWN(wwn string) error {
	if !wwnRegex.MatchString(wwn) {
		return ErrInvalidWWN
	}
	return nil
}

// ValidateGUID validates that a ZFS pool GUID follows the expected format.
// Valid formats:
//   - Decimal: up to 20 digits (max uint64 = 18446744073709551615)
//   - Hexadecimal: 0x prefix followed by up to 16 hex characters
//
// This validation prevents Flux query injection attacks by ensuring only safe characters.
func ValidateGUID(guid string) error {
	if !guidRegex.MatchString(guid) {
		return ErrInvalidGUID
	}
	return nil
}
