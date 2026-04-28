package ethereum

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// A minimal deployer + near-empty init code: enough to exercise the hot path
// without pulling in ABI encoding.
func newCreate2BenchGenerator(t testing.TB) *Create2AddressGenerator {
	gen, err := NewCreate2AddressGenerator(
		"0x4e59b44847b379578588920ca78fbf26c0b4956c", // well-known CREATE2 deployer
		"",
		"0x00",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	return gen
}

func BenchmarkCreate2GenerateAndMatch(b *testing.B) {
	gen := newCreate2BenchGenerator(b)
	matcher := benchMatcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr, _, err := gen.Generate()
		if err != nil {
			b.Fatal(err)
		}
		_ = matcher.Match(addr)
	}
}

// TestCreate2AddressGenerator_DerivationCorrect verifies the hot-path address
// matches the reference EIP-1014 formula implemented in go-ethereum:
//
//	keccak256(0xff || deployer || salt || initCodeHash)[12:]
func TestCreate2AddressGenerator_DerivationCorrect(t *testing.T) {
	deployer := "0x4e59b44847b379578588920ca78fbf26c0b4956c"
	gen, err := NewCreate2AddressGenerator(deployer, "prefix", "0x6080604052348015", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Capture init code hash from the template so we can re-derive the address.
	deployerAddr := common.HexToAddress(deployer)
	initCodeHash := make([]byte, 32)
	copy(initCodeHash, gen.hashInputTemplate[53:85])

	for i := 0; i < 1000; i++ {
		addrHex, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		if len(addrHex) != 40 {
			t.Fatalf("address length = %d, want 40", len(addrHex))
		}

		d := data.(*Create2AddressData)

		// Re-derive from stored salt string via the canonical go-ethereum helper.
		saltHash := crypto.Keccak256([]byte(d.SaltString()))
		var saltBytes [32]byte
		copy(saltBytes[:], saltHash)
		expected := crypto.CreateAddress2(deployerAddr, saltBytes, initCodeHash)

		if common.Address(d.AddressBytes()) != expected {
			t.Fatalf("iter %d: got %x, want %s", i, d.AddressBytes(), expected.Hex())
		}

		// The returned hex must decode to the same address.
		if common.HexToAddress(addrHex) != expected {
			t.Fatalf("iter %d: returned hex %s != %s", i, addrHex, expected.Hex())
		}

		// Salt string must deterministically round-trip through our numeric suffix.
		if d.saltSuffix != uint64(i) {
			t.Fatalf("iter %d: salt suffix = %d", i, d.saltSuffix)
		}
	}
}

func TestCreate2AddressGenerator_SaltPrefixed(t *testing.T) {
	// With a prefix set, the salt string must start with it.
	gen, err := NewCreate2AddressGenerator(
		"0x4e59b44847b379578588920ca78fbf26c0b4956c",
		"myPrefix_",
		"0x00",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		_, data, err := gen.Generate()
		if err != nil {
			t.Fatal(err)
		}
		d := data.(*Create2AddressData)
		if d.saltStr[:len("myPrefix_")] != "myPrefix_" {
			t.Fatalf("salt %q doesn't start with prefix", d.saltStr)
		}
	}
}
