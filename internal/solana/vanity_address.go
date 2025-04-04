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
	"golang.org/x/sync/errgroup"
)

func addressMatchesCriteria(contains string, mode int, address string) bool {
	switch mode {
	case 1:
		return address[:len(contains)] == contains
	case 2:
		return address[len(address)-len(contains):] == contains
	case 3:
		return address[:len(contains)] == contains || address[len(address)-len(contains):] == contains
	default:
		return false
	}
}

type vanityResult struct {
	Address    string
	PrivateKey string
}

func VanityAddress(
	threads int,
	contains string,
	mode int,
	caseSensitive bool,
	upperOrLower bool,
) error {
	log.Printf("REMINDER: address can not contains number 0, alphabet O, I, l")
	containsLower := strings.ToLower(contains)
	containsUpper := strings.ToUpper(contains)

	initialSeedBytes := make([]byte, 32)
	l, err := rand.Read(initialSeedBytes)
	if err != nil || l != 32 {
		return fmt.Errorf("Failed to generate random seed: %v", err)
	}
	initialSeedBn := new(big.Int).SetBytes(initialSeedBytes)
	initialSeedBnLock := new(sync.Mutex)

	MAX_UINT256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	remaining := new(big.Int).Sub(MAX_UINT256, initialSeedBn)
	estimateSecounds := new(big.Int).Mul(new(big.Int).Div(remaining, big.NewInt(int64(threads*10000000))), big.NewInt(23))
	secoundsOf100Years := new(big.Int).Mul(big.NewInt(100), big.NewInt(365*24*60*60))
	if estimateSecounds.Cmp(secoundsOf100Years) == 1 {
		estimateSecounds = secoundsOf100Years
	}
	estimateTime := time.Duration(estimateSecounds.Uint64()) * time.Second
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
	result := make(chan vanityResult, 1)

	for i := 0; i < threads; i++ {
		var seed [32]byte
		g.Go(func() error {
			for {
				start, end := generateTaskRange()
				for j := start; j.Cmp(end) == -1; j.Add(j, big.NewInt(1)) {
					select {
					case <-gctx.Done():
						return nil
					default:
						j.FillBytes(seed[:])
						privateKey := ed25519.NewKeyFromSeed(seed[:])
						address := base58.Encode(privateKey[32:])

						passed := addressMatchesCriteria(contains, mode, address)

						if !passed && !caseSensitive {
							passed = addressMatchesCriteria(containsLower, mode, strings.ToLower(address))
						}

						if !passed && upperOrLower {
							passed = addressMatchesCriteria(containsUpper, mode, address) || addressMatchesCriteria(containsLower, mode, address)
						}

						if passed {
							select {
							case result <- vanityResult{
								Address:    address,
								PrivateKey: strings.ReplaceAll(fmt.Sprintf("%v", privateKey), " ", ","),
							}:
								cancel() // 通知其他 goroutine 退出
							default: // 防止死锁
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
		log.Printf("%+v\n", res)
		var privateKey [64]byte
		if err := json.Unmarshal([]byte(res.PrivateKey), &privateKey); err != nil {
			return fmt.Errorf("Failed to unmarshal private key: %v", err)
		}
		log.Printf("Hex: %x", privateKey)
	}

	return nil
}
