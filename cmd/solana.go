package cmd

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log"
	"strings"
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

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		g, gctx := errgroup.WithContext(ctx)
		result := make(chan VanityResult, 1)

		for i := 0; i < threads; i++ {
			g.Go(func() error {
				for {
					select {
					case <-gctx.Done():
						return nil
					default:
						pubKey, pk, err := ed25519.GenerateKey(nil)
						if err != nil {
							time.Sleep(time.Microsecond * 100)
							continue
						}
						address := base58.Encode(pubKey)
						if addressMatchesCriteria(caseSensitive, contains, mode, address) {
							select {
							case result <- VanityResult{
								Address:    address,
								PrivateKey: strings.ReplaceAll(fmt.Sprintf("%v", pk), " ", ","),
							}:
								cancel() // 通知其他 goroutine 退出
							default: // 防止死锁
							}
							return nil
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
