package ethereum

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

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
		pkBn := new(big.Int).SetBytes(d.privateKeyBytes)

		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order: %x", d.privateKeyBytes)
		}

		// Verify deployer address matches private key
		pk, err := crypto.ToECDSA(d.privateKeyBytes)
		if err != nil {
			t.Fatalf("invalid private key bytes at iteration %d: %v", i, err)
		}
		expectedAddr := crypto.PubkeyToAddress(pk.PublicKey)
		if d.deployerAddress != expectedAddr {
			t.Fatalf("deployer address mismatch at iteration %d", i)
		}

		// Verify contract address matches CREATE1 derivation
		expectedContract := computeCreate1Address(d.deployerAddress)
		if d.contractAddress != expectedContract {
			t.Fatalf("contract address mismatch at iteration %d", i)
		}
	}
}

func TestCreate1AddressGenerator_Deterministic(t *testing.T) {
	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	seed := [32]byte{0x42}

	gen1 := &Create1AddressGenerator{baseSeed: seed, curveOrder: curveOrder}
	gen2 := &Create1AddressGenerator{baseSeed: seed, curveOrder: curveOrder}

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

	gen := &Create1AddressGenerator{baseSeed: seed, curveOrder: curveOrder}
	gen.counter.Store(math.MaxInt64 - 1)

	for i := 0; i < 5; i++ {
		_, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed near MaxInt64 boundary (i=%d): %v", i, err)
		}
		d := data.(*Create1AddressData)
		pkBn := new(big.Int).SetBytes(d.privateKeyBytes)
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

	gen := &Create1AddressGenerator{baseSeed: seed, curveOrder: curveOrder}

	for i := 0; i < 10; i++ {
		_, data, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed at iteration %d: %v", i, err)
		}
		d := data.(*Create1AddressData)
		pkBn := new(big.Int).SetBytes(d.privateKeyBytes)
		if pkBn.Sign() == 0 {
			t.Fatal("private key is zero")
		}
		if pkBn.Cmp(curveOrder) >= 0 {
			t.Fatalf("private key >= curve order at iteration %d", i)
		}
	}
}
