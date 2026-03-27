package solana

import (
	"crypto/ed25519"
	"math"
	"math/big"
	"testing"

	"github.com/mr-tron/base58"
)

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

		// Verify Base58 decodable and 32 bytes (Ed25519 public key)
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

		// Verify the public key half of the private key matches the address
		derivedAddr := base58.Encode(d.privateKey[32:])
		if derivedAddr != addr {
			t.Fatalf("address mismatch at iteration %d: %s vs %s", i, addr, derivedAddr)
		}

		// Verify signing works with the generated key
		msg := []byte("test message")
		sig := ed25519.Sign(d.privateKey, msg)
		pubKey := d.privateKey.Public().(ed25519.PublicKey)
		if !ed25519.Verify(pubKey, msg, sig) {
			t.Fatalf("signature verification failed at iteration %d", i)
		}
	}
}

func TestSolanaAddressGenerator_Deterministic(t *testing.T) {
	seed := [32]byte{0xde, 0xad}

	gen1 := &SolanaAddressGenerator{baseSeed: seed}
	gen2 := &SolanaAddressGenerator{baseSeed: seed}

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
	gen := &SolanaAddressGenerator{baseSeed: seed}
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
	// baseSeed = max uint256 value, FillBytes truncates to 32 bytes
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	var seed [32]byte
	maxUint256.FillBytes(seed[:])

	gen := &SolanaAddressGenerator{baseSeed: seed}

	for i := 0; i < 10; i++ {
		addr, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		if addr == "" {
			t.Fatalf("empty address at iteration %d", i)
		}
		d := data.(*SolanaAddressData)

		// Ed25519 accepts any 32-byte seed, so even overflowed values should work
		msg := []byte("verify")
		sig := ed25519.Sign(d.privateKey, msg)
		pubKey := d.privateKey.Public().(ed25519.PublicKey)
		if !ed25519.Verify(pubKey, msg, sig) {
			t.Fatalf("signature verification failed at iteration %d", i)
		}
	}
}
