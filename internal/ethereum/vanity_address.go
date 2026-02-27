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
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/naiba/nb/model"
)

// EthereumAddressGenerator generates random Ethereum EOA addresses
type EthereumAddressGenerator struct{}

type EthereumAddressData struct {
	privateKey *ecdsa.PrivateKey
	address    string
}

func (g *EthereumAddressGenerator) Generate() (string, interface{}, error) {
	// Generate random private key
	privateKey, err := crypto.GenerateKey()
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

	// Estimate remaining addresses
	initialSeedBytes := make([]byte, 32)
	l, err := rand.Read(initialSeedBytes)
	if err != nil || l != 32 {
		return fmt.Errorf("failed to generate random seed: %v", err)
	}
	initialSeedBn := new(big.Int).SetBytes(initialSeedBytes)
	MAX_UINT256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	remaining := new(big.Int).Sub(MAX_UINT256, initialSeedBn)
	estimateSeconds := new(big.Int).Mul(new(big.Int).Div(remaining, big.NewInt(int64(config.Threads*10000000))), big.NewInt(23))
	secondsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(secondsOf100Years) == 1 {
		estimateSeconds = secondsOf100Years
	}
	estimateTime := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Remaining addresses to search: %v, estimated time: %v (2.6 GHz 6-Core Intel Core i7)", remaining, estimateTime)

	// Create searcher
	generator := &EthereumAddressGenerator{}
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
