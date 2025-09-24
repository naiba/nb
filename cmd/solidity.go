package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v3"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, solidityCmd)
}

var solidityCmd = &cli.Command{
	Name:  "solidity",
	Usage: "Solidity helper.",
	Commands: []*cli.Command{
		unflattenCmd,
		create2vanityCmd,
		diamondCutUpgradeReportCmd,
	},
}

var facetsTy, _ = abi.NewType("tuple[]", "Facet", []abi.ArgumentMarshaling{
	{
		Name: "facetAddress",
		Type: "address",
	},
	{
		Name: "functionSelectors",
		Type: "bytes4[]",
	}})

var facetsMethod = abi.NewMethod("facets", "facets", abi.Function, "public", false, false, abi.Arguments{}, abi.Arguments{
	abi.Argument{
		Type: facetsTy,
	},
})

type foundryTomlData struct {
	RpcEndpoints map[string]string `toml:"rpc_endpoints"`
}

type upgradeConfigData []struct {
	Chain   string `json:"chain,omitempty"`
	Diamond string `json:"diamond,omitempty"`
	Facet   string `json:"facet,omitempty"`
	NewImpl string `json:"new_impl,omitempty"`
	Replace bool   `json:"replace,omitempty"`
	Add     bool   `json:"add,omitempty"`
	Remove  bool   `json:"remove,omitempty"`
}

var diamondCutUpgradeReportCmd = &cli.Command{
	Name:  "diamond-upgrade",
	Usage: "Generate a diamond cut upgrade params.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		currentDirPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		var upgradeConfigData upgradeConfigData
		upgradeConfigFilePath := filepath.Join(currentDirPath, "upgrade-config.json")
		upgradeConfigFile, err := os.Open(upgradeConfigFilePath)
		if err != nil {
			return fmt.Errorf("failed to open upgrade-config.json: %w", err)
		}
		defer upgradeConfigFile.Close()
		if err = json.NewDecoder(upgradeConfigFile).Decode(&upgradeConfigData); err != nil {
			return fmt.Errorf("failed to unmarshal upgrade-config.json: %w", err)
		}

		foundryTomlPath := filepath.Join(currentDirPath, "foundry.toml")
		foundryToml, err := os.ReadFile(foundryTomlPath)
		if err != nil {
			return fmt.Errorf("failed to read foundry.toml: %w", err)
		}
		var foundryTomlData foundryTomlData
		err = toml.Unmarshal(foundryToml, &foundryTomlData)
		if err != nil {
			return fmt.Errorf("failed to unmarshal foundry.toml: %w", err)
		}

		for _, upgradeConfig := range upgradeConfigData {
			chain := upgradeConfig.Chain
			rpcUrl, ok := foundryTomlData.RpcEndpoints[chain]
			if !ok {
				return fmt.Errorf("rpc not found")
			}
			rpcClient, err := ethclient.Dial(rpcUrl)
			if err != nil {
				return fmt.Errorf("failed to dial rpc: %w", err)
			}
			diamondAddressParsed := common.HexToAddress(upgradeConfig.Diamond)
			resp, err := rpcClient.CallContract(ctx, ethereum.CallMsg{
				To:   &diamondAddressParsed,
				Data: facetsMethod.ID,
			}, nil)
			if err != nil {
				return fmt.Errorf("failed to call contract: %w", err)
			}
			facets, err := facetsMethod.Outputs.Unpack(resp)
			if err != nil {
				return fmt.Errorf("failed to unpack facets: %w", err)
			}
			abiFile, err := os.Open(fmt.Sprintf("target/abi/%s.json", upgradeConfig.Facet))
			if err != nil {
				return fmt.Errorf("failed to open abi file %s: %w", upgradeConfig.Facet, err)
			}
			abiParsed, err := abi.JSON(abiFile)
			if err != nil {
				return fmt.Errorf("failed to parse abi file %s: %w", upgradeConfig.Facet, err)
			}

			facetsData := facets[0].([]struct {
				FacetAddress      common.Address "json:\"facetAddress\""
				FunctionSelectors [][4]uint8     "json:\"functionSelectors\""
			})

			// 获取新ABI中的所有函数签名
			newFuncSigs := make(map[string]bool)
			for _, method := range abiParsed.Methods {
				newFuncSigs[bytes2hexFixedWidth(method.ID, 4)] = true
			}

			// 收集所有现有的函数签名
			allExistingSigs := make(map[string]bool)
			for _, facet := range facetsData {
				for _, funcSig := range facet.FunctionSelectors {
					funcSigString := bytes2hexFixedWidth(funcSig[:], 4)
					allExistingSigs[funcSigString] = true
				}
			}

			// 计算需要替换和添加的函数签名
			var toReplace []string
			var toAdd []string

			// 需要替换：新ABI中存在且现有diamond中也存在的函数签名
			for sig := range newFuncSigs {
				if allExistingSigs[sig] {
					toReplace = append(toReplace, sig)
				}
			}

			// 需要添加：新ABI中存在但现有diamond中不存在的函数签名
			for sig := range newFuncSigs {
				if !allExistingSigs[sig] {
					toAdd = append(toAdd, sig)
				}
			}

			// 记录每个facet的信息，用于找到需要替换最多的facet
			type facetInfo struct {
				address      common.Address
				replaceCount int
			}

			var facetInfos []facetInfo
			for _, facet := range facetsData {
				replaceCount := 0
				for _, funcSig := range facet.FunctionSelectors {
					funcSigString := bytes2hexFixedWidth(funcSig[:], 4)
					if newFuncSigs[funcSigString] {
						replaceCount++
					}
				}
				facetInfos = append(facetInfos, facetInfo{
					address:      facet.FacetAddress,
					replaceCount: replaceCount,
				})
			}

			// 找到需要替换最多的facet
			var maxReplaceFacet facetInfo
			maxReplaceCount := 0
			for _, info := range facetInfos {
				if info.replaceCount > maxReplaceCount {
					maxReplaceCount = info.replaceCount
					maxReplaceFacet = info
				}
			}

			// 计算需要删除的函数签名：在匹配数量最多的facet中，不在新ABI中的函数签名
			var toDelete []string
			for _, facet := range facetsData {
				if facet.FacetAddress == maxReplaceFacet.address {
					for _, funcSig := range facet.FunctionSelectors {
						funcSigString := bytes2hexFixedWidth(funcSig[:], 4)
						if !newFuncSigs[funcSigString] {
							toDelete = append(toDelete, funcSigString)
						}
					}
					break
				}
			}

			var diamondCutParams [][]interface{}

			if upgradeConfig.Replace && len(toReplace) > 0 {
				replaceParam := []interface{}{
					upgradeConfig.NewImpl,
					"1",
					toReplace,
				}
				diamondCutParams = append(diamondCutParams, replaceParam)
			}

			if upgradeConfig.Add && len(toAdd) > 0 {
				addParam := []interface{}{
					upgradeConfig.NewImpl,
					"0",
					toAdd,
				}
				diamondCutParams = append(diamondCutParams, addParam)
			}

			if upgradeConfig.Remove && len(toDelete) > 0 {
				deleteParam := []interface{}{
					common.Address{},
					"2",
					toDelete,
				}
				diamondCutParams = append(diamondCutParams, deleteParam)
			}

			diamondCutParamsJson, err := json.Marshal(diamondCutParams)
			if err != nil {
				return fmt.Errorf("failed to marshal diamond cut params: %w", err)
			}
			fmt.Printf("%s %s\n\n", chain, diamondCutParamsJson)
		}

		return nil
	},
}

