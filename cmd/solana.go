package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/btcsuite/btcutil/base58"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
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
		sandwichAttackCheckCmd,
	},
}

var sandwichAttackCheckCmd = &cli.Command{
	Name:  "check-sandwich-attack",
	Usage: "Check sandwich attack.",
	Aliases: []string{
		"csa",
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "rpc",
			Aliases: []string{"r"},
			Value:   rpc.MainNetBeta_RPC,
		},
		&cli.StringFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Usage:   "The user's address.",
		},
		&cli.StringFlag{
			Name:    "signature",
			Aliases: []string{"s"},
			Usage:   "The transaction signature to check.",
		},
		&cli.StringFlag{
			Name:    "token",
			Aliases: []string{"t"},
			Usage:   "The token to check. \"sol\" is for SOL, others are for SPL tokens.",
		},
	},
	Action: func(c *cli.Context) error {
		addressToCheck := solana.MustPublicKeyFromBase58(c.String("address"))
		tokenToCheck := solana.MustPublicKeyFromBase58(c.String("token"))
		signature := solana.MustSignatureFromBase58(c.String("signature"))
		tokenAccountToCheck, _, err := solana.FindAssociatedTokenAddress(
			addressToCheck,
			tokenToCheck,
		)
		if err != nil {
			return err
		}

		maxSupportedTransactionVersion := uint64(0)
		rpcClient := rpc.New(c.String("rpc"))
		getTxRet, err := rpcClient.GetTransaction(
			c.Context,
			signature,
			&rpc.GetTransactionOpts{
				MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
			},
		)
		if err != nil {
			return err
		}

		tx, err := getTxRet.Transaction.GetTransaction()
		if err != nil {
			return err
		}

		var addressIdx int = -1
		for i, acc := range tx.Message.AccountKeys {
			if acc.Equals(tokenAccountToCheck) {
				addressIdx = i
				break
			}
		}
		if addressIdx == -1 {
			return fmt.Errorf("address not found in transaction")
		}

		var balanceChange *big.Int
		var postTokenBalance *big.Int
		for _, b := range getTxRet.Meta.PostTokenBalances {
			if b.AccountIndex == uint16(addressIdx) && b.Mint.Equals(tokenToCheck) {
				postTokenBalance = decimal.RequireFromString(b.UiTokenAmount.Amount).Mul(decimal.New(10, int32(b.UiTokenAmount.Decimals))).BigInt()
				break
			}
		}
		if postTokenBalance == nil {
			return fmt.Errorf("token balance not found in transaction")
		}
		for _, b := range getTxRet.Meta.PreTokenBalances {
			if b.AccountIndex == uint16(addressIdx) && b.Mint.Equals(tokenToCheck) {
				preBalance := decimal.RequireFromString(b.UiTokenAmount.Amount).Mul(decimal.New(10, int32(b.UiTokenAmount.Decimals))).BigInt()
				balanceChange = new(big.Int).Sub(postTokenBalance, preBalance)
				break
			}
		}

		log.Printf("User token balance change: %v", balanceChange)
		blockRet, err := rpcClient.GetBlockWithOpts(c.Context, getTxRet.Slot, &rpc.GetBlockOpts{
			MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
			Encoding:                       solana.EncodingBase64,
		})
		if err != nil {
			return err
		}

		var relatedTxs []struct {
			Signature string
			Idx       int
			Amount    *big.Int
		}
		var userTxIdx int = -1

		for i := 0; i < len(blockRet.Transactions); i++ {
			t := blockRet.Transactions[i]
			tx := t.MustGetTransaction()
			if t.Meta.Err != nil {
				continue
			}
			if tx.Signatures[0].Equals(signature) {
				if userTxIdx != -1 {
					return fmt.Errorf("user transaction found multiple times in block")
				}
				userTxIdx = i
			}
			balanceChangeIdx := slices.IndexFunc(t.Meta.PostTokenBalances, func(b rpc.TokenBalance) bool {
				return b.Mint.Equals(tokenToCheck)
			})
			if balanceChangeIdx != -1 {
				amount := decimal.RequireFromString(t.Meta.PostTokenBalances[balanceChangeIdx].UiTokenAmount.Amount).
					Sub(decimal.RequireFromString(t.Meta.PreTokenBalances[balanceChangeIdx].UiTokenAmount.Amount)).
					Mul(decimal.New(10, int32(t.Meta.PostTokenBalances[balanceChangeIdx].UiTokenAmount.Decimals))).
					BigInt()
				relatedTxs = append(relatedTxs, struct {
					Signature string
					Idx       int
					Amount    *big.Int
				}{
					Signature: base58.Encode(tx.Signatures[0][:]),
					Idx:       i,
					Amount:    amount,
				})
			}
		}

		if userTxIdx == -1 {
			return fmt.Errorf("user transaction not found in block")
		}

		for _, relatedTx := range relatedTxs {
			var desc string
			if relatedTx.Idx == userTxIdx {
				desc = "(user)"
			}
			log.Printf("idx: %d%s, amount: %v, tx: %s", relatedTx.Idx, desc, relatedTx.Amount, relatedTx.Signature)
		}

		return nil
	},
}

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
		&cli.BoolFlag{
			Name:    "upper-or-lower",
			Aliases: []string{"ul"},
			Usage:   "Whether the matching is upper or lower case.",
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
		upperOrLower := c.Bool("upper-or-lower")
		containsLower := strings.ToLower(contains)
		containsUpper := strings.ToUpper(contains)

		if (mode < 1 || mode > 3) || contains == "" {
			cli.ShowSubcommandHelpAndExit(c, 1)
		}

		log.Printf("REMINDER: address can not contains number 0, alphabet O, I, l")

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

							passed := addressMatchesCriteria(contains, mode, address)

							if !passed && !caseSensitive {
								passed = addressMatchesCriteria(containsLower, mode, strings.ToLower(address))
							}

							if !passed && upperOrLower {
								passed = addressMatchesCriteria(containsUpper, mode, address) || addressMatchesCriteria(containsLower, mode, address)
							}

							if passed {
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
			log.Printf("%+v\n", res)
			var privateKey [64]byte
			if err := json.Unmarshal([]byte(res.PrivateKey), &privateKey); err != nil {
				log.Fatalf("Failed to unmarshal private key: %v", err)
			}
			log.Printf("Hex: %x", privateKey)
		}
		return nil
	},
}
