package cmd

import (
	"context"
	"errors"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	solanax "github.com/naiba/nb/internal/solana"
	"github.com/naiba/nb/internal/solana/sandwich"
	"github.com/naiba/nb/model"
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
	Usage: "Check if a user's Solana swap was sandwiched (auto-verdict).",
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
			Usage:   "The user's wallet address.",
		},
		&cli.StringFlag{
			Name:    "signature",
			Aliases: []string{"s"},
			Usage:   "The transaction signature to check.",
		},
		&cli.StringFlag{
			Name:    "token",
			Aliases: []string{"t"},
			Usage:   "The token mint to check.",
		},
		&cli.IntFlag{
			Name:  "slots",
			Value: 2,
			Usage: "Scan user_slot ±N slots (default 2; bump to 3-4 for cross-leader sandwiches).",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return sandwich.Analyze(
			ctx,
			cmd.String("rpc"),
			cmd.String("signature"),
			cmd.String("address"),
			cmd.String("token"),
			int(cmd.Int("slots")),
		)
	},
}

var solanaVanityCmd = &cli.Command{
	Name:  "vanity",
	Usage: "Generate vanity address.",
	Flags: model.VanityFlags(),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		config, err := model.ParseVanityConfig(cmd)
		if err != nil {
			return err
		}

		return solanax.VanityAddress(config)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, solanaCmd)
}