func bytes2hexFixedWidth(b []byte, width int) string {
	length := width * 2
	s := common.Bytes2Hex(b)
	if len(s) < length {
		s = strings.Repeat("0", length-len(s)) + s
	}
	return "0x" + s
}

func abiStringArgToInterface(t string, v string) interface{} {
	switch t {
	case "uint", "int", "uint256", "int256":
		bn, ok := new(big.Int).SetString(v, 10)
		if !ok {
			panic(fmt.Sprintf("invalid big.Int %s", v))
		}
		return bn
	case "address":
		return common.HexToAddress(v)
	}
	panic(fmt.Sprintf("unsupported type %s", t))
}

var create2vanityCmd = &cli.Command{
	Name:  "create2vanity",
	Usage: "Generate a create2 address with a specific prefix.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "prefix",
			Aliases: []string{"p"},
			Usage:   "The prefix of the address.",
		},
		&cli.StringFlag{
			Name:    "deployer",
			Aliases: []string{"d"},
			Usage:   "The deployer address.",
		},
		&cli.StringFlag{
			Name:    "salt-prefix",
			Aliases: []string{"sp"},
			Usage:   "The prefix of the salt. keccak256(salt-prefix + randSaltSuffix)",
		},
		&cli.StringFlag{
			Name:    "contract-bin",
			Aliases: []string{"cb"},
			Usage:   "The contract binary.",
		},
		&cli.StringSliceFlag{
			Name:    "constructor-args",
			Aliases: []string{"ca"},
			Usage:   "The constructor arguments. uint256:123123",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		prefix := strings.ToLower(cmd.String("prefix"))
		deployer := cmd.String("deployer")
		saltPrefix := cmd.String("salt-prefix")
		contractBin := cmd.String("contract-bin")
		constructorArgs := cmd.StringSlice("constructor-args")

		var args abi.Arguments
		var vals []interface{}
		for _, arg := range constructorArgs {
			argParts := strings.Split(arg, ":")
			abiType, err := abi.NewType(argParts[0], "", nil)
			if err != nil {
				return err
			}
			args = append(args, abi.Argument{
				Type: abiType,
			})
			vals = append(vals, abiStringArgToInterface(argParts[0], argParts[1]))
		}
		argsPacked, err := args.Pack(vals...)
		if err != nil {
			return err
		}
		initCode := append(common.FromHex(contractBin), argsPacked...)
		initCodeHash := crypto.Keccak256(initCode)

		saltBytes32 := new([32]byte)
		for randSaltSuffix := 0; ; randSaltSuffix++ {
			saltStr := saltPrefix + fmt.Sprintf("%x", randSaltSuffix)
			salt := crypto.Keccak256([]byte(saltStr))
			copy(saltBytes32[:], salt)
			addr := crypto.CreateAddress2(common.HexToAddress(deployer), *saltBytes32, initCodeHash)
			if strings.HasPrefix(strings.ToLower(addr.String()), prefix) {
				fmt.Printf("Address: %s\nSalt: %s\n", addr.String(), saltStr)
				break
			}
		}
		return nil
	},
}

