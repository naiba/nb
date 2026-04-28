package ethereum

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/naiba/nb/model"
)

type Create2AddressData struct {
	addrBytes  [20]byte
	saltStr    string
	saltSuffix uint64
}

// Address returns the EIP-55 checksummed 0x-prefixed address.
func (d *Create2AddressData) Address() string {
	return common.Address(d.addrBytes).Hex()
}

// AddressBytes returns the raw 20-byte address.
func (d *Create2AddressData) AddressBytes() [20]byte {
	return d.addrBytes
}

// SaltString returns the salt (pre-hash) string used to produce the address.
func (d *Create2AddressData) SaltString() string {
	return d.saltStr
}

// Create2AddressGenerator mines CREATE2 addresses via EIP-1014:
//
//	addr = keccak256(0xff || deployer(20) || salt(32) || initCodeHash(32))[12:]
//
// deployer and initCodeHash are fixed per session, so the 85-byte hash input
// is pre-built at construction; the hot path only rewrites the salt window.
// maxSaltPrefixLen bounds the CLI `--salt-prefix` so the hot-path stack buffer
// (saltBuf in Generate) can always fit the prefix + hex(uint64) suffix.
// 16 = max hex chars of uint64. Keep saltBuf capacity in sync.
const maxSaltPrefixLen = saltBufLen - 16

// saltBufLen must be >= maxSaltPrefixLen + 16.
const saltBufLen = 64

type Create2AddressGenerator struct {
	counter    atomic.Uint64
	saltPrefix string

	// hashInputTemplate layout:
	//   [0]     0xff
	//   [1:21]  deployer
	//   [21:53] salt (mutated per call via keccak256 of saltStr)
	//   [53:85] initCodeHash
	// Read-only after init; each call copies it to a local buffer before patching salt.
	hashInputTemplate [85]byte
}

func NewCreate2AddressGenerator(deployer, saltPrefix, contractBin string, constructorArgs []string) (*Create2AddressGenerator, error) {
	// Bound the CLI-supplied prefix so Generate's fixed [saltBufLen]byte buffer
	// can always hold prefix + hex(uint64). Without this check, a long prefix
	// would cause `saltBuf[:n]` to panic with "slice bounds out of range" once
	// the counter grew enough hex digits to push n past saltBufLen.
	if len(saltPrefix) > maxSaltPrefixLen {
		return nil, fmt.Errorf("salt-prefix too long: %d bytes, max %d", len(saltPrefix), maxSaltPrefixLen)
	}

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
		args = append(args, abi.Argument{Type: abiType})
		vals = append(vals, abiStringArgToInterface(argParts[0], argParts[1]))
	}

	argsPacked, err := args.Pack(vals...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack constructor arguments: %w", err)
	}

	initCode := append(common.FromHex(contractBin), argsPacked...)
	initCodeHash := crypto.Keccak256(initCode)

	g := &Create2AddressGenerator{saltPrefix: saltPrefix}
	g.hashInputTemplate[0] = 0xff
	copy(g.hashInputTemplate[1:21], common.HexToAddress(deployer).Bytes())
	// salt window [21:53] is filled per-call
	copy(g.hashInputTemplate[53:85], initCodeHash)
	return g, nil
}

func (g *Create2AddressGenerator) Generate() (string, interface{}, error) {
	saltSuffix := g.counter.Add(1) - 1

	// Build "saltPrefix<hex(suffix)>" into a stack buffer, avoiding fmt.Sprintf.
	// NewCreate2AddressGenerator enforces len(saltPrefix) <= maxSaltPrefixLen,
	// so prefix + 16 hex chars (max for uint64) always fits in saltBuf.
	var saltBuf [saltBufLen]byte
	n := copy(saltBuf[:], g.saltPrefix)
	n += len(strconv.AppendUint(saltBuf[n:n], saltSuffix, 16))
	saltStr := string(saltBuf[:n])

	saltHash := crypto.Keccak256Hash([]byte(saltStr))

	hashInput := g.hashInputTemplate
	copy(hashInput[21:53], saltHash[:])
	out := crypto.Keccak256Hash(hashInput[:])

	var addr [20]byte
	copy(addr[:], out[12:32])

	var hexBuf [40]byte
	hex.Encode(hexBuf[:], addr[:])

	return string(hexBuf[:]), &Create2AddressData{
		addrBytes:  addr,
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

	if err := validateHexContains(config.Contains); err != nil {
		return err
	}

	if config.Mask != nil {
		log.Printf("Mask: 0x%x", config.Mask)
		log.Printf("MaskValue: 0x%x", config.MaskValue)
	}

	generator, err := NewCreate2AddressGenerator(deployer, saltPrefix, contractBin, constructorArgs)
	if err != nil {
		return err
	}

	searcher := model.NewVanitySearcher(config, generator).WithChecksum(EIP55Checksum)

	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	data := result.Data.(*Create2AddressData)
	salt := crypto.Keccak256([]byte(data.saltStr))

	log.Printf("Address: %s", data.Address())
	log.Printf("Salt: %s", data.saltStr)
	log.Printf("Salt (keccak256): 0x%x", salt)

	return nil
}
