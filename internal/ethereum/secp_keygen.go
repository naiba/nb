package ethereum

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"sync/atomic"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// Cached at package init to avoid per-call lookups and the
// (*BitCurve).Params allocation that shows up in pprof.
//
// Why cgo, not a pure-Go secp256k1: benchmarked on this repo; the C
// libsecp256k1 wins single- and multi-threaded on modern Go. Don't swap.
var curve = secp256k1.S256()

// nMinus1 is used so adding 1 to a reduced seed yields a key in [1, N-1].
var (
	curveOrderMinus1Words = [4]uint64{
		0xFFFFFFFFFFFFFFFF,
		0xFFFFFFFFFFFFFFFE,
		0xBAAEDCE6AF48A03B,
		0xBFD25E8CD0364140,
	}

	curveOrderBigInt = func() *big.Int {
		n, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
		return n
	}()
)

// SecpKeyGenerator is the shared secp256k1 address mining pipeline used by
// EOA, CREATE1, and Tron vanity generators. Each Next() returns
//
//	(seed ∈ [1, N-1], ethAddr = keccak256(pubkey)[12:])
//
// derived deterministically from the base seed + monotonic counter.
type SecpKeyGenerator struct {
	counter   atomic.Uint64
	baseWords [4]uint64
}

// NewSecpKeyGenerator seeds from crypto/rand.
func NewSecpKeyGenerator() (*SecpKeyGenerator, error) {
	var baseSeed [32]byte
	l, err := rand.Read(baseSeed[:])
	if err != nil || l != 32 {
		return nil, fmt.Errorf("failed to generate random seed: %v", err)
	}
	return newSecpKeyGeneratorFromSeed(baseSeed), nil
}

// NewSecpKeyGeneratorFromSeed is the exported variant of
// newSecpKeyGeneratorFromSeed for cross-package test/tron use.
func NewSecpKeyGeneratorFromSeed(seed [32]byte) *SecpKeyGenerator {
	return newSecpKeyGeneratorFromSeed(seed)
}

// newSecpKeyGeneratorFromSeed reduces seed mod (N-1) once so the hot path
// only needs a fixed-step add.
func newSecpKeyGeneratorFromSeed(seed [32]byte) *SecpKeyGenerator {
	nMinus1 := new(big.Int).Sub(curveOrderBigInt, big.NewInt(1))
	reduced := new(big.Int).Mod(new(big.Int).SetBytes(seed[:]), nMinus1)
	var reducedBytes [32]byte
	reduced.FillBytes(reducedBytes[:])
	return &SecpKeyGenerator{baseWords: bytesToWords(reducedBytes)}
}

// Next returns the next (seed, ethAddr) pair. Threadsafe via the atomic counter.
//
// The counter MUST stay uint64 end-to-end. An earlier implementation
// (pre-92594b4) did `big.NewInt(int64(counter))`, which wrapped negative once
// counter > math.MaxInt64 and corrupted seed derivation; the subsequent
// FillBytes could also panic when intermediates exceeded 256 bits. The
// bits.Add64 path below is structurally immune to both — please don't
// reintroduce int64 casts or big.Int here without re-adding the regression
// tests in *_CounterOverflowBoundary.
func (g *SecpKeyGenerator) Next() (seed [32]byte, addr [20]byte, err error) {
	counter := g.counter.Add(1) - 1

	// seed = (base + counter) mod (N-1), then + 1 -> in [1, N-1].
	sum := add256(g.baseWords, counter)
	if geq256(sum, curveOrderMinus1Words) {
		sum = sub256(sum, curveOrderMinus1Words)
	}
	// +1 without allocation
	sum[3]++
	if sum[3] == 0 {
		sum[2]++
		if sum[2] == 0 {
			sum[1]++
			if sum[1] == 0 {
				sum[0]++
			}
		}
	}
	seed = wordsToBytes(sum)

	x, y := curve.ScalarBaseMult(seed[:])
	if x == nil {
		return seed, addr, errors.New("invalid private key")
	}
	var pub [64]byte
	ethmath.ReadBits(x, pub[:32])
	ethmath.ReadBits(y, pub[32:])
	// Keccak256Hash returns a value-type common.Hash so the output doesn't escape.
	hash := crypto.Keccak256Hash(pub[:])
	copy(addr[:], hash[12:32])
	return seed, addr, nil
}

func bytesToWords(b [32]byte) [4]uint64 {
	return [4]uint64{
		binary.BigEndian.Uint64(b[0:8]),
		binary.BigEndian.Uint64(b[8:16]),
		binary.BigEndian.Uint64(b[16:24]),
		binary.BigEndian.Uint64(b[24:32]),
	}
}

func wordsToBytes(w [4]uint64) [32]byte {
	var b [32]byte
	binary.BigEndian.PutUint64(b[0:8], w[0])
	binary.BigEndian.PutUint64(b[8:16], w[1])
	binary.BigEndian.PutUint64(b[16:24], w[2])
	binary.BigEndian.PutUint64(b[24:32], w[3])
	return b
}

func add256(a [4]uint64, b uint64) [4]uint64 {
	var r [4]uint64
	var carry uint64
	r[3], carry = bits.Add64(a[3], b, 0)
	r[2], carry = bits.Add64(a[2], 0, carry)
	r[1], carry = bits.Add64(a[1], 0, carry)
	r[0], _ = bits.Add64(a[0], 0, carry)
	return r
}

func sub256(a, b [4]uint64) [4]uint64 {
	var r [4]uint64
	var borrow uint64
	r[3], borrow = bits.Sub64(a[3], b[3], 0)
	r[2], borrow = bits.Sub64(a[2], b[2], borrow)
	r[1], borrow = bits.Sub64(a[1], b[1], borrow)
	r[0], _ = bits.Sub64(a[0], b[0], borrow)
	return r
}

func geq256(a, b [4]uint64) bool {
	for i := 0; i < 4; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return true
}
