package ethereum

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
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
		pkBytes := crypto.FromECDSA(d.privateKey)
		pkBn := new(big.Int).SetBytes(pkBytes)

		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order: %x", pkBytes)
		}
	}
}

func TestEthereumAddressGenerator_Deterministic(t *testing.T) {
	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	seed := [32]byte{0x01}

	gen1 := &EthereumAddressGenerator{baseSeed: seed, curveOrder: curveOrder}
	gen2 := &EthereumAddressGenerator{baseSeed: seed, curveOrder: curveOrder}

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
	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	seed := [32]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	gen := &EthereumAddressGenerator{baseSeed: seed, curveOrder: curveOrder}
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

	gen := &EthereumAddressGenerator{baseSeed: seed, curveOrder: curveOrder}

	for i := 0; i < 10; i++ {
		_, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		d := data.(*EthereumAddressData)
		pkBn := new(big.Int).SetBytes(crypto.FromECDSA(d.privateKey))
		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order at iteration %d", i)
		}
	}
}
