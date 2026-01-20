package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v3"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, forgeCmd)
}

var forgeCmd = &cli.Command{
	Name:  "forge",
	Usage: "Forge helper.",
	Commands: []*cli.Command{
		forgeExportAbiCmd,
		forgeUnflattenCmd,
		forgeDiamondUpgradeCmd,
	},
}

type forgeArtifactData struct {
	Abi []any `json:"abi"`
}

type forgeTomlData struct {
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

var forgeExportAbiCmd = &cli.Command{
	Name: "export-abi",
	Aliases: []string{
		"ea",
	},
	Usage: "Export abi to directory.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "dist",
			Aliases: []string{"d"},
			Value:   "target/abi",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		dist := cmd.String("dist")
		subFolderContracts, err := filepath.Glob("src/*/*.sol")
		if err != nil {
			return err
		}
		mainContracts, err := filepath.Glob("src/*.sol")
		if err != nil {
			return err
		}
		contracts := append(subFolderContracts, mainContracts...)
		abis := make(map[string]struct{})
		if err := exportAbis(contracts, abis, dist); err != nil {
			return err
		}
		return clearUnusedAbis(abis, dist)
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

var forgeDiamondUpgradeCmd = &cli.Command{
	Name:  "diamond-upgrade",
	Usage: "Generate a diamond cut upgrade params.",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verify",
			Aliases: []string{"v"},
			Usage:   "Verify the upgrade after completion",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		verify := cmd.Bool("verify")
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

		forgeTomlPath := filepath.Join(currentDirPath, "foundry.toml")
		forgeToml, err := os.ReadFile(forgeTomlPath)
		if err != nil {
			return fmt.Errorf("failed to read foundry.toml: %w", err)
		}
		var forgeTomlData forgeTomlData
		err = toml.Unmarshal(forgeToml, &forgeTomlData)
		if err != nil {
			return fmt.Errorf("failed to unmarshal foundry.toml: %w", err)
		}

		// 定义结果结构
		type upgradeResult struct {
			Chain          string
			UpgradeParams  [][]interface{}
			RollbackParams [][]interface{}
			VerifySuccess  bool
			Error          error
		}

		var results []upgradeResult

		// 处理每个升级配置
		for i, upgradeConfig := range upgradeConfigData {
			chain := upgradeConfig.Chain
			fmt.Printf("\r🔄 Processing chain %s (%d/%d)...", chain, i+1, len(upgradeConfigData))

			result := upgradeResult{Chain: chain}

			// 获取RPC连接
			rpcUrl, ok := forgeTomlData.RpcEndpoints[chain]
			if !ok {
				result.Error = fmt.Errorf("rpc not found for chain %s", chain)
				results = append(results, result)
				continue
			}

			rpcClient, err := ethclient.Dial(rpcUrl)
			if err != nil {
				result.Error = fmt.Errorf("failed to dial rpc: %w", err)
				results = append(results, result)
				continue
			}

			// 获取diamond facets信息
			diamondAddressParsed := common.HexToAddress(upgradeConfig.Diamond)
			resp, err := rpcClient.CallContract(ctx, ethereum.CallMsg{
				To:   &diamondAddressParsed,
				Data: facetsMethod.ID,
			}, nil)
			if err != nil {
				result.Error = fmt.Errorf("failed to call contract: %w", err)
				results = append(results, result)
				continue
			}

			facets, err := facetsMethod.Outputs.Unpack(resp)
			if err != nil {
				result.Error = fmt.Errorf("failed to unpack facets: %w", err)
				results = append(results, result)
				continue
			}

			// 读取ABI文件
			abiFile, err := os.Open(fmt.Sprintf("target/abi/%s.json", upgradeConfig.Facet))
			if err != nil {
				result.Error = fmt.Errorf("failed to open abi file %s: %w", upgradeConfig.Facet, err)
				results = append(results, result)
				continue
			}

			abiParsed, err := abi.JSON(abiFile)
			abiFile.Close()
			if err != nil {
				result.Error = fmt.Errorf("failed to parse abi file %s: %w", upgradeConfig.Facet, err)
				results = append(results, result)
				continue
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
				// 检查需要替换的function selector对应的现有实现地址
				newImplAddress := common.HexToAddress(upgradeConfig.NewImpl)
				var actualToReplace []string

				// 创建function selector到facet地址的映射
				selectorToFacet := make(map[string]common.Address)
				for _, facet := range facetsData {
					for _, funcSig := range facet.FunctionSelectors {
						funcSigString := bytes2hexFixedWidth(funcSig[:], 4)
						selectorToFacet[funcSigString] = facet.FacetAddress
					}
				}

				// 只替换那些现有实现地址与新地址不同的function selector
				for _, selector := range toReplace {
					if existingFacet, exists := selectorToFacet[selector]; exists {
						if existingFacet != newImplAddress {
							actualToReplace = append(actualToReplace, selector)
						}
					}
				}

				if len(actualToReplace) > 0 {
					replaceParam := []interface{}{
						upgradeConfig.NewImpl,
						"1",
						actualToReplace,
					}
					diamondCutParams = append(diamondCutParams, replaceParam)
				}
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

			result.UpgradeParams = diamondCutParams

			// 生成回滚参数
			result.RollbackParams = generateRollbackParams(upgradeConfig, facetsData, toReplace, toAdd, toDelete)

			if verify {
				// 如果是验证模式，执行升级验证
				if err := verifyDiamondUpgrade(upgradeConfig, diamondCutParams); err != nil {
					result.Error = fmt.Errorf("verification failed: %w", err)
				} else {
					result.VerifySuccess = true
				}
			}

			results = append(results, result)
			fmt.Printf("\r✅ Completed chain %s (%d/%d)\n", chain, i+1, len(upgradeConfigData))
		}

		// 输出最终结果
		fmt.Printf("\n📋 Upgrade Results:\n")
		fmt.Printf("==================\n")

		for _, result := range results {
			if result.Error != nil {
				fmt.Printf("❌ %s: %v\n", result.Chain, result.Error)
				continue
			}

			if verify {
				if result.VerifySuccess {
					fmt.Printf("✅ %s: Upgrade verification passed\n", result.Chain)
				} else {
					fmt.Printf("❌ %s: Upgrade verification failed\n", result.Chain)
				}
			} else {
				// 输出升级参数
				upgradeParamsJson, err := json.Marshal(result.UpgradeParams)
				if err != nil {
					fmt.Printf("❌ %s: Failed to marshal upgrade params: %v\n", result.Chain, err)
					continue
				}
				fmt.Printf("🔧 %s upgrade: %s\n", result.Chain, upgradeParamsJson)

				// 输出rollback参数
				rollbackParamsJson, err := json.Marshal(result.RollbackParams)
				if err != nil {
					fmt.Printf("❌ %s: Failed to marshal rollback params: %v\n", result.Chain, err)
					continue
				}
				fmt.Printf("🔄 %s rollback: %s\n", result.Chain, rollbackParamsJson)
			}
		}

		return nil
	},
}

var extractFileNamePattern = regexp.MustCompile(`.*\/([^\/]*\.sol)`)

var forgeUnflattenCmd = &cli.Command{
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

func exportAbis(contracts []string, abis map[string]struct{}, dist string) error {
	var errs []error
	for _, contract := range contracts {
		if !contractHasAnyEntities(contract) {
			fmt.Printf("⏭️  %s has no entities, skipping\n", contract)
			continue
		}

		contractPath := strings.TrimPrefix(contract, "src/")
		contractName, contractArtifact, err := getContractArtifact(contractPath)
		if err != nil {
			fmt.Printf("❌ %s: %v\n", contract, err)
			errs = append(errs, fmt.Errorf("%s: %w", contract, err))
			continue
		}
		artifact, err := os.ReadFile(contractArtifact)
		if err != nil {
			fmt.Printf("❌ %s: failed to read artifact: %v\n", contract, err)
			errs = append(errs, fmt.Errorf("%s: %w", contract, err))
			continue
		}
		var artifactData forgeArtifactData
		err = json.Unmarshal(artifact, &artifactData)
		if err != nil {
			fmt.Printf("❌ %s: failed to parse artifact: %v\n", contract, err)
			errs = append(errs, fmt.Errorf("%s: %w", contract, err))
			continue
		}
		targetAbiFolder := filepath.Join(dist, filepath.Dir(contractPath))
		_, err = os.Stat(targetAbiFolder)
		if os.IsNotExist(err) {
			os.MkdirAll(targetAbiFolder, 0755)
		} else if err != nil {
			fmt.Printf("❌ %s: failed to check target folder: %v\n", contract, err)
			errs = append(errs, fmt.Errorf("%s: %w", contract, err))
			continue
		}
		targetAbiFile := filepath.Join(targetAbiFolder, fmt.Sprintf("%s.json", contractName))
		abiBytes, err := json.MarshalIndent(artifactData.Abi, "", "  ")
		if err != nil {
			fmt.Printf("❌ %s: failed to marshal abi: %v\n", contract, err)
			errs = append(errs, fmt.Errorf("%s: %w", contract, err))
			continue
		}
		err = os.WriteFile(targetAbiFile, abiBytes, 0644)
		if err != nil {
			fmt.Printf("❌ %s: failed to write abi: %v\n", contract, err)
			errs = append(errs, fmt.Errorf("%s: %w", contract, err))
			continue
		}
		abis[targetAbiFile] = struct{}{}
		fmt.Printf("✅ %s\n", contractName)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to export %d contract(s)", len(errs))
	}
	return nil
}

func contractHasAnyEntities(contract string) bool {
	contractContent, err := os.ReadFile(contract)
	if err != nil {
		return false
	}
	return bytes.Contains(contractContent, []byte("interface ")) ||
		bytes.Contains(contractContent, []byte("library ")) ||
		bytes.Contains(contractContent, []byte("contract "))
}

func getContractArtifact(contract string) (contractName string, contractArtifact string, err error) {
	contractName = strings.TrimSuffix(filepath.Base(contract), ".sol")
	contractArtifact = filepath.Join("out", contract, fmt.Sprintf("%s.json", contractName))
	_, err = os.Stat(contractArtifact)
	if err != nil && os.IsNotExist(err) {
		contractArtifactMain := filepath.Join("out", filepath.Base(contract), fmt.Sprintf("%s.json", contractName))
		_, err = os.Stat(contractArtifactMain)
		if err != nil {
			return "", "", fmt.Errorf("contract artifact %s in %s does not exist", contract, contractArtifactMain)
		}
		contractArtifact = contractArtifactMain
	}
	return
}

func clearUnusedAbis(abis map[string]struct{}, dist string) error {
	subFolderAbis, err := filepath.Glob(fmt.Sprintf("%s/*/*.json", dist))
	if err != nil {
		return err
	}
	mainAbis, err := filepath.Glob(fmt.Sprintf("%s/*.json", dist))
	if err != nil {
		return err
	}
	allAbis := append(subFolderAbis, mainAbis...)
	for _, abi := range allAbis {
		if _, ok := abis[abi]; !ok {
			if err := os.Remove(abi); err != nil {
				return errors.Join(fmt.Errorf("failed to remove abi %s", abi), err)
			}
		}
	}
	return nil
}

func bytes2hexFixedWidth(b []byte, width int) string {
	length := width * 2
	s := common.Bytes2Hex(b)
	if len(s) < length {
		s = strings.Repeat("0", length-len(s)) + s
	}
	return "0x" + s
}

// verifyDiamondUpgrade 验证diamond升级是否正确
// 这是事后校验：期望 upgradeConfig 中对应的项的数据为空而不是非空
func verifyDiamondUpgrade(upgradeConfig struct {
	Chain   string `json:"chain,omitempty"`
	Diamond string `json:"diamond,omitempty"`
	Facet   string `json:"facet,omitempty"`
	NewImpl string `json:"new_impl,omitempty"`
	Replace bool   `json:"replace,omitempty"`
	Add     bool   `json:"add,omitempty"`
	Remove  bool   `json:"remove,omitempty"`
}, diamondCutParams [][]interface{}) error {
	// 检查diamondCutParams中的条目
	hasReplace := false
	hasAdd := false
	hasRemove := false

	for _, param := range diamondCutParams {
		if len(param) >= 2 {
			action := param[1].(string)
			switch action {
			case "1": // Replace
				hasReplace = true
			case "0": // Add
				hasAdd = true
			case "2": // Remove
				hasRemove = true
			}
		}
	}

	// 事后校验：期望 upgradeConfig 中对应的项的数据为空而不是非空
	// 如果配置了某个操作但diamondCutParams中有对应的条目，说明升级后还有需要操作的内容，这是不正确的
	if upgradeConfig.Replace && hasReplace {
		return fmt.Errorf("replace operation is configured but replace entry found in diamondCutParams - upgrade is incorrect, please check")
	}

	if upgradeConfig.Add && hasAdd {
		return fmt.Errorf("add operation is configured but add entry found in diamondCutParams - upgrade is incorrect, please check")
	}

	if upgradeConfig.Remove && hasRemove {
		return fmt.Errorf("remove operation is configured but remove entry found in diamondCutParams - upgrade is incorrect, please check")
	}

	return nil
}

// generateRollbackParams 生成rollback参数
func generateRollbackParams(upgradeConfig struct {
	Chain   string `json:"chain,omitempty"`
	Diamond string `json:"diamond,omitempty"`
	Facet   string `json:"facet,omitempty"`
	NewImpl string `json:"new_impl,omitempty"`
	Replace bool   `json:"replace,omitempty"`
	Add     bool   `json:"add,omitempty"`
	Remove  bool   `json:"remove,omitempty"`
}, facetsData []struct {
	FacetAddress      common.Address "json:\"facetAddress\""
	FunctionSelectors [][4]uint8     "json:\"functionSelectors\""
}, toReplace, toAdd, toDelete []string) [][]interface{} {
	var rollbackParams [][]interface{}

	// 创建function selector到facet地址的映射
	selectorToFacet := make(map[string]common.Address)
	for _, facet := range facetsData {
		for _, funcSig := range facet.FunctionSelectors {
			funcSigString := bytes2hexFixedWidth(funcSig[:], 4)
			selectorToFacet[funcSigString] = facet.FacetAddress
		}
	}

	// 1. Replace操作的回滚：将替换的函数恢复到原来的地址
	if upgradeConfig.Replace && len(toReplace) > 0 {
		// 找到这些函数原来的地址
		originalAddresses := make(map[string]common.Address)
		for _, selector := range toReplace {
			if originalAddr, exists := selectorToFacet[selector]; exists {
				originalAddresses[selector] = originalAddr
			}
		}

		// 按地址分组，生成rollback参数
		addressToSelectors := make(map[common.Address][]string)
		for selector, addr := range originalAddresses {
			addressToSelectors[addr] = append(addressToSelectors[addr], selector)
		}

		for addr, selectors := range addressToSelectors {
			rollbackParam := []interface{}{
				addr.Hex(),
				1, // Replace
				selectors,
			}
			rollbackParams = append(rollbackParams, rollbackParam)
		}
	}

	// 2. Add操作的回滚：删除新添加的函数
	if upgradeConfig.Add && len(toAdd) > 0 {
		rollbackParam := []interface{}{
			common.Address{},
			2, // Remove
			toAdd,
		}
		rollbackParams = append(rollbackParams, rollbackParam)
	}

	// 3. Remove操作的回滚：重新添加被删除的函数
	if upgradeConfig.Remove && len(toDelete) > 0 {
		// 找到这些函数原来的地址
		originalAddresses := make(map[string]common.Address)
		for _, selector := range toDelete {
			if originalAddr, exists := selectorToFacet[selector]; exists {
				originalAddresses[selector] = originalAddr
			}
		}

		// 按地址分组，生成rollback参数
		addressToSelectors := make(map[common.Address][]string)
		for selector, addr := range originalAddresses {
			addressToSelectors[addr] = append(addressToSelectors[addr], selector)
		}

		for addr, selectors := range addressToSelectors {
			rollbackParam := []interface{}{
				addr.Hex(),
				0, // Add
				selectors,
			}
			rollbackParams = append(rollbackParams, rollbackParam)
		}
	}

	return rollbackParams
}
