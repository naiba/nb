package tron

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
)

func TestTronAddressGenerator_AddressValidity(t *testing.T) {
	gen := &TronAddressGenerator{}

	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

	for i := 0; i < 1000; i++ {
		addr, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}

		// Tron mainnet addresses always start with 'T'
		if !strings.HasPrefix(addr, "T") {
			t.Fatalf("address doesn't start with T at iteration %d: %s", i, addr)
		}

		// Verify Base58 decodable
		decoded, err := base58.Decode(addr)
		if err != nil {
			t.Fatalf("address not valid Base58 at iteration %d: %v", i, err)
		}
		// 1 byte prefix (0x41) + 20 bytes address + 4 bytes checksum = 25 bytes
		if len(decoded) != 25 {
			t.Fatalf("decoded address length = %d, want 25", len(decoded))
		}
		// First byte must be 0x41 (Tron mainnet)
		if decoded[0] != 0x41 {
			t.Fatalf("first byte = 0x%02x, want 0x41", decoded[0])
		}

		d := data.(*TronAddressData)
		pkBytes := crypto.FromECDSA(d.privateKey)
		pkBn := new(big.Int).SetBytes(pkBytes)

		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order: %x", pkBytes)
		}

		// Verify address matches the private key by re-deriving
		pubBytes := crypto.FromECDSAPub(&d.privateKey.PublicKey)[1:]
		hash := crypto.Keccak256(pubBytes)
		addrBytes := hash[len(hash)-20:]
		if decoded[0] != 0x41 {
			t.Fatal("prefix mismatch")
		}
		for j := 0; j < 20; j++ {
			if decoded[1+j] != addrBytes[j] {
				t.Fatalf("address byte mismatch at position %d, iteration %d", j, i)
			}
		}
	}
}
