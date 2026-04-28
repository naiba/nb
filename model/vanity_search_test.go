package model

import (
	"testing"
)

func TestVanityMatcher_Match(t *testing.T) {
	tests := []struct {
		name          string
		config        *VanityConfig
		address       string
		expectedMatch bool
	}{
		// Prefix mode tests
		{
			name: "prefix mode - exact match at start",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef1234567890",
			expectedMatch: true,
		},
		{
			name: "prefix mode - no match",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "xyzabc1234567890",
			expectedMatch: false,
		},
		{
			name: "prefix mode - case sensitive match",
			config: &VanityConfig{
				Contains:      "ABC",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "ABCdef1234567890",
			expectedMatch: true,
		},
		{
			name: "prefix mode - case sensitive no match",
			config: &VanityConfig{
				Contains:      "ABC",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef1234567890",
			expectedMatch: false,
		},

		// Suffix mode tests
		{
			name: "suffix mode - exact match at end",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          VanityModeSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "1234567890abcxyz",
			expectedMatch: true,
		},
		{
			name: "suffix mode - no match",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          VanityModeSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "1234567890xyz123",
			expectedMatch: false,
		},
		{
			name: "suffix mode - case sensitive match",
			config: &VanityConfig{
				Contains:      "XYZ",
				Mode:          VanityModeSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "1234567890abcXYZ",
			expectedMatch: true,
		},

		// Prefix-or-suffix mode tests
		{
			name: "prefix-or-suffix mode - match at start",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "123abcdef",
			expectedMatch: true,
		},
		{
			name: "prefix-or-suffix mode - match at end",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef123",
			expectedMatch: true,
		},
		{
			name: "prefix-or-suffix mode - match at both",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "123abc123",
			expectedMatch: true,
		},
		{
			name: "prefix-or-suffix mode - no match",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abc123def",
			expectedMatch: false,
		},

		// Case insensitive tests (CaseSensitive = false, UpperOrLower = false)
		{
			name: "case insensitive - lowercase pattern matches uppercase address",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address:       "ABCdef1234567890",
			expectedMatch: true,
		},
		{
			name: "case insensitive - uppercase pattern matches lowercase address",
			config: &VanityConfig{
				Contains:      "ABC",
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address:       "abcdef1234567890",
			expectedMatch: true,
		},
		{
			name: "case insensitive - mixed case pattern matches",
			config: &VanityConfig{
				Contains:      "AbC",
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address:       "aBcdef1234567890",
			expectedMatch: true,
		},

		// UpperOrLower mode tests (CaseSensitive = false, UpperOrLower = true)
		// This mode checks if address matches upper(contains) OR lower(contains)
		// WITHOUT modifying the address
		{
			name: "upper-or-lower - all uppercase match",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "ABCdef1234567890",
			expectedMatch: true,
		},
		{
			name: "upper-or-lower - all lowercase match",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "abcdef1234567890",
			expectedMatch: true,
		},
		{
			name: "upper-or-lower - mixed case no match",
			config: &VanityConfig{
				Contains:      "abc", // will check for "ABC" or "abc"
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "aBcdef1234567890", // "aBc" doesn't match "ABC" or "abc"
			expectedMatch: false,
		},
		{
			name: "upper-or-lower - suffix mode uppercase match",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          VanityModeSuffix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "1234567890abcXYZ",
			expectedMatch: true,
		},
		{
			name: "upper-or-lower - suffix mode lowercase match",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          VanityModeSuffix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "1234567890abcxyz",
			expectedMatch: true,
		},
		{
			name: "upper-or-lower - suffix mode mixed case no match",
			config: &VanityConfig{
				Contains:      "xyz", // will check for "XYZ" or "xyz"
				Mode:          VanityModeSuffix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "1234567890abcxYz", // "xYz" doesn't match "XYZ" or "xyz"
			expectedMatch: false,
		},
		{
			name: "upper-or-lower - pattern not matching in either case",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "abcdef1234567890",
			expectedMatch: false,
		},
		{
			name: "upper-or-lower - prefix-or-suffix mode, uppercase at start",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "ABC1234567890",
			expectedMatch: true, // matches "ABC" at prefix
		},
		{
			name: "upper-or-lower - prefix-or-suffix mode, lowercase at end",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "1234567890abc",
			expectedMatch: true, // matches "abc" at suffix
		},
		{
			name: "upper-or-lower - prefix-or-suffix mode, mixed case no match",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address:       "AbC1234567890",
			expectedMatch: false, // "AbC" doesn't match "ABC" or "abc"
		},

		// Edge cases
		{
			name: "edge case - empty pattern",
			config: &VanityConfig{
				Contains:      "",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef1234567890",
			expectedMatch: true,
		},
		{
			name: "edge case - pattern equals address",
			config: &VanityConfig{
				Contains:      "abcdef",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef",
			expectedMatch: true,
		},
		{
			name: "edge case - pattern longer than address (prefix)",
			config: &VanityConfig{
				Contains:      "abcdefghij",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcde",
			expectedMatch: false,
		},
		{
			name: "edge case - pattern longer than address (suffix)",
			config: &VanityConfig{
				Contains:      "abcdefghij",
				Mode:          VanityModeSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "fghij",
			expectedMatch: false,
		},
		{
			name: "edge case - single character prefix match",
			config: &VanityConfig{
				Contains:      "T",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "T1234567890abcdef",
			expectedMatch: true,
		},
		{
			name: "edge case - single character suffix match",
			config: &VanityConfig{
				Contains:      "z",
				Mode:          VanityModeSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "1234567890abcdefz",
			expectedMatch: true,
		},

		// Numeric patterns
		{
			name: "numeric pattern - prefix",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "123456789",
			expectedMatch: true,
		},
		{
			name: "numeric pattern - suffix",
			config: &VanityConfig{
				Contains:      "888",
				Mode:          VanityModeSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "TUko4kGyHZZLYv1MrZhpeTSZXMMhZDh888",
			expectedMatch: true,
		},

		// Hexadecimal patterns (for Ethereum addresses)
		{
			name: "hex pattern - prefix with 0x",
			config: &VanityConfig{
				Contains:      "0xabc",
				Mode:          VanityModePrefix,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address:       "0xABCdef1234567890",
			expectedMatch: true,
		},
		{
			name: "hex pattern - suffix deadbeef",
			config: &VanityConfig{
				Contains:      "deadbeef",
				Mode:          VanityModeSuffix,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address:       "1234567890abcdefdeadbeef",
			expectedMatch: true,
		},

		// Bitmask tests
		{
			name: "bitmask - match low 14 bits",
			config: &VanityConfig{
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				Mask:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
				MaskValue:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
			},
			address:       "B0B0000000000000000000000000000000002280",
			expectedMatch: true,
		},
		{
			name: "bitmask - no match low 14 bits",
			config: &VanityConfig{
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				Mask:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
				MaskValue:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
			},
			address:       "B0B0000000000000000000000000000000001111",
			expectedMatch: false,
		},
		{
			name: "bitmask - full V4 hook mask (prefix + flags)",
			config: &VanityConfig{
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				Mask:          []byte{0xFF, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
				MaskValue:     []byte{0xB0, 0xB0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
			},
			address:       "B0B7429Ea01F76102f053213463D4e95D5D02280",
			expectedMatch: true,
		},
		{
			name: "bitmask - prefix mismatch",
			config: &VanityConfig{
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				Mask:          []byte{0xFF, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
				MaskValue:     []byte{0xB0, 0xB0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
			},
			address:       "A0B7429Ea01F76102f053213463D4e95D5D02280",
			expectedMatch: false,
		},
		{
			name: "bitmask only - no contains",
			config: &VanityConfig{
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				Mask:          []byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				MaskValue:     []byte{0xAA, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			},
			address:       "AA11223344556677889900aabbccddeeff001122",
			expectedMatch: true,
		},
		{
			name: "bitmask with contains - both must match",
			config: &VanityConfig{
				Contains:      "B0B",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				Mask:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
				MaskValue:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
			},
			address:       "B0B7429Ea01F76102f053213463D4e95D5D02280",
			expectedMatch: true,
		},
		{
			name: "bitmask with contains - mask matches but contains fails",
			config: &VanityConfig{
				Contains:      "B0B",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				Mask:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
				MaskValue:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
			},
			address:       "A0A7429Ea01F76102f053213463D4e95D5D02280",
			expectedMatch: false,
		},

		// prefix-and-suffix mode tests
		{
			name: "prefix-and-suffix - both match",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixAndSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef1234567abc",
			expectedMatch: true,
		},
		{
			name: "prefix-and-suffix - only prefix matches",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixAndSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef1234567890",
			expectedMatch: false,
		},
		{
			name: "prefix-and-suffix - only suffix matches",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixAndSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "xyz1234567890abc",
			expectedMatch: false,
		},
		{
			name: "prefix-and-suffix - neither matches",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixAndSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "xyz1234567890xyz",
			expectedMatch: false,
		},
		{
			// Guards against the overlap bug: address shorter than 2*contains
			// must NOT be considered a match even if prefix and suffix text overlap.
			name: "prefix-and-suffix - too short, prefix/suffix would overlap",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixAndSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcbc",
			expectedMatch: false,
		},
		{
			name: "prefix-and-suffix - exact 2x length, no overlap",
			config: &VanityConfig{
				Contains:      "ab",
				Mode:          VanityModePrefixAndSuffix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abab",
			expectedMatch: true,
		},
		{
			name: "prefix-and-suffix - case insensitive match",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefixAndSuffix,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address:       "ABCdef1234567ABC",
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewVanityMatcher(tt.config)
			result := matcher.Match(tt.address)

			if result != tt.expectedMatch {
				t.Errorf("VanityMatcher.Match() = %v, want %v\nConfig: %+v\nAddress: %s",
					result, tt.expectedMatch, tt.config, tt.address)
			}
		})
	}
}

// TestVanityMatcher_EitherMode exercises the `--case either` semantic on the
// fast-path (checksumFn set, generator output is lowercase hex). "either"
// means: at the matched position, the EIP-55 form is all-upper OR all-lower.
// Mixed case in the matched segment must be rejected.
//
// Fake EIP-55 here is just an upper/lower control knob; the real EIP-55 logic
// lives in the ethereum package. The matcher only needs the checksum->display
// contract (lowercase in, display casing out).
func TestVanityMatcher_EitherMode(t *testing.T) {
	// EIP-55 stub: caller supplies the exact display form to test against.
	makeChecksum := func(display string) ChecksumFunc {
		return func(lower string) string {
			if len(display) != len(lower) {
				t.Fatalf("display length %d != lower length %d", len(display), len(lower))
			}
			return display
		}
	}

	tests := []struct {
		name          string
		contains      string
		mode          VanityMode
		lowerAddr     string
		displayAddr   string
		expectedMatch bool
	}{
		{
			name:          "prefix all-lowercase in EIP-55",
			contains:      "abc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "abcDEF1234567890", // "abc" all-lower
			expectedMatch: true,
		},
		{
			name:          "prefix all-uppercase in EIP-55",
			contains:      "abc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "ABCdef1234567890", // "ABC" all-upper
			expectedMatch: true,
		},
		{
			name:          "prefix mixed-case in EIP-55 rejected",
			contains:      "abc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "aBcdef1234567890", // "aBc" mixed → reject
			expectedMatch: false,
		},
		{
			name:          "prefix lowercase prefilter rejects wrong letters",
			contains:      "abc",
			mode:          VanityModePrefix,
			lowerAddr:     "xyzdef1234567890",
			displayAddr:   "XYZdef1234567890",
			expectedMatch: false,
		},
		{
			name:          "suffix all-lowercase",
			contains:      "def",
			mode:          VanityModeSuffix,
			lowerAddr:     "1234567890abcdef",
			displayAddr:   "1234567890ABCdef",
			expectedMatch: true,
		},
		{
			name:          "suffix mixed rejected",
			contains:      "def",
			mode:          VanityModeSuffix,
			lowerAddr:     "1234567890abcdef",
			displayAddr:   "1234567890ABCdEf",
			expectedMatch: false,
		},
		{
			name:          "digits-only contains matches any case",
			contains:      "123",
			mode:          VanityModePrefix,
			lowerAddr:     "123abcdef4567890",
			displayAddr:   "123aBcDeF4567890", // digits have no case, matched segment is "123"
			expectedMatch: true,
		},
		{
			name:          "prefix-or-suffix all-upper at end",
			contains:      "abc",
			mode:          VanityModePrefixOrSuffix,
			lowerAddr:     "1234567890defabc",
			displayAddr:   "1234567890defABC",
			expectedMatch: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &VanityConfig{
				Contains:      tt.contains,
				Mode:          tt.mode,
				CaseSensitive: true,
				UpperOrLower:  true,
			}
			m := NewVanityMatcher(cfg)
			m.checksumFn = makeChecksum(tt.displayAddr)
			got := m.Match(tt.lowerAddr)
			if got != tt.expectedMatch {
				t.Fatalf("got %v, want %v (contains=%q, lower=%q, display=%q)",
					got, tt.expectedMatch, tt.contains, tt.lowerAddr, tt.displayAddr)
			}
		})
	}
}

// TestVanityMatcher_FastPathSensitive exercises the `--case sensitive` branch
// of the fast-path (checksumFn set): lowercase prefilter, then exact EIP-55
// verify. Critical scenarios:
//   - lowercase contains "abc" must reject EIP-55 display "aBc" (documented
//     as a prior silent-false-match bug in the matcher comment).
//   - mixed-case contains "aBc" requires display form to be exactly "aBc".
//   - uppercase contains "ABC" requires display form to be "ABC".
func TestVanityMatcher_FastPathSensitive(t *testing.T) {
	makeChecksum := func(display string) ChecksumFunc {
		return func(lower string) string {
			if len(display) != len(lower) {
				t.Fatalf("display length %d != lower length %d", len(display), len(lower))
			}
			return display
		}
	}

	tests := []struct {
		name          string
		contains      string
		mode          VanityMode
		lowerAddr     string
		displayAddr   string
		expectedMatch bool
	}{
		{
			name:          "lowercase contains rejects mixed-case EIP-55",
			contains:      "abc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "aBcdef1234567890",
			expectedMatch: false,
		},
		{
			name:          "lowercase contains accepts all-lowercase EIP-55",
			contains:      "abc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "abcDEF1234567890",
			expectedMatch: true,
		},
		{
			name:          "uppercase contains requires all-uppercase EIP-55",
			contains:      "ABC",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "ABCdef1234567890",
			expectedMatch: true,
		},
		{
			name:          "uppercase contains rejects all-lowercase EIP-55",
			contains:      "ABC",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "abcdef1234567890",
			expectedMatch: false,
		},
		{
			name:          "mixed-case contains requires exact EIP-55 pattern",
			contains:      "aBc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "aBcdef1234567890",
			expectedMatch: true,
		},
		{
			name:          "mixed-case contains rejects different pattern",
			contains:      "aBc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "AbCdef1234567890",
			expectedMatch: false,
		},
		{
			name:          "lowercase prefilter short-circuits before checksum",
			contains:      "xyz",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			displayAddr:   "abcdef1234567890", // checksumFn must not be called; any value works
			expectedMatch: false,
		},
		{
			name:          "suffix mode mixed-case contains",
			contains:      "dEf",
			mode:          VanityModeSuffix,
			lowerAddr:     "1234567890abcdef",
			displayAddr:   "1234567890abcdEf",
			expectedMatch: true,
		},
		{
			name:          "prefix-and-suffix mode both ends verified",
			contains:      "aB",
			mode:          VanityModePrefixAndSuffix,
			lowerAddr:     "ab1234567890ab",
			displayAddr:   "aB1234567890aB",
			expectedMatch: true,
		},
		{
			name:          "prefix-and-suffix mode suffix wrong case rejected",
			contains:      "aB",
			mode:          VanityModePrefixAndSuffix,
			lowerAddr:     "ab1234567890ab",
			displayAddr:   "aB1234567890Ab",
			expectedMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &VanityConfig{
				Contains:      tt.contains,
				Mode:          tt.mode,
				CaseSensitive: true,
				UpperOrLower:  false,
			}
			m := NewVanityMatcher(cfg)
			m.checksumFn = makeChecksum(tt.displayAddr)
			got := m.Match(tt.lowerAddr)
			if got != tt.expectedMatch {
				t.Fatalf("got %v, want %v (contains=%q, lower=%q, display=%q)",
					got, tt.expectedMatch, tt.contains, tt.lowerAddr, tt.displayAddr)
			}
		})
	}
}

// TestVanityMatcher_FastPathInsensitive verifies the `--case insensitive`
// fast-path branch: matches directly against lowercase contains without
// invoking the checksum function (checksumFn must never be called).
func TestVanityMatcher_FastPathInsensitive(t *testing.T) {
	var checksumCalls int
	trackingChecksum := func(lower string) string {
		checksumCalls++
		return lower
	}

	tests := []struct {
		name          string
		contains      string
		mode          VanityMode
		lowerAddr     string
		expectedMatch bool
	}{
		{
			name:          "prefix lowercase match",
			contains:      "abc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			expectedMatch: true,
		},
		{
			name:          "prefix uppercase contains matches lowercase addr",
			contains:      "ABC",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			expectedMatch: true,
		},
		{
			name:          "prefix mixed-case contains normalized to lower",
			contains:      "aBc",
			mode:          VanityModePrefix,
			lowerAddr:     "abcdef1234567890",
			expectedMatch: true,
		},
		{
			name:          "suffix no match",
			contains:      "xyz",
			mode:          VanityModeSuffix,
			lowerAddr:     "1234567890abcdef",
			expectedMatch: false,
		},
		{
			name:          "prefix-or-suffix match at end",
			contains:      "DEF",
			mode:          VanityModePrefixOrSuffix,
			lowerAddr:     "1234567890abcdef",
			expectedMatch: true,
		},
		{
			name:          "prefix-and-suffix both ends",
			contains:      "AB",
			mode:          VanityModePrefixAndSuffix,
			lowerAddr:     "ab1234567890ab",
			expectedMatch: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksumCalls = 0
			cfg := &VanityConfig{
				Contains:      tt.contains,
				Mode:          tt.mode,
				CaseSensitive: false,
				UpperOrLower:  false,
			}
			m := NewVanityMatcher(cfg)
			m.checksumFn = trackingChecksum
			got := m.Match(tt.lowerAddr)
			if got != tt.expectedMatch {
				t.Fatalf("got %v, want %v", got, tt.expectedMatch)
			}
			if checksumCalls != 0 {
				t.Fatalf("checksumFn called %d times in insensitive fast-path; must be 0",
					checksumCalls)
			}
		})
	}
}

func BenchmarkVanityMatcher_Match(b *testing.B) {
	benchmarks := []struct {
		name    string
		config  *VanityConfig
		address string
	}{
		{
			name: "prefix-case-sensitive",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          VanityModePrefix,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address: "abcdef1234567890",
		},
		{
			name: "suffix-case-insensitive",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          VanityModeSuffix,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address: "1234567890abcXYZ",
		},
		{
			name: "prefix-or-suffix-upper-or-lower",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          VanityModePrefixOrSuffix,
				CaseSensitive: false,
				UpperOrLower:  true,
			},
			address: "123abcdef123",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			matcher := NewVanityMatcher(bm.config)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				matcher.Match(bm.address)
			}
		})
	}
}
