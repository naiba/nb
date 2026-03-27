package solana

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
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

// SolanaAddressGenerator generates Solana addresses
type SolanaAddressGenerator struct {
	counter  atomic.Uint64
	baseSeed [32]byte
}

func NewSolanaAddressGenerator() (*SolanaAddressGenerator, error) {
	var baseSeed [32]byte
	l, err := rand.Read(baseSeed[:])
	if err != nil || l != 32 {
		return nil, fmt.Errorf("failed to generate random seed: %v", err)
	}

	return &SolanaAddressGenerator{
		baseSeed: baseSeed,
	}, nil
}

func (g *SolanaAddressGenerator) Generate() (string, interface{}, error) {
	counter := g.counter.Add(1) - 1

	// Combine base seed with counter to create unique seed
	seedBn := new(big.Int).SetBytes(g.baseSeed[:])
	// Use SetUint64 to avoid uint64→int64 overflow when counter > math.MaxInt64
	seedBn.Add(seedBn, new(big.Int).SetUint64(counter))

	// Truncate to 32 bytes to prevent FillBytes panic when sum exceeds 256 bits.
	// Ed25519 NewKeyFromSeed accepts any 32 bytes so wrapping is safe.
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	if seedBn.Cmp(maxUint256) > 0 {
		seedBn.And(seedBn, maxUint256)
	}

	var seed [32]byte
	seedBn.FillBytes(seed[:])

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

	// Create generator
	generator, err := NewSolanaAddressGenerator()
	if err != nil {
		return err
	}

	// Estimate search space and time
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	estimateSeconds := new(big.Int).Mul(new(big.Int).Div(maxUint256, big.NewInt(int64(config.Threads*10000000))), big.NewInt(23))
	secondsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(secondsOf100Years) == 1 {
		estimateSeconds = secondsOf100Years
	}
	estimateTime := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Search space: %v addresses, estimated max time: %v (2.6 GHz 6-Core Intel Core i7)", maxUint256, estimateTime)

	// Create searcher
	searcher := model.NewVanitySearcher(config, generator)

	// Search
	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	// Output result
	data := result.Data.(*SolanaAddressData)

	// Format private key as JSON byte array only when found
	var privateKeyArray [64]byte
	copy(privateKeyArray[:], data.privateKey)
	privateKeyJSON, _ := json.Marshal(privateKeyArray)

	log.Printf("Address: %s", data.address)
	log.Printf("Private Key (bytes): %s", string(privateKeyJSON))
	log.Printf("Private Key (hex): %x", privateKeyArray)

	return nil
}
