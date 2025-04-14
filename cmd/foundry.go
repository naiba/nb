package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, foundryCmd)
}

var foundryCmd = &cli.Command{
	Name:  "foundry",
	Usage: "Foundry helper.",
	Subcommands: []*cli.Command{
		foundryExportAbiCmd,
	},
}

type foundryArtifactData struct {
	Abi []any `json:"abi"`
}

var foundryExportAbiCmd = &cli.Command{
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
	Action: func(c *cli.Context) error {
		dist := c.String("dist")
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

func exportAbis(contracts []string, abis map[string]struct{}, dist string) error {
	for _, contract := range contracts {
		contract = strings.TrimPrefix(contract, "src/")
		contractName, contractArtifact, err := getContractArtifact(contract)
		if err != nil {
			return err
		}
		artifact, err := os.ReadFile(contractArtifact)
		if err != nil {
			return err
		}
		var artifactData foundryArtifactData
		err = json.Unmarshal(artifact, &artifactData)
		if err != nil {
			return err
		}
		targetAbiFolder := filepath.Join(dist, filepath.Dir(contract))
		_, err = os.Stat(targetAbiFolder)
		if os.IsNotExist(err) {
			os.MkdirAll(targetAbiFolder, 0755)
		} else if err != nil {
			return err
		}
		targetAbiFile := filepath.Join(targetAbiFolder, fmt.Sprintf("%s.json", contractName))
		abiBytes, err := json.MarshalIndent(artifactData.Abi, "", "  ")
		if err != nil {
			return err
		}
		err = os.WriteFile(targetAbiFile, abiBytes, 0644)
		if err != nil {
			return err
		}
		abis[targetAbiFile] = struct{}{}
	}
	return nil
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
