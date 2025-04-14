package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/naiba/nb/internal/ethereum"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, ethereumCmd)
}

var ethereumCmd = &cli.Command{
	Name:  "ethereum",
	Usage: "Ethereum helper.",
	Subcommands: []*cli.Command{
		timestampToBlockNumberCmd,
		checkSandwichAttackCmd,
	},
}

var checkSandwichAttackCmd = &cli.Command{
	Name:    "check-sandwich-attack",
	Usage:   "Check sandwich attack.",
	Aliases: []string{"csa"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "rpc",
			Aliases: []string{"r"},
			Usage:   "Ethereum RPC endpoint.",
			Value:   "https://bsc-rpc.publicnode.com",
		},
		&cli.StringFlag{
			Name:  "tx",
			Usage: "Transaction hash.",
		},
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
			Usage:   "User address.",
		},
		&cli.StringFlag{
			Name:    "token",
			Aliases: []string{"t"},
			Usage:   "Token address.",
		},
		&cli.IntFlag{
			Name:    "max-check-tx-count",
			Aliases: []string{"m"},
			Usage:   "Max check tx count.",
			Value:   20,
		},
	},
	Action: func(c *cli.Context) error {
		rpc := c.String("rpc")
		if rpc == "" {
			return fmt.Errorf("rpc endpoint is required")
		}

		txHash := c.String("tx")
		if txHash == "" {
			return fmt.Errorf("transaction hash is required")
		}
		user := c.String("user")
		if user == "" {
			return fmt.Errorf("user address is required")
		}
		token := c.String("token")
		if token == "" {
			return fmt.Errorf("token address is required")
		}
		maxCheckTxCount := c.Int("max-check-tx-count")
		if maxCheckTxCount == 0 {
			return fmt.Errorf("max check tx count is required")
		}
		return ethereum.CheckSandwichAttack(c.Context, rpc, txHash, user, token, maxCheckTxCount)
	},
}

type jsonRpcReq struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type jsonRpcResp struct {
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	ID      int         `json:"id"`
}

func jsonRpcCall(rpc string, method string, params interface{}) (*jsonRpcResp, error) {
	req, err := json.Marshal(jsonRpcReq{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      time.Now().Nanosecond(),
	})
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(rpc, "application/json", bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rpcResp jsonRpcResp
	err = json.NewDecoder(resp.Body).Decode(&rpcResp)
	return &rpcResp, err
}

func getBlockTimestampByNumber(rpc string, blockNumber *big.Int) (int64, error) {
	blockResp, err := jsonRpcCall(rpc, "eth_getBlockByNumber", []interface{}{
		"0x" + blockNumber.Text(16),
		false,
	})
	if err != nil {
		return 0, err
	}
	blockTimestamp, ok := new(big.Int).SetString(blockResp.Result.(map[string]interface{})["timestamp"].(string)[2:], 16)
	if !ok {
		return 0, cli.Exit("Invalid block timestamp", 1)
	}
	return blockTimestamp.Int64(), nil
}

var timestampToBlockNumberCmd = &cli.Command{
	Name:    "timestamp-to-block-number",
	Aliases: []string{"t2b"},
	Usage:   "Convert timestamp to block number.",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:    "timestamp",
			Aliases: []string{"t"},
			Usage:   "Timestamp to convert.",
		},
		&cli.StringFlag{
			Name:    "rpc",
			Aliases: []string{"r"},
			Usage:   "Ethereum RPC endpoint.",
		},
	},
	Action: func(c *cli.Context) error {
		rpc := c.String("rpc")
		if rpc == "" {
			return cli.Exit("RPC endpoint is required", 1)
		}
		targetTimestap := c.Int64("timestamp")
		if targetTimestap == 0 {
			return cli.Exit("Timestamp is required", 1)
		}
		blockNumberResp, err := jsonRpcCall(rpc, "eth_blockNumber", []interface{}{})
		if err != nil {
			return err
		}
		blockNumber, ok := new(big.Int).SetString(blockNumberResp.Result.(string)[2:], 16)
		if !ok {
			return cli.Exit("Invalid block number", 1)
		}

		timestampA, err := getBlockTimestampByNumber(rpc, blockNumber)
		if err != nil {
			return err
		}
		timestampB, err := getBlockTimestampByNumber(rpc, new(big.Int).Sub(blockNumber, big.NewInt(100)))
		if err != nil {
			return err
		}

		avgBlockTime := (timestampA - timestampB) / 100
		if avgBlockTime == 0 {
			avgBlockTime = 1
		}
		log.Printf("Average block time: %d", avgBlockTime)

		blockNumber.Add(blockNumber, big.NewInt((targetTimestap-timestampA)/avgBlockTime))
		log.Printf("Estimate block number: %s", blockNumber.Text(10))

		blockToTimestampCache := make(map[int64]int64)
		for {
			blockTimestamp, err := getBlockTimestampByNumber(rpc, blockNumber)
			if err != nil {
				return err
			}
			blockToTimestampCache[blockNumber.Int64()] = blockTimestamp

			if blockTimestamp == targetTimestap {
				break
			}

			blockDiff := (targetTimestap - blockTimestamp) / avgBlockTime
			if (targetTimestap-blockTimestamp)%avgBlockTime != 0 {
				if targetTimestap > blockTimestamp {
					blockDiff++
				} else {
					blockDiff--
				}
			}
			if math.Abs(float64(blockDiff)) <= 10 {
				blockDiff = int64(math.Copysign(1, float64(blockDiff)))
			}
			log.Printf("===> Block number: %s, timestamp: %d, blockDiff: %d", blockNumber.Text(10), blockTimestamp, blockDiff)

			if blockTimestamp > targetTimestap {
				olderBlockTimestamp, exists := blockToTimestampCache[blockNumber.Int64()-1]
				if exists && olderBlockTimestamp < targetTimestap {
					break
				}
			}

			if blockTimestamp < targetTimestap {
				newerBlockTimestamp, exists := blockToTimestampCache[blockNumber.Int64()+1]
				if exists && newerBlockTimestamp > targetTimestap {
					break
				}
			}

			blockNumber.Add(blockNumber, big.NewInt(blockDiff))
		}

		log.Printf("Block number: %s", blockNumber.Text(10))
		return nil
	},
}
