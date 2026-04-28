package tron

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/mr-tron/base58"
	"github.com/naiba/nb/internal/ethereum"
	"github.com/naiba/nb/model"
)

type TronAddressData struct {
	address string
	seed    [32]byte
}

// PrivateKeyBytes returns the 32-byte private key of the Tron address.
func (d *TronAddressData) PrivateKeyBytes() []byte {
	return d.seed[:]
}

// TronAddressGenerator builds Tron mainnet addresses on top of the shared
// ethereum.SecpKeyGenerator. Tron reuses Ethereum's secp256k1→keccak address
// hash; the only Tron-specific steps are the 0x41 prefix and the double-SHA256
// checksum wrapped in base58.
type TronAddressGenerator struct {
	*ethereum.SecpKeyGenerator
}

func NewTronAddressGenerator() (*TronAddressGenerator, error) {
	kg, err := ethereum.NewSecpKeyGenerator()
	if err != nil {
		return nil, err
	}
	return &TronAddressGenerator{SecpKeyGenerator: kg}, nil
}

// NewTronAddressGeneratorFromSeed is exported for deterministic tests.
func NewTronAddressGeneratorFromSeed(seed [32]byte) *TronAddressGenerator {
	return &TronAddressGenerator{SecpKeyGenerator: ethereum.NewSecpKeyGeneratorFromSeed(seed)}
}

func (g *TronAddressGenerator) Generate() (string, interface{}, error) {
	seed, ethAddr, err := g.Next()
	if err != nil {
		return "", nil, err
	}

	// 25-byte payload: 0x41 || eth-addr (20) || checksum (4)
	var payload [25]byte
	payload[0] = 0x41
	copy(payload[1:21], ethAddr[:])

	// Double SHA-256 on the first 21 bytes → take first 4 as checksum
	h1 := sha256.Sum256(payload[:21])
	h2 := sha256.Sum256(h1[:])
	copy(payload[21:25], h2[:4])

	address := base58.Encode(payload[:])
	return address, &TronAddressData{address: address, seed: seed}, nil
}

func VanityAddress(config *model.VanityConfig) error {
	log.Printf("REMINDER: Tron addresses use Base58 encoding (excludes 0, O, I, l)")

	// Base58 alphabet: 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
	// Excluded: 0 (zero), O (capital o), I (capital i), l (lowercase L)
	validBase58Chars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, char := range config.Contains {
		if !strings.ContainsRune(validBase58Chars, char) {
			return fmt.Errorf("contains illegal character: %c (Tron addresses use Base58: excludes 0, O, I, l)", char)
		}
	}

	// Tron addresses always start with 'T' for mainnet
	if config.Mode == model.VanityModePrefix {
		if !strings.HasPrefix(config.Contains, "T") {
			log.Printf("WARNING: Tron mainnet addresses always start with 'T'. Your search pattern '%s' will need to match after the 'T'", config.Contains)
		}
	}

	generator, err := NewTronAddressGenerator()
	if err != nil {
		return err
	}
	searcher := model.NewVanitySearcher(config, generator)

	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	data := result.Data.(*TronAddressData)

	privateKeyHex := hex.EncodeToString(data.PrivateKeyBytes())

	log.Printf("Address: %s", data.address)
	log.Printf("Private Key (hex): %s", privateKeyHex)
	log.Printf("Private Key (with 0x prefix): 0x%s", privateKeyHex)

	return nil
}
