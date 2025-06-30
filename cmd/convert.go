package cmd

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"
	"github.com/urfave/cli/v3"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, convertCmd)
}

var convertCmd = &cli.Command{
	Name:  "convert",
	Usage: "Convert helper. Supported types: base58, base64, hex, bigint.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "in",
			Aliases:  []string{"i"},
			Usage:    "Input unit",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "out",
			Aliases:  []string{"o"},
			Usage:    "Output unit",
			Required: true,
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		in := cmd.String("in")
		out := cmd.String("out")
		val := cmd.Args().First()

		b, err := decodeToBytes(in, val)
		if err != nil {
			return err
		}

		ret, err := encodeToString(out, b)
		if err != nil {
			return err
		}

		fmt.Println(ret)
		return nil
	},
}

func decodeToBytes(t string, val string) ([]byte, error) {
	switch t {
	case "base58":
		return base58.Decode(val)
	case "base64":
		return base64.StdEncoding.DecodeString(val)
	case "hex":
		return hex.DecodeString(val)
	case "bigint":
		bn, ok := big.NewInt(0).SetString(val, 10)
		if !ok {
			return nil, fmt.Errorf("invalid bigint: %s", val)
		}
		return bn.Bytes(), nil
	}
	return nil, fmt.Errorf("unsupported type: %s", t)
}

func encodeToString(t string, b []byte) (string, error) {
	switch t {
	case "base58":
		return base58.Encode(b), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(b), nil
	case "hex":
		return hex.EncodeToString(b), nil
	case "bigint":
		return big.NewInt(0).SetBytes(b).String(), nil
	}
	return "", fmt.Errorf("unsupported type: %s", t)
}
