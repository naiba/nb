package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/btcsuite/btcutil/base58"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, solanaCmd)
}

var solanaCmd = &cli.Command{
	Name:  "solana",
	Usage: "Solana helper.",
	Subcommands: []*cli.Command{
		solanaVanityCmd,
	},
}

func addressMatchesCriteria(caseSensitive bool, contains string, mode int, address string) bool {
	if !caseSensitive {
		address = strings.ToLower(address)
	}
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

type VanityResult struct {
	Address    string
	PrivateKey string
}

var solanaVanityCmd = &cli.Command{
	Name:  "vanity",
	Usage: "Generate vanity address.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "contains",
			Aliases: []string{"c"},
			Usage:   "The address must contain this string.",
		},
		&cli.IntFlag{
			Name:    "mode",
			Aliases: []string{"m"},
			Usage:   "The mode of matching. 1: prefix, 2: suffix, 3: prefix or suffix.",
		},
		&cli.BoolFlag{
			Name:    "case-sensitive",
			Aliases: []string{"cs"},
			Usage:   "Whether the matching is case sensitive.",
		},
		&cli.IntFlag{
			Name:    "threads",
			Aliases: []string{"t"},
			Usage:   "The number of threads to use.",
			Value:   1,
		},
	},
	Action: func(c *cli.Context) error {
		threads := c.Int("threads")
		contains := c.String("contains")
		mode := c.Int("mode")
		caseSensitive := c.Bool("case-sensitive")

		if (mode < 1 || mode > 3) || contains == "" {
			cli.ShowSubcommandHelpAndExit(c, 1)
		}

		if !caseSensitive {
			contains = strings.ToLower(contains)
		}

		initialSeedBytes := make([]byte, 32)
		l, err := rand.Read(initialSeedBytes)
		if err != nil || l != 32 {
			log.Fatalf("Failed to generate random seed: %v", err)
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
		result := make(chan VanityResult, 1)

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
							if addressMatchesCriteria(caseSensitive, contains, mode, address) {
								select {
								case result <- VanityResult{
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
			log.Printf("%+v", res)
			return nil
		}
		return nil
	},
}
