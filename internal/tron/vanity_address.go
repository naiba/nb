package tron

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
	"github.com/naiba/nb/model"
)

type TronAddressData struct {
	address    string
	privateKey *ecdsa.PrivateKey
}

// TronAddressGenerator generates Tron addresses
type TronAddressGenerator struct{}

func (g *TronAddressGenerator) Generate() (string, interface{}, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return "", nil, err
	}

	// Get public key
	publicKey := privateKey.PublicKey
	publicKeyBytes := crypto.FromECDSAPub(&publicKey)
	// Remove the first byte (0x04 prefix for uncompressed public key)
	publicKeyBytes = publicKeyBytes[1:]

	// Hash the public key with Keccak256
	hash := crypto.Keccak256(publicKeyBytes)
	// Take last 20 bytes
	addressBytes := hash[len(hash)-20:]

	// Add Tron mainnet prefix (0x41)
	tronAddress := append([]byte{0x41}, addressBytes...)

	// Calculate checksum (double SHA256)
	hash1 := sha256.Sum256(tronAddress)
	hash2 := sha256.Sum256(hash1[:])
	checksum := hash2[:4]

	// Append checksum
	addressWithChecksum := append(tronAddress, checksum...)

	// Encode with Base58
	address := base58.Encode(addressWithChecksum)

	return address, &TronAddressData{
		address:    address,
		privateKey: privateKey,
	}, nil
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
	if config.Mode == 1 { // prefix mode
		if !strings.HasPrefix(config.Contains, "T") {
			log.Printf("WARNING: Tron mainnet addresses always start with 'T'. Your search pattern '%s' will need to match after the 'T'", config.Contains)
		}
	}

	generator := &TronAddressGenerator{}
	searcher := model.NewVanitySearcher(config, generator)

	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	data := result.Data.(*TronAddressData)

	// Convert private key to hex only when found
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(data.privateKey))

	log.Printf("Address: %s", data.address)
	log.Printf("Private Key (hex): %s", privateKeyHex)
	log.Printf("Private Key (with 0x prefix): 0x%s", privateKeyHex)

	return nil
}
