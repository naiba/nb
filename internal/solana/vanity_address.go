package solana

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"math/bits"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mr-tron/base58"
	"github.com/naiba/nb/model"
)

type SolanaAddressData struct {
	address    string
	privateKey ed25519.PrivateKey
}

// SolanaAddressGenerator produces ed25519 keys from (baseSeed + counter) mod
// 2^256. ed25519 accepts any 32-byte seed, so no modular reduction is needed.
type SolanaAddressGenerator struct {
	counter   atomic.Uint64
	baseWords [4]uint64
}

func NewSolanaAddressGenerator() (*SolanaAddressGenerator, error) {
	var baseSeed [32]byte
	l, err := rand.Read(baseSeed[:])
	if err != nil || l != 32 {
		return nil, fmt.Errorf("failed to generate random seed: %v", err)
	}
	return newSolanaAddressGeneratorFromSeed(baseSeed), nil
}

func newSolanaAddressGeneratorFromSeed(seed [32]byte) *SolanaAddressGenerator {
	return &SolanaAddressGenerator{baseWords: [4]uint64{
		binary.BigEndian.Uint64(seed[0:8]),
		binary.BigEndian.Uint64(seed[8:16]),
		binary.BigEndian.Uint64(seed[16:24]),
		binary.BigEndian.Uint64(seed[24:32]),
	}}
}

func (g *SolanaAddressGenerator) Generate() (string, interface{}, error) {
	counter := g.counter.Add(1) - 1

	// 256-bit add with carry; wrap at 2^256 is fine for ed25519 seeds.
	var r [4]uint64
	var carry uint64
	r[3], carry = bits.Add64(g.baseWords[3], counter, 0)
	r[2], carry = bits.Add64(g.baseWords[2], 0, carry)
	r[1], carry = bits.Add64(g.baseWords[1], 0, carry)
	r[0], _ = bits.Add64(g.baseWords[0], 0, carry)

	var seed [32]byte
	binary.BigEndian.PutUint64(seed[0:8], r[0])
	binary.BigEndian.PutUint64(seed[8:16], r[1])
	binary.BigEndian.PutUint64(seed[16:24], r[2])
	binary.BigEndian.PutUint64(seed[24:32], r[3])

	privateKey := ed25519.NewKeyFromSeed(seed[:])
	address := base58.Encode(privateKey[32:])

	return address, &SolanaAddressData{
		address:    address,
		privateKey: privateKey,
	}, nil
}

func VanityAddress(config *model.VanityConfig) error {
	log.Printf("REMINDER: Solana addresses use Base58 encoding (excludes 0, O, I, l)")

	// Base58 alphabet: 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
	// Excluded: 0 (zero), O (capital o), I (capital i), l (lowercase L)
	validBase58Chars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, char := range config.Contains {
		if !strings.ContainsRune(validBase58Chars, char) {
			return fmt.Errorf("contains illegal character: %c (Solana addresses use Base58: excludes 0, O, I, l)", char)
		}
	}

	generator, err := NewSolanaAddressGenerator()
	if err != nil {
		return err
	}

	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	estimateSeconds := new(big.Int).Mul(new(big.Int).Div(maxUint256, big.NewInt(int64(config.Threads*10000000))), big.NewInt(23))
	secondsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(secondsOf100Years) == 1 {
		estimateSeconds = secondsOf100Years
	}
	estimateTime := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Search space: %v addresses, estimated max time: %v (2.6 GHz 6-Core Intel Core i7)", maxUint256, estimateTime)

	searcher := model.NewVanitySearcher(config, generator)

	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	data := result.Data.(*SolanaAddressData)

	var privateKeyArray [64]byte
	copy(privateKeyArray[:], data.privateKey)
	privateKeyJSON, _ := json.Marshal(privateKeyArray)

	log.Printf("Address: %s", data.address)
	log.Printf("Private Key (bytes): %s", string(privateKeyJSON))
	log.Printf("Private Key (hex): %x", privateKeyArray)

	return nil
}
