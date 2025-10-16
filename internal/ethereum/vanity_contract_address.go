package ethereum

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/sync/errgroup"
)

type contractVanityResult struct {
	DeployerAddress string
	ContractAddress string
	PrivateKey      string
}

// computeContractAddress computes the contract address for the first transaction (nonce=0)
func computeContractAddress(deployerAddress common.Address) common.Address {
	// For nonce=0, we compute: Keccak256(RLP([address, 0]))
	data, _ := rlp.EncodeToBytes([]interface{}{deployerAddress, uint64(0)})
	hash := crypto.Keccak256Hash(data)

	var contractAddr common.Address
	copy(contractAddr[:], hash[12:]) // Take the last 20 bytes
	return contractAddr
}

func VanityContractAddress(
	threads int,
	contains string,
	mode int,
	caseSensitive bool,
	upperOrLower bool,
) error {
	log.Printf("REMINDER: Ethereum addresses only contain hexadecimal characters (0-9, a-f, A-F)")
	log.Printf("Searching for contract address (first deployment, nonce=0) containing: %s", contains)

	// Validate that contains only has valid hex characters
	validHexChars := "0123456789abcdefABCDEF"
	for _, char := range contains {
		if !strings.ContainsRune(validHexChars, char) {
			return fmt.Errorf("contains illegal character: %c (Ethereum addresses only contain 0-9, a-f, A-F)", char)
		}
	}

	containsLower := strings.ToLower(contains)
	containsUpper := strings.ToUpper(contains)

	initialSeedBytes := make([]byte, 32)
	l, err := rand.Read(initialSeedBytes)
	if err != nil || l != 32 {
		return fmt.Errorf("failed to generate random seed: %v", err)
	}
	initialSeedBn := new(big.Int).SetBytes(initialSeedBytes)
	initialSeedBnLock := new(sync.Mutex)

	MAX_UINT256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	remaining := new(big.Int).Sub(MAX_UINT256, initialSeedBn)
	estimateSeconds := new(big.Int).Mul(new(big.Int).Div(remaining, big.NewInt(int64(threads*10000000))), big.NewInt(23))
	secondsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSeconds.Cmp(secondsOf100Years) == 1 {
		estimateSeconds = secondsOf100Years
	}
	estimateTime := time.Duration(estimateSeconds.Uint64()) * time.Second
	log.Printf("Remaining addresses to search: %v, estimated time: %v (2.6 GHz 6-Core Intel Core i7)", remaining, estimateTime)

	generateTaskRange := func() (start, end *big.Int) {
		initialSeedBnLock.Lock()
		defer initialSeedBnLock.Unlock()

		if initialSeedBn.Cmp(MAX_UINT256) != -1 {
			panic("Seed exhausted")
		}

		start = new(big.Int).Set(initialSeedBn)

		end = new(big.Int).Add(initialSeedBn, big.NewInt(10000000))
		if end.Cmp(MAX_UINT256) == 1 {
			end.Set(MAX_UINT256)
		}

		initialSeedBn.Set(end)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)
	result := make(chan contractVanityResult, 1)

	for i := 0; i < threads; i++ {
		g.Go(func() error {
			// Pre-allocate buffers to avoid repeated allocations
			var seedBytes [32]byte
			var contractAddrLower string

			for {
				start, end := generateTaskRange()
				for j := start; j.Cmp(end) == -1; j.Add(j, big.NewInt(1)) {
					select {
					case <-gctx.Done():
						return nil
					default:
						// Reuse the same buffer for seed bytes
						j.FillBytes(seedBytes[:])
						privateKey, err := crypto.ToECDSA(seedBytes[:])
						if err != nil {
							continue
						}

						// Generate deployer address
						deployerAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

						// Compute contract address for first deployment (nonce=0)
						contractAddress := computeContractAddress(deployerAddress)

						// Get checksum address (EIP-55 format)
						contractAddrChecksum := contractAddress.Hex()
						// Remove 0x prefix for matching
						contractAddrHex := contractAddrChecksum[2:]

						// Pre-compute lowercase version if needed
						if !caseSensitive || upperOrLower {
							contractAddrLower = strings.ToLower(contractAddrHex)
						}

						// Optimized matching logic
						var passed bool
						if caseSensitive {
							passed = addressMatchesCriteria(contains, mode, contractAddrHex)
						} else if upperOrLower {
							passed = addressMatchesCriteria(containsLower, mode, contractAddrLower) ||
								addressMatchesCriteria(containsUpper, mode, contractAddrHex)
						} else {
							passed = addressMatchesCriteria(containsLower, mode, contractAddrLower)
						}

						if passed {
							privateKeyBytes := crypto.FromECDSA(privateKey)
							select {
							case result <- contractVanityResult{
								DeployerAddress: deployerAddress.Hex(),
								ContractAddress: contractAddrChecksum,
								PrivateKey:      hex.EncodeToString(privateKeyBytes),
							}:
								cancel() // Notify other goroutines to exit
							default: // Prevent deadlock
							}
							return nil
						}
					}
				}
			}
		})
	}

	go func() {
		g.Wait()
		close(result)
	}()

	if res, ok := <-result; ok {
		log.Printf("Deployer Address: %s", res.DeployerAddress)
		log.Printf("Contract Address (first deployment, nonce=0): %s", res.ContractAddress)
		log.Printf("Private Key (hex): %s", res.PrivateKey)
		log.Printf("Private Key (with 0x prefix): 0x%s", res.PrivateKey)
	}

	return nil
}
