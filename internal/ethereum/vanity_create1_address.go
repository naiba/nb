package ethereum

import (
	"context"
	"encoding/hex"
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/naiba/nb/model"
)

type Create1AddressData struct {
	seed              [32]byte
	deployerAddrBytes [20]byte
	contractAddrBytes [20]byte
}

// DeployerAddress returns the EIP-55 checksummed deployer address (0x-prefixed).
func (d *Create1AddressData) DeployerAddress() string {
	return common.Address(d.deployerAddrBytes).Hex()
}

// ContractAddress returns the EIP-55 checksummed contract address (0x-prefixed).
func (d *Create1AddressData) ContractAddress() string {
	return common.Address(d.contractAddrBytes).Hex()
}

// PrivateKeyBytes returns the 32-byte private key of the deployer.
func (d *Create1AddressData) PrivateKeyBytes() []byte {
	return d.seed[:]
}

// Create1AddressGenerator generates CREATE1 contract addresses (nonce=0).
// Wraps SecpKeyGenerator with an extra fixed RLP+Keccak step to turn the
// deployer address into the nonce-0 contract address.
type Create1AddressGenerator struct {
	*SecpKeyGenerator
}

func NewCreate1AddressGenerator() (*Create1AddressGenerator, error) {
	kg, err := NewSecpKeyGenerator()
	if err != nil {
		return nil, err
	}
	return &Create1AddressGenerator{SecpKeyGenerator: kg}, nil
}

func newCreate1AddressGeneratorFromSeed(seed [32]byte) *Create1AddressGenerator {
	return &Create1AddressGenerator{SecpKeyGenerator: newSecpKeyGeneratorFromSeed(seed)}
}

func (g *Create1AddressGenerator) Generate() (string, interface{}, error) {
	seed, deployerAddr, err := g.Next()
	if err != nil {
		return "", nil, err
	}

	contractAddr := computeCreate1AddressBytes(deployerAddr)

	var hexBuf [40]byte
	hex.Encode(hexBuf[:], contractAddr[:])
	return string(hexBuf[:]), &Create1AddressData{
		seed:              seed,
		deployerAddrBytes: deployerAddr,
		contractAddrBytes: contractAddr,
	}, nil
}

// computeCreate1AddressBytes returns keccak256(rlp([deployer, 0]))[12:].
//
// The RLP encoding is fixed-shape, so we build it manually to avoid
// rlp.EncodeToBytes's reflect overhead. Layout (23 bytes):
//
//	0xd6       list header, 22-byte payload (0xc0 + 22)
//	0x94       string header, 20-byte address (0x80 + 20)
//	addr[0:20]
//	0x80       zero nonce encoded as empty string
func computeCreate1AddressBytes(deployer [20]byte) [20]byte {
	var rlpBuf [23]byte
	rlpBuf[0] = 0xd6
	rlpBuf[1] = 0x94
	copy(rlpBuf[2:22], deployer[:])
	rlpBuf[22] = 0x80

	hash := crypto.Keccak256Hash(rlpBuf[:])
	var contract [20]byte
	copy(contract[:], hash[12:32])
	return contract
}

func VanityCreate1Address(config *model.VanityConfig) error {
	log.Printf("REMINDER: Ethereum addresses only contain hexadecimal characters (0-9, a-f, A-F)")
	log.Printf("Searching for contract address (first deployment, nonce=0) containing: %s", config.Contains)

	if err := validateHexContains(config.Contains); err != nil {
		return err
	}

	if config.Mask != nil {
		log.Printf("Mask: 0x%x", config.Mask)
		log.Printf("MaskValue: 0x%x", config.MaskValue)
	}

	generator, err := NewCreate1AddressGenerator()
	if err != nil {
		return err
	}

	logSearchEstimate(curveOrderBigInt, config.Threads)

	searcher := model.NewVanitySearcher(config, generator).WithChecksum(EIP55Checksum)

	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	data := result.Data.(*Create1AddressData)
	privateKeyHex := hex.EncodeToString(data.PrivateKeyBytes())

	log.Printf("Deployer Address: %s", data.DeployerAddress())
	log.Printf("Contract Address (first deployment, nonce=0): %s", data.ContractAddress())
	log.Printf("Private Key (hex): %s", privateKeyHex)
	log.Printf("Private Key (with 0x prefix): 0x%s", privateKeyHex)

	return nil
}
