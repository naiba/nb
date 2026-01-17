package ethereum

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/naiba/nb/model"
)

type Create1AddressData struct {
	deployerAddress common.Address
	contractAddress common.Address
	privateKeyBytes []byte
}

// Create1AddressGenerator generates CREATE1 contract addresses
type Create1AddressGenerator struct {
	counter    atomic.Uint64
	baseSeed   [32]byte
	maxUint256 *big.Int
}

func NewCreate1AddressGenerator() (*Create1AddressGenerator, error) {
	var baseSeed [32]byte
	l, err := rand.Read(baseSeed[:])
	if err != nil || l != 32 {
		return nil, fmt.Errorf("failed to generate random seed: %v", err)
	}
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	return &Create1AddressGenerator{
		baseSeed:   baseSeed,
		maxUint256: maxUint256,
	}, nil
}

func (g *Create1AddressGenerator) Generate() (string, interface{}, error) {
	counter := g.counter.Add(1) - 1

	// Combine base seed with counter to create unique seed
	seedBn := new(big.Int).SetBytes(g.baseSeed[:])
	seedBn.Add(seedBn, big.NewInt(int64(counter)))
	if seedBn.Cmp(g.maxUint256) >= 0 {
		seedBn.Mod(seedBn, g.maxUint256)
	}

	var seedBytes [32]byte
	seedBn.FillBytes(seedBytes[:])

	privateKey, err := crypto.ToECDSA(seedBytes[:])
	if err != nil {
		return "", nil, err
	}

	// Generate deployer address
	deployerAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Compute contract address for first deployment (nonce=0)
	contractAddress := computeCreate1Address(deployerAddress)

	// Get checksum address (EIP-55 format)
	contractAddrChecksum := contractAddress.Hex()
	contractAddrHex := contractAddrChecksum[2:] // Remove 0x prefix

	privateKeyBytes := crypto.FromECDSA(privateKey)

	return contractAddrHex, &Create1AddressData{
		deployerAddress: deployerAddress,
		contractAddress: contractAddress,
		privateKeyBytes: privateKeyBytes,
	}, nil
}

// computeCreate1Address computes the contract address for the first transaction (nonce=0)
func computeCreate1Address(deployerAddress common.Address) common.Address {
	// For nonce=0, we compute: Keccak256(RLP([address, 0]))
	data, _ := rlp.EncodeToBytes([]interface{}{deployerAddress, uint64(0)})
	hash := crypto.Keccak256Hash(data)

	var contractAddr common.Address
	copy(contractAddr[:], hash[12:]) // Take the last 20 bytes
	return contractAddr
}

func VanityCreate1Address(config *model.VanityConfig) error {
	log.Printf("REMINDER: Ethereum addresses only contain hexadecimal characters (0-9, a-f, A-F)")
	log.Printf("Searching for contract address (first deployment, nonce=0) containing: %s", config.Contains)

	// Validate that contains only has valid hex characters
	validHexChars := "0123456789abcdefABCDEF"
	for _, char := range config.Contains {
		if !strings.ContainsRune(validHexChars, char) {
			return fmt.Errorf("contains illegal character: %c (Ethereum addresses only contain 0-9, a-f, A-F)", char)
		}
	}

	// Create generator
	generator, err := NewCreate1AddressGenerator()
	if err != nil {
		return err
	}

	// Estimate search space and time (using maxUint256 as upper bound)
	estimateSeconds := new(big.Int).Mul(new(big.Int).Div(generator.maxUint256, big.NewInt(int64(config.Threads*10000000))), big.NewInt(23))
	secondsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(secondsOf100Years) == 1 {
		estimateSeconds = secondsOf100Years
	}
	estimateTime := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Search space: %v addresses, estimated max time: %v (2.6 GHz 6-Core Intel Core i7)", generator.maxUint256, estimateTime)

	// Create searcher
	searcher := model.NewVanitySearcher(config, generator)

	// Search
	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	// Output result
	data := result.Data.(*Create1AddressData)
	privateKeyHex := hex.EncodeToString(data.privateKeyBytes)

	log.Printf("Deployer Address: %s", data.deployerAddress.Hex())
	log.Printf("Contract Address (first deployment, nonce=0): %s", data.contractAddress.Hex())
	log.Printf("Private Key (hex): %s", privateKeyHex)
	log.Printf("Private Key (with 0x prefix): 0x%s", privateKeyHex)

	return nil
}
