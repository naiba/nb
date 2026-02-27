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
		// Prefix mode tests (mode = 1)
		{
			name: "prefix mode - exact match at start",
			config: &VanityConfig{
				Contains:      "abc",
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "abcdef1234567890",
			expectedMatch: false,
		},

		// Suffix mode tests (mode = 2)
		{
			name: "suffix mode - exact match at end",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          2,
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
				Mode:          2,
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
				Mode:          2,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address:       "1234567890abcXYZ",
			expectedMatch: true,
		},

		// Prefix-or-suffix mode tests (mode = 3)
		{
			name: "prefix-or-suffix mode - match at start",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          3,
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
				Mode:          3,
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
				Mode:          3,
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
				Mode:          3,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          2,
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
				Mode:          2,
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
				Mode:          2,
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
				Mode:          1,
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
				Mode:          3,
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
				Mode:          3,
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
				Mode:          3,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          2,
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
				Mode:          1,
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
				Mode:          2,
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
				Mode:          1,
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
				Mode:          2,
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
				Mode:          1,
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
				Mode:          2,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
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
				Mode:          1,
				CaseSensitive: true,
				Mask:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
				MaskValue:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
			},
			address:       "A0A7429Ea01F76102f053213463D4e95D5D02280",
			expectedMatch: false,
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
				Mode:          1,
				CaseSensitive: true,
				UpperOrLower:  false,
			},
			address: "abcdef1234567890",
		},
		{
			name: "suffix-case-insensitive",
			config: &VanityConfig{
				Contains:      "xyz",
				Mode:          2,
				CaseSensitive: false,
				UpperOrLower:  false,
			},
			address: "1234567890abcXYZ",
		},
		{
			name: "prefix-or-suffix-upper-or-lower",
			config: &VanityConfig{
				Contains:      "123",
				Mode:          3,
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
