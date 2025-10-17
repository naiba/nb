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
	"sync"
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
	initialSeedBn     *big.Int
	initialSeedBnLock *sync.Mutex
	maxUint256        *big.Int
}

func NewSolanaAddressGenerator() (*SolanaAddressGenerator, error) {
	initialSeedBytes := make([]byte, 32)
	l, err := rand.Read(initialSeedBytes)
	if err != nil || l != 32 {
		return nil, fmt.Errorf("failed to generate random seed: %v", err)
	}
	initialSeedBn := new(big.Int).SetBytes(initialSeedBytes)
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	return &SolanaAddressGenerator{
		initialSeedBn:     initialSeedBn,
		initialSeedBnLock: &sync.Mutex{},
		maxUint256:        maxUint256,
	}, nil
}

func (g *SolanaAddressGenerator) Generate() (string, interface{}, error) {
	g.initialSeedBnLock.Lock()
	if g.initialSeedBn.Cmp(g.maxUint256) >= 0 {
		g.initialSeedBnLock.Unlock()
		return "", nil, fmt.Errorf("seed exhausted")
	}
	seedBn := new(big.Int).Set(g.initialSeedBn)
	g.initialSeedBn.Add(g.initialSeedBn, big.NewInt(1))
	g.initialSeedBnLock.Unlock()

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

	// Estimate remaining addresses and time
	remaining := new(big.Int).Sub(generator.maxUint256, generator.initialSeedBn)
	estimateSeconds := new(big.Int).Mul(new(big.Int).Div(remaining, big.NewInt(int64(config.Threads*10000000))), big.NewInt(23))
	secondsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(secondsOf100Years) == 1 {
		estimateSeconds = secondsOf100Years
	}
	estimateTime := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Remaining addresses to search: %v, estimated time: %v (2.6 GHz 6-Core Intel Core i7)", remaining, estimateTime)

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
