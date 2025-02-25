package cmd

import (
	"math/big"
	"testing"
)

func TestBigInt(t *testing.T) {
	MAX_UINT256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	if MAX_UINT256.String() != "115792089237316195423570985008687907853269984665640564039457584007913129639935" {
		t.Errorf("MAX_UINT256 is not correct")
	}
	var seed [32]byte
	MAX_UINT256.FillBytes(seed[:])
	t.Logf("MAX_UINT256: %v", MAX_UINT256)
	t.Logf("seed: %v", seed)
	MAX_UINT256_PLUS_1 := new(big.Int).Add(MAX_UINT256, big.NewInt(1))
	if MAX_UINT256_PLUS_1.String() != "115792089237316195423570985008687907853269984665640564039457584007913129639936" {
		t.Errorf("MAX_UINT256_PLUS_1 is not correct")
	}
	t.Run("expect panic when FillBytes with large number", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but got none")
			}
		}()
		MAX_UINT256_PLUS_1.FillBytes(seed[:])
	})
}
