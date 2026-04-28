package ethereum

import (
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"
)

// validateHexContains rejects any character outside the Ethereum hex alphabet.
// Empty input is accepted (callers may bit-mask instead of prefix-match).
func validateHexContains(s string) error {
	const validHexChars = "0123456789abcdefABCDEF"
	for _, c := range s {
		if !strings.ContainsRune(validHexChars, c) {
			return fmt.Errorf("contains illegal character: %c (Ethereum addresses only contain 0-9, a-f, A-F)", c)
		}
	}
	return nil
}

// logSearchEstimate logs a rough upper-bound search time for a keyspace of
// `space` addresses with `threads` workers, capped at 100 years.
func logSearchEstimate(space *big.Int, threads int) {
	if threads < 1 {
		threads = 1
	}
	// ~23 seconds per 10M keys per thread on a 2.6 GHz 6-Core i7 (kept as the
	// historical reference point so the log line stays comparable across runs).
	estimateSeconds := new(big.Int).Mul(
		new(big.Int).Div(space, big.NewInt(int64(threads*10000000))),
		big.NewInt(23),
	)
	hundredYearsSec := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(hundredYearsSec) > 0 {
		estimateSeconds = hundredYearsSec
	}
	estimate := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Search space: %v addresses, estimated max time: %v (2.6 GHz 6-Core Intel Core i7)", space, estimate)
}
