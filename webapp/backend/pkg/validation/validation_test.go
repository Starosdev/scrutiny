package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateWWN(t *testing.T) {
	tests := []struct {
		name    string
		wwn     string
		wantErr bool
	}{
		// Valid WWN formats
		{
			name:    "valid WWN lowercase",
			wwn:     "0x5000cca264eb01d7",
			wantErr: false,
		},
		{
			name:    "valid WWN uppercase",
			wwn:     "0x5000CCA264EB01D7",
			wantErr: false,
		},
		{
			name:    "valid WWN mixed case",
			wwn:     "0x5002538e40a22954",
			wantErr: false,
		},
		{
			name:    "valid WWN all zeros",
			wwn:     "0x0000000000000000",
			wantErr: false,
		},
		{
			name:    "valid WWN all F",
			wwn:     "0xFFFFFFFFFFFFFFFF",
			wantErr: false,
		},
		// Invalid WWN formats
		{
			name:    "empty string",
			wwn:     "",
			wantErr: true,
		},
		{
			name:    "missing 0x prefix",
			wwn:     "5000cca264eb01d7",
			wantErr: true,
		},
		{
			name:    "only 0x prefix",
			wwn:     "0x",
			wantErr: true,
		},
		{
			name:    "too short",
			wwn:     "0x5000cca264eb01",
			wantErr: true,
		},
		{
			name:    "too long",
			wwn:     "0x5000cca264eb01d7a",
			wantErr: true,
		},
		{
			name:    "invalid hex character",
			wwn:     "0x5000cca264eb01dg",
			wantErr: true,
		},
		// Injection attempts
		{
			name:    "injection with quote",
			wwn:     `0x5000cca264eb01d7"`,
			wantErr: true,
		},
		{
			name:    "injection with closing paren",
			wwn:     "0x5000cca264eb01d7)",
			wantErr: true,
		},
		{
			name:    "injection with or",
			wwn:     `0x5000cca264eb01d7" or 1=1`,
			wantErr: true,
		},
		{
			name:    "injection with pipe",
			wwn:     "0x5000cca264eb01d7 |> yield",
			wantErr: true,
		},
		{
			name:    "injection with newline",
			wwn:     "0x5000cca264eb01d7\n|>",
			wantErr: true,
		},
		{
			name:    "sql-style injection",
			wwn:     "'; DROP TABLE devices; --",
			wantErr: true,
		},
		{
			name:    "flux-style injection",
			wwn:     `" or true) or (r["`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWWN(tt.wwn)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, ErrInvalidWWN, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGUID(t *testing.T) {
	tests := []struct {
		name    string
		guid    string
		wantErr bool
	}{
		// Valid GUID formats - decimal
		{
			name:    "valid decimal GUID",
			guid:    "12345678901234567890",
			wantErr: false,
		},
		{
			name:    "valid short decimal GUID",
			guid:    "123",
			wantErr: false,
		},
		{
			name:    "valid single digit GUID",
			guid:    "0",
			wantErr: false,
		},
		{
			name:    "valid max uint64 GUID",
			guid:    "18446744073709551615",
			wantErr: false,
		},
		// Valid GUID formats - hexadecimal
		{
			name:    "valid hex GUID lowercase",
			guid:    "0xabcd1234",
			wantErr: false,
		},
		{
			name:    "valid hex GUID uppercase",
			guid:    "0xABCD1234",
			wantErr: false,
		},
		{
			name:    "valid hex GUID max length",
			guid:    "0xFFFFFFFFFFFFFFFF",
			wantErr: false,
		},
		{
			name:    "valid hex GUID single digit",
			guid:    "0x0",
			wantErr: false,
		},
		// Invalid GUID formats
		{
			name:    "empty string",
			guid:    "",
			wantErr: true,
		},
		{
			name:    "only 0x prefix",
			guid:    "0x",
			wantErr: true,
		},
		{
			name:    "hex too long",
			guid:    "0x12345678901234567",
			wantErr: true,
		},
		{
			name:    "decimal too long",
			guid:    "123456789012345678901",
			wantErr: true,
		},
		{
			name:    "contains letters without 0x prefix",
			guid:    "abcd1234",
			wantErr: true,
		},
		{
			name:    "invalid hex character",
			guid:    "0xGHIJ",
			wantErr: true,
		},
		// Injection attempts
		{
			name:    "injection with quote",
			guid:    `12345"`,
			wantErr: true,
		},
		{
			name:    "injection with closing paren",
			guid:    "12345)",
			wantErr: true,
		},
		{
			name:    "injection with or",
			guid:    `12345" or 1=1`,
			wantErr: true,
		},
		{
			name:    "injection with pipe",
			guid:    "12345 |> yield",
			wantErr: true,
		},
		{
			name:    "sql-style injection",
			guid:    "'; DROP TABLE pools; --",
			wantErr: true,
		},
		{
			name:    "flux-style injection",
			guid:    `" or true) or (r["`,
			wantErr: true,
		},
		{
			name:    "negative number",
			guid:    "-12345",
			wantErr: true,
		},
		{
			name:    "decimal with spaces",
			guid:    "123 456",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGUID(tt.guid)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, ErrInvalidGUID, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
