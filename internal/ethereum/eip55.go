package ethereum

import "github.com/ethereum/go-ethereum/crypto"

// EIP55Checksum converts a 40-char lowercase hex address (no 0x prefix) into
// its EIP-55 mixed-case form. Used by the vanity matcher to defer per-candidate
// checksum computation until a lowercase prefilter passes.
//
// Algorithm: keccak256 the lowercase hex bytes; uppercase each 'a'..'f' whose
// corresponding hash nibble is >= 8.
func EIP55Checksum(lowerHex string) string {
	if len(lowerHex) != 40 {
		return lowerHex
	}
	hash := crypto.Keccak256([]byte(lowerHex))
	buf := make([]byte, 40)
	for i := 0; i < 40; i++ {
		c := lowerHex[i]
		var nibble byte
		if i&1 == 0 {
			nibble = hash[i/2] >> 4
		} else {
			nibble = hash[i/2] & 0x0f
		}
		if c >= 'a' && c <= 'f' && nibble >= 8 {
			c -= 32 // to upper
		}
		buf[i] = c
	}
	return string(buf)
}