var extractFileNamePattern = regexp.MustCompile(`.*\/([^\/]*\.sol)`)

var unflattenCmd = &cli.Command{
	Name:  "unflatten",
	Usage: "Unflatten a flattened solidity file.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "file",
			Usage: "The flattened solidity file.",
		},
		&cli.StringFlag{
			Name:  "output",
			Usage: "The output directory.",
			Value: ".",
		},
		&cli.BoolFlag{
			Name:    "keep-dependencies",
			Aliases: []string{"kd"},
			Usage:   "Keep dependencies.",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		file := cmd.String("file")
		outputPath := cmd.String("output")
		keepDependencies := cmd.Bool("keep-dependencies")

		f, err := os.OpenFile(file, os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to open abi file %s: %w", file, err)
		}
		defer f.Close()

		fileInfo, err := f.Stat()
		if err != nil {
			return fmt.Errorf("failed to parse abi file %s: %w", file, err)
		}
		if fileInfo.Size() > 10*1024*1024 {
			return fmt.Errorf("file size is too large")
		}

		scanner := bufio.NewScanner(f)

		var filename, license, solcVersion string
		sb := new(strings.Builder)
		dependencies := make(map[string]string)

		writeContract := func(fName string, fContent string) error {
			fmt.Printf("Writing file %s %d\n", fName, sb.Len())

			dir := path.Dir(fName)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
			}

			var dep strings.Builder
			for k, v := range dependencies {
				if strings.Contains(fContent, k) && !strings.HasSuffix(fName, k+".sol") {
					dep.WriteString(v)
				}
			}

			return os.WriteFile(fName, []byte(license+solcVersion+dep.String()+fContent), 0644)
		}

		genDependence := func(line string) string {
			filename = extractFileNamePattern.FindStringSubmatch(line)[1]
			filepath, _, _ := strings.Cut(line[9:], "@")
			filepath = string(line[8]) + filepath
			contractName, _ := strings.CutSuffix(filename, ".sol")
			dependencies[contractName] = fmt.Sprintf("import {%s} from \"%s\";\n", contractName, filepath)
			return filepath
		}

		var filenames []string
		var fileContents []string

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "import") && !keepDependencies {
				continue
			}

			if strings.HasPrefix(line, "// SPDX-License-Identifier:") {
				license = line + "\n\n"
				continue
			}

			if strings.HasPrefix(line, "pragma solidity") {
				solcVersion = line + "\n\n"
				continue
			}

			if strings.HasPrefix(line, "// File ") {
				if filename != "" {
					filenames = append(filenames, filename)
					fileContents = append(fileContents, sb.String())
				}
				sb.Reset()
				if line[8] == '@' && !keepDependencies {
					fmt.Printf("Skipping file %s\n", line[8:])
					genDependence(line)
					filename = ""
				} else {
					filename = genDependence(line)
					filename = outputPath + "/" + filename
				}
				continue
			}
			sb.WriteString(line)
			sb.Write([]byte("\n"))
		}

		if filename != "" {
			filenames = append(filenames, filename)
			fileContents = append(fileContents, sb.String())
		}

		for i := 0; i < len(filenames); i++ {
			if err := writeContract(filenames[i], fileContents[i]); err != nil {
				return err
			}
		}
		return nil
	},
}
