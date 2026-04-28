package ethereum

import (
	"context"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/naiba/nb/model"
)

func TestEthereumAddressGenerator_PrivateKeyValidity(t *testing.T) {
	gen, err := NewEthereumAddressGenerator()
	if err != nil {
		t.Fatal(err)
	}

	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

	for i := 0; i < 1000; i++ {
		addr, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		if len(addr) != 40 {
			t.Fatalf("address length = %d, want 40", len(addr))
		}

		d := data.(*EthereumAddressData)
		pkBytes := d.seed[:]
		pkBn := new(big.Int).SetBytes(pkBytes)

		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order: %x", pkBytes)
		}

		// Address must match pubkey derivation from seed, and signing must work.
		pk, err := crypto.ToECDSA(pkBytes)
		if err != nil {
			t.Fatalf("invalid private key at iter %d: %v", i, err)
		}
		expected := crypto.PubkeyToAddress(pk.PublicKey)
		if common.Address(d.addrBytes) != expected {
			t.Fatalf("address mismatch at iter %d: got %x, want %s", i, d.addrBytes, expected.Hex())
		}
		// Display address must be EIP-55 of the same 20 bytes.
		if d.Address() != expected.Hex() {
			t.Fatalf("Address() = %s, want %s", d.Address(), expected.Hex())
		}

		if i < 10 {
			// Signature round-trip on a sample — proves the generated key is usable.
			digest := crypto.Keccak256([]byte("eoa signing test"))
			sig, err := crypto.Sign(digest, pk)
			if err != nil {
				t.Fatalf("Sign failed: %v", err)
			}
			recovered, err := crypto.SigToPub(digest, sig)
			if err != nil {
				t.Fatalf("SigToPub failed: %v", err)
			}
			if crypto.PubkeyToAddress(*recovered) != expected {
				t.Fatalf("signature recovery mismatch at iter %d", i)
			}
		}
	}
}

