package ethereum

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/naiba/nb/model"
)

// EthereumAddressGenerator generates Ethereum EOA addresses using deterministic
// seed derivation (baseSeed + counter) for better performance than per-call CSPRNG.
type EthereumAddressGenerator struct {
	counter    atomic.Uint64
	baseSeed   [32]byte
	curveOrder *big.Int
}

type EthereumAddressData struct {
	privateKey *ecdsa.PrivateKey
	address    string
}

func NewEthereumAddressGenerator() (*EthereumAddressGenerator, error) {
	var baseSeed [32]byte
	l, err := rand.Read(baseSeed[:])
	if err != nil || l != 32 {
		return nil, fmt.Errorf("failed to generate random seed: %v", err)
	}
	// secp256k1 curve order n, valid private key range is [1, n-1]
	curveOrder, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

	return &EthereumAddressGenerator{
		baseSeed:   baseSeed,
		curveOrder: curveOrder,
	}, nil
}

func (g *EthereumAddressGenerator) Generate() (string, interface{}, error) {
	counter := g.counter.Add(1) - 1

	// Combine base seed with counter to create unique seed
	seedBn := new(big.Int).SetBytes(g.baseSeed[:])
	// Use SetUint64 to avoid uint64→int64 overflow when counter > math.MaxInt64
	seedBn.Add(seedBn, new(big.Int).SetUint64(counter))
	// Mod by (n-1) then add 1 to ensure result is in [1, n-1]
	seedBn.Mod(seedBn, new(big.Int).Sub(g.curveOrder, big.NewInt(1)))
	seedBn.Add(seedBn, big.NewInt(1))

	var seedBytes [32]byte
	seedBn.FillBytes(seedBytes[:])

	privateKey, err := crypto.ToECDSA(seedBytes[:])
	if err != nil {
		return "", nil, err
	}

	// Get address
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	addressChecksum := address.Hex()
	addressHex := addressChecksum[2:] // Remove 0x prefix

	return addressHex, &EthereumAddressData{
		privateKey: privateKey,
		address:    addressChecksum,
	}, nil
}

func VanityAddress(config *model.VanityConfig) error {
	log.Printf("REMINDER: Ethereum addresses only contain hexadecimal characters (0-9, a-f, A-F)")

	if config.Contains != "" {
		validHexChars := "0123456789abcdefABCDEF"
		for _, char := range config.Contains {
			if !strings.ContainsRune(validHexChars, char) {
				return fmt.Errorf("contains illegal character: %c (Ethereum addresses only contain 0-9, a-f, A-F)", char)
			}
		}
	}

	if config.Mask != nil {
		log.Printf("Mask: 0x%x", config.Mask)
		log.Printf("MaskValue: 0x%x", config.MaskValue)
	}

	// Create generator
	generator, err := NewEthereumAddressGenerator()
	if err != nil {
		return err
	}

	// Estimate search space and time (using curve order as upper bound)
	estimateSeconds := new(big.Int).Mul(new(big.Int).Div(generator.curveOrder, big.NewInt(int64(config.Threads*10000000))), big.NewInt(23))
	secondsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(secondsOf100Years) == 1 {
		estimateSeconds = secondsOf100Years
	}
	estimateTime := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Search space: %v addresses, estimated max time: %v (2.6 GHz 6-Core Intel Core i7)", generator.curveOrder, estimateTime)

	// Create searcher
	searcher := model.NewVanitySearcher(config, generator)

	// Search
	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	// Output result
	data := result.Data.(*EthereumAddressData)
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(data.privateKey))

	log.Printf("Address: %s", data.address)
	log.Printf("Private Key (hex): %s", privateKeyHex)
	log.Printf("Private Key (with 0x prefix): 0x%s", privateKeyHex)

	return nil
}
