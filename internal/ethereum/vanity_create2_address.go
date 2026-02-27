package ethereum

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/naiba/nb/model"
)

type Create2AddressData struct {
	address    common.Address
	saltStr    string
	saltSuffix uint64
}

// Create2AddressGenerator generates CREATE2 contract addresses
type Create2AddressGenerator struct {
	deployer     common.Address
	saltPrefix   string
	initCodeHash []byte
	counter      *atomic.Uint64
}

func NewCreate2AddressGenerator(deployer, saltPrefix, contractBin string, constructorArgs []string) (*Create2AddressGenerator, error) {
	// Parse constructor arguments
	var args abi.Arguments
	var vals []interface{}
	for _, arg := range constructorArgs {
		argParts := strings.Split(arg, ":")
		if len(argParts) != 2 {
			return nil, fmt.Errorf("invalid constructor argument format: %s (expected type:value)", arg)
		}
		abiType, err := abi.NewType(argParts[0], "", nil)
		if err != nil {
			return nil, fmt.Errorf("invalid ABI type %s: %w", argParts[0], err)
		}
		args = append(args, abi.Argument{
			Type: abiType,
		})
		vals = append(vals, abiStringArgToInterface(argParts[0], argParts[1]))
	}

	// Pack constructor arguments
	argsPacked, err := args.Pack(vals...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack constructor arguments: %w", err)
	}

	// Prepare init code
	initCode := append(common.FromHex(contractBin), argsPacked...)
	initCodeHash := crypto.Keccak256(initCode)

	return &Create2AddressGenerator{
		deployer:     common.HexToAddress(deployer),
		saltPrefix:   saltPrefix,
		initCodeHash: initCodeHash,
		counter:      &atomic.Uint64{},
	}, nil
}

func (g *Create2AddressGenerator) Generate() (string, interface{}, error) {
	saltSuffix := g.counter.Add(1) - 1
	saltStr := g.saltPrefix + fmt.Sprintf("%x", saltSuffix)
	salt := crypto.Keccak256([]byte(saltStr))

	saltBytes32 := new([32]byte)
	copy(saltBytes32[:], salt)

	addr := crypto.CreateAddress2(g.deployer, *saltBytes32, g.initCodeHash)

	// Get checksum address (EIP-55 format)
	addressChecksum := addr.Hex()
	addressHex := addressChecksum[2:] // Remove 0x prefix

	return addressHex, &Create2AddressData{
		address:    addr,
		saltStr:    saltStr,
		saltSuffix: saltSuffix,
	}, nil
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

func VanityCreate2Address(config *model.VanityConfig, deployer, saltPrefix, contractBin string, constructorArgs []string) error {
	log.Printf("REMINDER: Ethereum addresses only contain hexadecimal characters (0-9, a-f, A-F)")
	log.Printf("Searching for CREATE2 address with deployer: %s", deployer)

	if config.Contains != "" {
		validHexChars := "0123456789abcdefABCDEF"
		for _, char := range config.Contains {
			if !strings.ContainsRune(validHexChars, char) {
				return fmt.Errorf("contains illegal character: %c (Ethereum addresses only contain 0-9, a-f, A-F)", char)
			}
		}
	}

	if config.Mask != nil {
		log.Printf("Mask: 0x%x", config.Mask)
		log.Printf("MaskValue: 0x%x", config.MaskValue)
	}

	// Create generator
	generator, err := NewCreate2AddressGenerator(deployer, saltPrefix, contractBin, constructorArgs)
	if err != nil {
		return err
	}

	searcher := model.NewVanitySearcher(config, generator)

	// Search
	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	// Output result
	data := result.Data.(*Create2AddressData)

	// Compute salt hash only when found
	salt := crypto.Keccak256([]byte(data.saltStr))

	log.Printf("Address: %s", data.address.Hex())
	log.Printf("Salt: %s", data.saltStr)
	log.Printf("Salt (keccak256): 0x%x", salt)

	return nil
}
