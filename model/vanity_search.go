package model

import (
	"context"
	"encoding/hex"
	"strings"
)

// ChecksumFunc converts a lowercase address (e.g. hex for Ethereum) into its
// display-cased form (e.g. EIP-55). If the matcher has one set, it treats the
// incoming address as lowercase and defers the checksum computation until a
// lowercase prefilter passes — avoiding the cost on every candidate.
//
// Ethereum generators set this; chains whose encoding is inherently
// case-sensitive (base58 etc.) leave it nil and feed the address as-is.
type ChecksumFunc func(lowerHex string) string

// VanityMatcher performs address matching based on configuration
type VanityMatcher struct {
	config        *VanityConfig
	containsLower string
	containsUpper string
	checksumFn    ChecksumFunc
}

// NewVanityMatcher creates a new matcher with the given configuration
func NewVanityMatcher(config *VanityConfig) *VanityMatcher {
	return &VanityMatcher{
		config:        config,
		containsLower: strings.ToLower(config.Contains),
		containsUpper: strings.ToUpper(config.Contains),
	}
}

func (m *VanityMatcher) Match(address string) bool {
	if m.config.Mask != nil {
		addrBytes, err := hex.DecodeString(address)
		if err != nil || len(addrBytes) != 20 {
			return false
		}
		for i := range m.config.Mask {
			if addrBytes[i]&m.config.Mask[i] != m.config.MaskValue[i] {
				return false
			}
		}
		if m.config.Contains == "" {
			return true
		}
	}

	// Fast path when the generator promised lowercase input + provided a
	// checksum function. We can always match case-insensitively (directly,
	// no ToLower copy), and only compute the checksum when we actually need
	// case-sensitive verification of mixed-case contains.
	if m.checksumFn != nil {
		switch {
		case !m.config.CaseSensitive:
			return m.matchesCriteria(m.containsLower, address)
		case m.config.UpperOrLower:
			// "either" 语义：EIP-55 形式下匹配段必须全大写或全小写（非混合）。
			// 先用 lowercase 预过滤（字母位必须命中目标字符），再拿 EIP-55 形式
			// 分别与 containsLower / containsUpper 做字面比较——任一命中即证明
			// 该段字母全为同一 case。纯数字的 contains 两侧都会命中，等价于
			// 字面匹配，符合预期。
			// 老代码里这个分支在 CS=true 时不可达（if/else if），事实等同 sensitive；
			// 修正后 sensitive < either < insensitive 严格度单调。
			if !m.matchesCriteria(m.containsLower, address) {
				return false
			}
			checksum := m.checksumFn(address)
			return m.matchesCriteria(m.containsLower, checksum) ||
				m.matchesCriteria(m.containsUpper, checksum)
		default:
			// CaseSensitive: the *displayed* (EIP-55) form must contain
			// config.Contains verbatim. Do a cheap lowercase prefilter first,
			// then verify the checksum form on a hit. This applies equally to
			// all-lowercase and mixed-case contains: for contains="abc" we
			// must still reject addresses whose EIP-55 renders as "aBc", so
			// the fast-path that skipped the checksum check for lowercase-only
			// contains would silently accept false matches (see git history).
			if !m.matchesCriteria(m.containsLower, address) {
				return false
			}
			return m.matchesCriteria(m.config.Contains, m.checksumFn(address))
		}
	}

	var passed bool
	if m.config.CaseSensitive {
		passed = m.matchesCriteria(m.config.Contains, address)
	} else if m.config.UpperOrLower {
		passed = m.matchesCriteria(m.containsLower, address) ||
			m.matchesCriteria(m.containsUpper, address)
	} else {
		addressLower := strings.ToLower(address)
		passed = m.matchesCriteria(m.containsLower, addressLower)
	}

	return passed
}

// matchesCriteria checks if address matches the contains string based on mode
func (m *VanityMatcher) matchesCriteria(contains string, address string) bool {
	switch m.config.Mode {
	case VanityModePrefix:
		return len(address) >= len(contains) && address[:len(contains)] == contains
	case VanityModeSuffix:
		return len(address) >= len(contains) && address[len(address)-len(contains):] == contains
	case VanityModePrefixOrSuffix:
		return (len(address) >= len(contains) && address[:len(contains)] == contains) ||
			(len(address) >= len(contains) && address[len(address)-len(contains):] == contains)
	case VanityModePrefixAndSuffix:
		// address length must be >= 2 * len(contains) to avoid prefix/suffix overlap.
		// Otherwise "aaa" with contains="aa" would spuriously match as both prefix
		// (indices 0..1) and suffix (indices 1..2), since the two windows share index 1.
		return len(address) >= 2*len(contains) &&
			address[:len(contains)] == contains &&
			address[len(address)-len(contains):] == contains
	default:
		return false
	}
}

// AddressGenerator is an interface for generating addresses
type AddressGenerator interface {
	// Generate generates a new address and returns the address string and any associated data
	Generate() (address string, data interface{}, err error)
}

// VanitySearcher performs vanity address search with multiple threads
type VanitySearcher struct {
	config    *VanityConfig
	matcher   *VanityMatcher
	generator AddressGenerator
}

// NewVanitySearcher creates a new searcher
func NewVanitySearcher(config *VanityConfig, generator AddressGenerator) *VanitySearcher {
	return &VanitySearcher{
		config:    config,
		matcher:   NewVanityMatcher(config),
		generator: generator,
	}
}

// WithChecksum wires in a checksum function so the matcher can treat the
// generator's output as lowercase and compute the display form only on hits.
// Chainable: returns the searcher.
func (s *VanitySearcher) WithChecksum(fn ChecksumFunc) *VanitySearcher {
	s.matcher.checksumFn = fn
	return s
}

// VanityResult holds the result of a vanity search
type VanityResult struct {
	Address string
	Data    interface{}
}

// Search performs the vanity address search
func (s *VanitySearcher) Search(ctx context.Context) (*VanityResult, error) {
	result := make(chan *VanityResult, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start worker goroutines
	for i := 0; i < s.config.Threads; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					address, data, err := s.generator.Generate()
					if err != nil {
						continue
					}

					if s.matcher.Match(address) {
						select {
						case result <- &VanityResult{
							Address: address,
							Data:    data,
						}:
							cancel() // Notify other goroutines to exit
						default: // Prevent deadlock
						}
						return
					}
				}
			}
		}()
	}

	// Wait for result
	select {
	case res := <-result:
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
