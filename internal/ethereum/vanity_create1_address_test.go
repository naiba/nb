package ethereum

import (
	"bytes"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func BenchmarkCreate1GenerateAndMatch(b *testing.B) {
	gen, err := NewCreate1AddressGenerator()
	if err != nil {
		b.Fatal(err)
	}
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

func TestCreate1AddressGenerator_PrivateKeyValidity(t *testing.T) {
	gen, err := NewCreate1AddressGenerator()
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

		d := data.(*Create1AddressData)
		pkBn := new(big.Int).SetBytes(d.PrivateKeyBytes())

		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order: %x", d.PrivateKeyBytes())
		}

		// 1. Deployer address must match pubkey(priv).
		pk, err := crypto.ToECDSA(d.PrivateKeyBytes())
		if err != nil {
			t.Fatalf("invalid private key bytes at iteration %d: %v", i, err)
		}
		expectedDeployer := crypto.PubkeyToAddress(pk.PublicKey)
		if common.Address(d.deployerAddrBytes) != expectedDeployer {
			t.Fatalf("deployer address mismatch at iteration %d: got %x, want %s",
				i, d.deployerAddrBytes, expectedDeployer.Hex())
		}

		// 2. Contract address must match CREATE1 derivation.
		expectedContract := computeCreate1AddressBytes(d.deployerAddrBytes)
		if d.contractAddrBytes != expectedContract {
			t.Fatalf("contract address mismatch at iteration %d", i)
		}

		// 3. Signature round-trip: the generated key must actually be usable.
		msg := []byte("create1 test message")
		digest := crypto.Keccak256(msg)
		sig, err := crypto.Sign(digest, pk)
		if err != nil {
			t.Fatalf("Sign failed at iteration %d: %v", i, err)
		}
		recovered, err := crypto.SigToPub(digest, sig)
		if err != nil {
			t.Fatalf("SigToPub failed at iteration %d: %v", i, err)
		}
		if crypto.PubkeyToAddress(*recovered) != expectedDeployer {
			t.Fatalf("signature recovery mismatch at iteration %d", i)
		}
	}
}

// TestCreate1_ManualRLPMatchesLibrary cross-checks our hand-rolled RLP against
// go-ethereum's rlp.EncodeToBytes. The RLP encoding of [address, 0] has a
// single valid form, so byte-for-byte equality is the right check.
func TestCreate1_ManualRLPMatchesLibrary(t *testing.T) {
	addrs := [][20]byte{
		{},
		{0x01},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0xde, 0xad, 0xbe, 0xef, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
	}
	for _, a := range addrs {
		// Manual encoding: 0xd6 | 0x94 | addr[0:20] | 0x80
		var manual [23]byte
		manual[0] = 0xd6
		manual[1] = 0x94
		copy(manual[2:22], a[:])
		manual[22] = 0x80

		// Library encoding
		ref, err := rlp.EncodeToBytes([]interface{}{common.Address(a), uint64(0)})
		if err != nil {
			t.Fatalf("rlp encode failed: %v", err)
		}

		if !bytes.Equal(manual[:], ref) {
			t.Fatalf("mismatch for %x:\n manual=%x\n lib   =%x", a, manual, ref)
		}

		// And the derived address must match too.
		expected := crypto.Keccak256Hash(ref).Bytes()[12:]
		got := computeCreate1AddressBytes(a)
		if !bytes.Equal(got[:], expected) {
			t.Fatalf("derived address mismatch for %x", a)
		}
	}
}

func TestCreate1AddressGenerator_Deterministic(t *testing.T) {
	seed := [32]byte{0x42}

	gen1 := newCreate1AddressGeneratorFromSeed(seed)
	gen2 := newCreate1AddressGeneratorFromSeed(seed)

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

func TestCreate1AddressGenerator_CounterOverflowBoundary(t *testing.T) {
	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	seed := [32]byte{0xab, 0xcd}

	gen := newCreate1AddressGeneratorFromSeed(seed)
	gen.counter.Store(math.MaxInt64 - 1)

	for i := 0; i < 5; i++ {
		_, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed near MaxInt64 boundary (i=%d): %v", i, err)
		}
		d := data.(*Create1AddressData)
		pkBn := new(big.Int).SetBytes(d.PrivateKeyBytes())
		if pkBn.Sign() == 0 {
			t.Fatalf("private key is zero at i=%d", i)
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order at i=%d", i)
		}
	}
}

func TestCreate1AddressGenerator_SeedNearCurveOrder(t *testing.T) {
	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

	nearOrder := new(big.Int).Sub(curveOrder, big.NewInt(2))
	var seed [32]byte
	nearOrder.FillBytes(seed[:])

	gen := newCreate1AddressGeneratorFromSeed(seed)

	for i := 0; i < 10; i++ {
		_, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		d := data.(*Create1AddressData)
		pkBn := new(big.Int).SetBytes(d.PrivateKeyBytes())
		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order at iteration %d", i)
		}
	}
}
