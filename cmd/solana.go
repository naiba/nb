package cmd

import (
	"context"
	"errors"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/urfave/cli/v2"

	solanax "github.com/naiba/nb/internal/solana"
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
		decodeTransactionCmd,
		getTransactionCmd,
	},
}

var getTransactionCmd = &cli.Command{
	Name:  "get-transaction",
	Usage: "Get transaction.",
	Aliases: []string{
		"gt",
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "rpc",
			Aliases: []string{"r"},
			Value:   "https://solana-rpc.publicnode.com",
		},
		&cli.StringFlag{
			Name:    "signature",
			Aliases: []string{"s"},
			Usage:   "The transaction signature.",
		},
	},
	Action: func(c *cli.Context) error {
		rpcUrl := c.String("rpc")
		signature := c.String("signature")

		if signature == "" {
			cli.ShowSubcommandHelp(c)
			return errors.New("Transaction signature is required")
		}

		rpcClient := rpc.New(rpcUrl)
		maxSupportedTransactionVersion := uint64(0)
		ret, err := rpcClient.GetTransaction(
			c.Context,
			solana.MustSignatureFromBase58(signature),
			&rpc.GetTransactionOpts{
				Encoding:                       solana.EncodingBase64,
				MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
			},
		)
		if err != nil {
			return err
		}
		tx, err := ret.Transaction.GetTransaction()
		if err != nil {
			return err
		}
		if err := solanax.FillAddressLookupTable(c.Context, rpcClient, tx); err != nil {
			return err
		}
		log.Print(tx.String())
		return nil
	},
}

var decodeTransactionCmd = &cli.Command{
	Name:  "decode-transaction",
	Usage: "Decode transaction.",
	Aliases: []string{
		"dt",
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "rpc",
			Aliases: []string{"r"},
			Value:   "https://solana-rpc.publicnode.com",
		},
		&cli.StringFlag{
			Name:    "tx-base64",
			Aliases: []string{"t"},
			Usage:   "The transaction to decode.",
		},
		&cli.BoolFlag{
			Name:    "load-alt",
			Aliases: []string{"l"},
			Usage:   "Whether to load the address lookup table.",
		},
		&cli.BoolFlag{
			Name:    "pretty",
			Aliases: []string{"p"},
			Usage:   "Whether to pretty print the output.",
		},
	},
	Action: func(c *cli.Context) error {
		rpcUrl := c.String("rpc")
		txBase64 := c.String("tx-base64")
		parseALT := c.Bool("load-alt")

		if txBase64 == "" {
			cli.ShowSubcommandHelp(c)
			return cli.Exit("Transaction is required", 1)
		}

		if c.Bool("pretty") {
			return solanax.DecodeTransaction(c.Context, rpcUrl, txBase64, parseALT)
		}

		return solanax.DecodeTransactionByteByByte(c.Context, rpcUrl, txBase64, parseALT)
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
			Value:   "https://solana-rpc.publicnode.com",
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
		rpcUrl := c.String("rpc")
		address := c.String("address")
		signature := c.String("signature")
		token := c.String("token")
		return solanax.CheckSandwichAttack(context.Background(), rpcUrl, signature, address, token, 100)
	},
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

		if (mode < 1 || mode > 3) || contains == "" {
			cli.ShowSubcommandHelpAndExit(c, 1)
		}

		return solanax.VanityAddress(
			threads,
			contains,
			mode,
			caseSensitive,
			upperOrLower,
		)
	},
}
