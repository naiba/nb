package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	solanax "github.com/naiba/nb/internal/solana"
	"github.com/urfave/cli/v3"
)

var solanaCmd = &cli.Command{
	Name:  "solana",
	Usage: "Solana helper.",
	Commands: []*cli.Command{
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
	Action: func(ctx context.Context, cmd *cli.Command) error {
		rpcUrl := cmd.String("rpc")
		signature := cmd.String("signature")

		if signature == "" {
			return errors.New("transaction signature is required")
		}

		rpcClient := rpc.New(rpcUrl)
		maxSupportedTransactionVersion := uint64(0)
		ret, err := rpcClient.GetTransaction(
			ctx,
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
		if err := solanax.FillAddressLookupTable(ctx, rpcClient, tx); err != nil {
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
		&cli.BoolFlag{
			Name:    "no-signature",
			Aliases: []string{"s"},
			Usage:   "This data has no signature only the message.",
		},
		&cli.BoolFlag{
			Name:    "simulate",
			Aliases: []string{"d"},
			Usage:   "Whether to simulate the transaction.",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		rpcUrl := cmd.String("rpc")
		txBase64 := cmd.String("tx-base64")
		parseALT := cmd.Bool("load-alt")
		noSignature := cmd.Bool("no-signature")
		simulate := cmd.Bool("simulate")
		pretty := cmd.Bool("pretty")

		if txBase64 == "" {
			return errors.New("transaction is required")
		}

		var ret string
		var err error

		if pretty {
			ret, err = solanax.DecodeTransaction(ctx, rpcUrl, txBase64, parseALT, noSignature)
		} else {
			ret, err = solanax.DecodeTransactionByteByByte(ctx, rpcUrl, txBase64, parseALT, noSignature)
		}

		if simulate && err == nil {
			err = solanax.Simulate(rpcUrl, ret)
		}

		return err
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
	Action: func(ctx context.Context, cmd *cli.Command) error {
		rpcUrl := cmd.String("rpc")
		address := cmd.String("address")
		signature := cmd.String("signature")
		token := cmd.String("token")
		return solanax.CheckSandwichAttack(ctx, rpcUrl, signature, address, token, 100)
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
	Action: func(ctx context.Context, cmd *cli.Command) error {
		threads := cmd.Int("threads")
		contains := cmd.String("contains")
		mode := cmd.Int("mode")
		caseSensitive := cmd.Bool("case-sensitive")
		upperOrLower := cmd.Bool("upper-or-lower")

		if (mode < 1 || mode > 3) || contains == "" {
			return fmt.Errorf("mode must be 1, 2, or 3 and contains cannot be empty")
		}

		return solanax.VanityAddress(
			int(threads),
			contains,
			int(mode),
			caseSensitive,
			upperOrLower,
		)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, solanaCmd)
}
