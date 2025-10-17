package model

import (
	"context"
	"strings"
)

// VanityMatcher performs address matching based on configuration
type VanityMatcher struct {
	config        *VanityConfig
	containsLower string
	containsUpper string
}

// NewVanityMatcher creates a new matcher with the given configuration
func NewVanityMatcher(config *VanityConfig) *VanityMatcher {
	return &VanityMatcher{
		config:        config,
		containsLower: strings.ToLower(config.Contains),
		containsUpper: strings.ToUpper(config.Contains),
	}
}

// Match checks if an address matches the configured criteria
// address should be without 0x prefix for Ethereum addresses
func (m *VanityMatcher) Match(address string) bool {
	// Optimized matching logic
	var passed bool
	if m.config.CaseSensitive {
		// Case sensitive: match exact pattern
		passed = m.matchesCriteria(m.config.Contains, address)
	} else if m.config.UpperOrLower {
		// Upper-or-lower: address must match all uppercase OR all lowercase pattern
		// Do NOT modify address, only match against upper/lower patterns
		passed = m.matchesCriteria(m.containsLower, address) ||
			m.matchesCriteria(m.containsUpper, address)
	} else {
		// Case insensitive: convert address to lowercase and match
		addressLower := strings.ToLower(address)
		passed = m.matchesCriteria(m.containsLower, addressLower)
	}

	return passed
}

// matchesCriteria checks if address matches the contains string based on mode
func (m *VanityMatcher) matchesCriteria(contains string, address string) bool {
	switch m.config.Mode {
	case 1: // prefix
		return len(address) >= len(contains) && address[:len(contains)] == contains
	case 2: // suffix
		return len(address) >= len(contains) && address[len(address)-len(contains):] == contains
	case 3: // prefix or suffix
		return (len(address) >= len(contains) && address[:len(contains)] == contains) ||
			(len(address) >= len(contains) && address[len(address)-len(contains):] == contains)
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
