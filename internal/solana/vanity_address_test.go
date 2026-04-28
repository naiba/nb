package solana

import (
	"crypto/ed25519"
	"math"
	"testing"

	"github.com/mr-tron/base58"
	"github.com/naiba/nb/model"
)

func BenchmarkSolanaGenerateAndMatch(b *testing.B) {
	gen, err := NewSolanaAddressGenerator()
	if err != nil {
		b.Fatal(err)
	}
	cfg := &model.VanityConfig{Contains: "zzzzzz", Mode: model.VanityModePrefixOrSuffix, CaseSensitive: false}
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

func TestSolanaAddressGenerator_AddressValidity(t *testing.T) {
	gen, err := NewSolanaAddressGenerator()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 1000; i++ {
		addr, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}

		pubBytes, err := base58.Decode(addr)
		if err != nil {
			t.Fatalf("address not valid Base58 at iteration %d: %v", i, err)
		}
		if len(pubBytes) != ed25519.PublicKeySize {
			t.Fatalf("public key length = %d, want %d", len(pubBytes), ed25519.PublicKeySize)
		}

		d := data.(*SolanaAddressData)
		if len(d.privateKey) != ed25519.PrivateKeySize {
			t.Fatalf("private key length = %d, want %d", len(d.privateKey), ed25519.PrivateKeySize)
		}

		// 1. Public half of the private key must match the address.
		derivedAddr := base58.Encode(d.privateKey[32:])
		if derivedAddr != addr {
			t.Fatalf("address mismatch at iteration %d: %s vs %s", i, addr, derivedAddr)
		}

		// 2. Seed-only re-derivation: ed25519.NewKeyFromSeed(priv[0:32]) must
		//    reproduce the exact private key. This proves the first 32 bytes
		//    we treat as "seed" are actually the seed that produced the key
		//    (any future refactor that drops the seed or swaps bytes fails this).
		reDerived := ed25519.NewKeyFromSeed(d.privateKey[:32])
		for j := range d.privateKey {
			if reDerived[j] != d.privateKey[j] {
				t.Fatalf("seed re-derivation mismatch at iteration %d byte %d", i, j)
			}
		}

		// 3. Signing must work.
		msg := []byte("solana vanity signing test")
		sig := ed25519.Sign(d.privateKey, msg)
		pubKey := d.privateKey.Public().(ed25519.PublicKey)
		if !ed25519.Verify(pubKey, msg, sig) {
			t.Fatalf("signature verification failed at iteration %d", i)
		}
	}
}

func TestSolanaAddressGenerator_Deterministic(t *testing.T) {
	seed := [32]byte{0xde, 0xad}

	gen1 := newSolanaAddressGeneratorFromSeed(seed)
	gen2 := newSolanaAddressGeneratorFromSeed(seed)

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

func TestSolanaAddressGenerator_CounterOverflowBoundary(t *testing.T) {
	seed := [32]byte{0x01, 0x02, 0x03}
	gen := newSolanaAddressGeneratorFromSeed(seed)
	gen.counter.Store(math.MaxInt64 - 1)

	for i := 0; i < 5; i++ {
		addr, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed near MaxInt64 boundary (i=%d): %v", i, err)
		}
		if addr == "" {
			t.Fatalf("empty address at i=%d", i)
		}
		d := data.(*SolanaAddressData)
		if len(d.privateKey) != ed25519.PrivateKeySize {
			t.Fatalf("invalid private key length at i=%d", i)
		}
	}
}

func TestSolanaAddressGenerator_MaxSeed(t *testing.T) {
	// baseSeed = all 0xff — any 32 bytes are valid for ed25519, including the
	// maximum. Generation must still produce a usable key.
	var seed [32]byte
	for i := range seed {
		seed[i] = 0xff
	}

	gen := newSolanaAddressGeneratorFromSeed(seed)

	for i := 0; i < 10; i++ {
		addr, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		if addr == "" {
			t.Fatalf("empty address at iteration %d", i)
		}
		d := data.(*SolanaAddressData)

		msg := []byte("verify")
		sig := ed25519.Sign(d.privateKey, msg)
		pubKey := d.privateKey.Public().(ed25519.PublicKey)
		if !ed25519.Verify(pubKey, msg, sig) {
			t.Fatalf("signature verification failed at iteration %d", i)
		}
	}
}
