package tron

import (
	"crypto/sha256"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
	"github.com/naiba/nb/model"
)

func BenchmarkTronGenerateAndMatch(b *testing.B) {
	gen, err := NewTronAddressGenerator()
	if err != nil {
		b.Fatal(err)
	}
	cfg := &model.VanityConfig{Contains: "TzzzzzZ", Mode: model.VanityModePrefixOrSuffix, CaseSensitive: false}
	matcher := model.NewVanityMatcher(cfg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr, _, err := gen.Generate()
		if err != nil {
			b.Fatal(err)
		}
		_ = matcher.Match(addr)
	}
}

func TestTronAddressGenerator_AddressValidity(t *testing.T) {
	gen, err := NewTronAddressGenerator()
	if err != nil {
		t.Fatal(err)
	}

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

		decoded, err := base58.Decode(addr)
		if err != nil {
			t.Fatalf("address not valid Base58 at iteration %d: %v", i, err)
		}
		// 1 byte prefix (0x41) + 20 bytes address + 4 bytes checksum = 25 bytes
		if len(decoded) != 25 {
			t.Fatalf("decoded address length = %d, want 25", len(decoded))
		}
		if decoded[0] != 0x41 {
			t.Fatalf("first byte = 0x%02x, want 0x41", decoded[0])
		}

		// Checksum must validate (double SHA-256, take first 4 bytes).
		h1 := sha256.Sum256(decoded[:21])
		h2 := sha256.Sum256(h1[:])
		for j := 0; j < 4; j++ {
			if decoded[21+j] != h2[j] {
				t.Fatalf("checksum byte %d mismatch at iteration %d: got %02x want %02x",
					j, i, decoded[21+j], h2[j])
			}
		}

		d := data.(*TronAddressData)
		pkBytes := d.PrivateKeyBytes()
		pkBn := new(big.Int).SetBytes(pkBytes)

		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order: %x", pkBytes)
		}

		// 1. Address must match pubkey-derived hash: keccak256(X||Y)[12:] == decoded[1:21].
		pk, err := crypto.ToECDSA(pkBytes)
		if err != nil {
			t.Fatalf("invalid private key at iteration %d: %v", i, err)
		}
		pubBytes := crypto.FromECDSAPub(&pk.PublicKey)[1:]
		hash := crypto.Keccak256(pubBytes)
		addrFromKey := hash[len(hash)-20:]
		for j := 0; j < 20; j++ {
			if decoded[1+j] != addrFromKey[j] {
				t.Fatalf("address byte mismatch at position %d, iteration %d", j, i)
			}
		}

		// 2. Signature round-trip verifies the key is actually usable.
		msg := []byte("tron vanity signing test")
		digest := crypto.Keccak256(msg)
		sig, err := crypto.Sign(digest, pk)
		if err != nil {
			t.Fatalf("Sign failed at iteration %d: %v", i, err)
		}
		recovered, err := crypto.Ecrecover(digest, sig)
		if err != nil {
			t.Fatalf("Ecrecover failed at iteration %d: %v", i, err)
		}
		expectedPub := crypto.FromECDSAPub(&pk.PublicKey)
		if string(recovered) != string(expectedPub) {
			t.Fatalf("recovered pubkey != signer at iteration %d", i)
		}
	}
}

func TestTronAddressGenerator_Deterministic(t *testing.T) {
	seed := [32]byte{0x07, 0x07}

	gen1 := NewTronAddressGeneratorFromSeed(seed)
	gen2 := NewTronAddressGeneratorFromSeed(seed)

	for i := 0; i < 100; i++ {
		addr1, _, err1 := gen1.Generate()
		addr2, _, err2 := gen2.Generate()
		if err1 != nil || err2 != nil {
			t.Fatalf("Generate() error: %v, %v", err1, err2)
		}
		if addr1 != addr2 {
			t.Fatalf("iteration %d: addresses differ: %s vs %s", i, addr1, addr2)
		}
	}
}
