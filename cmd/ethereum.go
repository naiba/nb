package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal/ethereum"
	"github.com/naiba/nb/model"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, ethereumCmd)
}

var ethereumCmd = &cli.Command{
	Name:  "ethereum",
	Usage: "Ethereum helper.",
	Commands: []*cli.Command{
		ethereumVanityCmd,
		ethereumVanityCreate1Cmd,
		ethereumVanityCreate2Cmd,
		timestampToBlockNumberCmd,
	},
}

var ethereumVanityCmd = &cli.Command{
	Name:  "vanity",
	Usage: "Generate vanity address.",
	Flags: model.VanityFlags(),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		config, err := model.ParseVanityConfig(cmd)
		if err != nil {
			return err
		}

		return ethereum.VanityAddress(config)
	},
}

var ethereumVanityCreate1Cmd = &cli.Command{
	Name:    "vanity-create1",
	Aliases: []string{"vc1"},
	Usage:   "Generate vanity CREATE1 contract address (first deployment, nonce=0).",
	Flags:   model.VanityFlags(),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		config, err := model.ParseVanityConfig(cmd)
		if err != nil {
			return err
		}

		return ethereum.VanityCreate1Address(config)
	},
}

var ethereumVanityCreate2Cmd = &cli.Command{
	Name:    "vanity-create2",
	Aliases: []string{"vc2"},
	Usage:   "Generate vanity CREATE2 address.",
	Flags:   model.VanityCreate2Flags(),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		config, err := model.ParseVanityConfig(cmd)
		if err != nil {
			return err
		}

		// Get CREATE2 specific parameters
		deployer := cmd.String("deployer")
		saltPrefix := cmd.String("salt-prefix")
		contractBin := cmd.String("contract-bin")
		constructorArgs := cmd.StringSlice("constructor-args")

		return ethereum.VanityCreate2Address(config, deployer, saltPrefix, contractBin, constructorArgs)
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
		return 0, fmt.Errorf("invalid block timestamp")
	}
	return blockTimestamp.Int64(), nil
}

var timestampToBlockNumberCmd = &cli.Command{
	Name:    "timestamp-to-block-number",
	Aliases: []string{"ttb"},
	Usage:   "Get block number by timestamp.",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "timestamp",
			Aliases: []string{"t"},
			Usage:   "Timestamp.",
		},
		&cli.StringFlag{
			Name:    "rpc",
			Aliases: []string{"r"},
			Usage:   "Ethereum RPC endpoint.",
			Value:   "https://bsc-rpc.publicnode.com",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		rpc := cmd.String("rpc")
		if rpc == "" {
			return fmt.Errorf("RPC endpoint is required")
		}

		timestamp := cmd.Int("timestamp")
		if timestamp == 0 {
			return fmt.Errorf("timestamp is required")
		}

		blockNumber, err := getBlockNumberByTimestamp(rpc, int64(timestamp))
		if err != nil {
			return err
		}

		fmt.Println(blockNumber)
		return nil
	},
}

func getBlockNumberByTimestamp(rpc string, targetTimestamp int64) (uint64, error) {
	// Get the latest block number
	blockNumberResp, err := jsonRpcCall(rpc, "eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, err
	}
	blockNumber, ok := new(big.Int).SetString(blockNumberResp.Result.(string)[2:], 16)
	if !ok {
		return 0, fmt.Errorf("invalid block number")
	}

	// Get timestamps for calculating average block time
	timestampA, err := getBlockTimestampByNumber(rpc, blockNumber)
	if err != nil {
		return 0, err
	}
	timestampB, err := getBlockTimestampByNumber(rpc, new(big.Int).Sub(blockNumber, big.NewInt(100)))
	if err != nil {
		return 0, err
	}

	// Calculate average block time
	avgBlockTime := (timestampA - timestampB) / 100
	if avgBlockTime == 0 {
		avgBlockTime = 1
	}
	log.Printf("Average block time: %d", avgBlockTime)

	// Initial estimate
	blockNumber.Add(blockNumber, big.NewInt((targetTimestamp-timestampA)/avgBlockTime))
	log.Printf("Estimate block number: %s", blockNumber.Text(10))

	// Binary search with cache
	blockToTimestampCache := make(map[int64]int64)
	for {
		blockTimestamp, err := getBlockTimestampByNumber(rpc, blockNumber)
		if err != nil {
			return 0, err
		}
		blockToTimestampCache[blockNumber.Int64()] = blockTimestamp

		if blockTimestamp == targetTimestamp {
			break
		}

		blockDiff := (targetTimestamp - blockTimestamp) / avgBlockTime
		if (targetTimestamp-blockTimestamp)%avgBlockTime != 0 {
			if targetTimestamp > blockTimestamp {
				blockDiff++
			} else {
				blockDiff--
			}
		}
		if math.Abs(float64(blockDiff)) <= 10 {
			blockDiff = int64(math.Copysign(1, float64(blockDiff)))
		}
		log.Printf("===> Block number: %s, timestamp: %d, blockDiff: %d", blockNumber.Text(10), blockTimestamp, blockDiff)

		// Check if we're close enough with cached values
		if blockTimestamp > targetTimestamp {
			olderBlockTimestamp, exists := blockToTimestampCache[blockNumber.Int64()-1]
			if exists && olderBlockTimestamp < targetTimestamp {
				break
			}
		}

		if blockTimestamp < targetTimestamp {
			newerBlockTimestamp, exists := blockToTimestampCache[blockNumber.Int64()+1]
			if exists && newerBlockTimestamp > targetTimestamp {
				break
			}
		}

		blockNumber.Add(blockNumber, big.NewInt(blockDiff))
	}

	log.Printf("Block number: %s", blockNumber.Text(10))
	return blockNumber.Uint64(), nil
}