// TestEOAIntegration_SearchProducesValidMatch exercises the full matcher +
// searcher path end-to-end: pick a 2-character prefix, run the real search,
// verify the returned address matches the pattern, and verify the private key
// re-derives to the same address.
func TestEOAIntegration_SearchProducesValidMatch(t *testing.T) {
	gen, err := NewEthereumAddressGenerator()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &model.VanityConfig{
		Contains:      "ab",
		Mode:          model.VanityModePrefix,
		CaseSensitive: false,
		Threads:       1,
	}
	searcher := model.NewVanitySearcher(cfg, gen).WithChecksum(EIP55Checksum)

	res, err := searcher.Search(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	lower := strings.ToLower(res.Address)
	if !strings.HasPrefix(lower, "ab") {
		t.Fatalf("returned address %q doesn't start with 'ab'", res.Address)
	}

	d := res.Data.(*EthereumAddressData)
	pk, err := d.PrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	expected := crypto.PubkeyToAddress(pk.PublicKey).Hex()
	if d.Address() != expected {
		t.Fatalf("Data.Address() = %s, priv-derived = %s", d.Address(), expected)
	}
	if !strings.HasPrefix(strings.ToLower(expected[2:]), "ab") {
		t.Fatalf("priv-derived address %s doesn't match pattern", expected)
	}
}

// TestEOAIntegration_CaseSensitiveMatchUsesChecksum verifies the lazy EIP-55
// path: a mixed-case contains forces the matcher to recompute the checksum
// for preliminary hits. Use a 1-character uppercase contains so we land on
// a case-sensitive hit quickly.
func TestEOAIntegration_CaseSensitiveMatchUsesChecksum(t *testing.T) {
	gen, err := NewEthereumAddressGenerator()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &model.VanityConfig{
		Contains:      "A", // uppercase: requires EIP-55 first nibble
		Mode:          model.VanityModePrefix,
		CaseSensitive: true,
		Threads:       1,
	}
	searcher := model.NewVanitySearcher(cfg, gen).WithChecksum(EIP55Checksum)

	res, err := searcher.Search(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	d := res.Data.(*EthereumAddressData)
	// Display address (EIP-55) must have 'A' at position 2 (after "0x").
	if d.Address()[2] != 'A' {
		t.Fatalf("case-sensitive match failed: %s", d.Address())
	}
}

func TestEthereumAddressGenerator_Deterministic(t *testing.T) {
	seed := [32]byte{0x01}

	gen1 := newEthereumAddressGeneratorFromSeed(seed)
	gen2 := newEthereumAddressGeneratorFromSeed(seed)

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

func TestEthereumAddressGenerator_CounterOverflowBoundary(t *testing.T) {
	seed := [32]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	gen := newEthereumAddressGeneratorFromSeed(seed)
	// Simulate counter near uint64 int64 boundary
	gen.counter.Store(math.MaxInt64 - 1)

	for i := 0; i < 5; i++ {
		_, _, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed near MaxInt64 boundary (i=%d): %v", i, err)
		}
	}
}

func TestEthereumAddressGenerator_SeedNearCurveOrder(t *testing.T) {
	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

	// baseSeed = curveOrder - 2, so after mod (n-1) and +1 the key wraps around
	nearOrder := new(big.Int).Sub(curveOrder, big.NewInt(2))
	var seed [32]byte
	nearOrder.FillBytes(seed[:])

	gen := newEthereumAddressGeneratorFromSeed(seed)

	for i := 0; i < 10; i++ {
		_, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		d := data.(*EthereumAddressData)
		pkBn := new(big.Int).SetBytes(d.seed[:])
		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order at iteration %d", i)
		}
	}
}

// BenchmarkGenerate measures the cost of producing a single EOA address
// (seed derivation + ECDSA pubkey derivation + EIP-55 checksum encoding).
func BenchmarkGenerate(b *testing.B) {
	gen, err := NewEthereumAddressGenerator()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := gen.Generate(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateAndMatch measures the true mining hot loop: generate an
// address then run the matcher. Use an impossible target so every iteration
// exercises the full path and the loop never exits early.
func BenchmarkGenerateAndMatch(b *testing.B) {
	gen, err := NewEthereumAddressGenerator()
	if err != nil {
		b.Fatal(err)
	}
	cfg := &model.VanityConfig{
		Contains:      "zzzzzz", // never matches (non-hex)
		Mode:          model.VanityModePrefixOrSuffix,
		CaseSensitive: false,
		UpperOrLower:  false,
	}
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

// BenchmarkGenerateAndMatchParallel measures multi-threaded scaling with a
// per-goroutine generator (no shared atomic counter). Run with -cpu=1,4,8,14
// to see the scaling curve.
func BenchmarkGenerateAndMatchParallel(b *testing.B) {
	cfg := &model.VanityConfig{
		Contains:      "zzzzzz",
		Mode:          model.VanityModePrefixOrSuffix,
		CaseSensitive: false,
		UpperOrLower:  false,
	}
	b.RunParallel(func(pb *testing.PB) {
		gen, err := NewEthereumAddressGenerator()
		if err != nil {
			b.Fatal(err)
		}
		matcher := model.NewVanityMatcher(cfg)
		for pb.Next() {
			addr, _, err := gen.Generate()
			if err != nil {
				b.Fatal(err)
			}
			_ = matcher.Match(addr)
		}
	})
}

// BenchmarkGenerateAndMatchBitmask exercises the bitmask branch (hex.DecodeString
// on every candidate), which is the V4 hook mining path.
func BenchmarkGenerateAndMatchBitmask(b *testing.B) {
	gen, err := NewEthereumAddressGenerator()
	if err != nil {
		b.Fatal(err)
	}
	cfg := &model.VanityConfig{
		Mode:          model.VanityModePrefix,
		CaseSensitive: true,
		Mask:          []byte{0xFF, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3F, 0xFF},
		MaskValue:     []byte{0xB0, 0xB0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x80},
	}
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
