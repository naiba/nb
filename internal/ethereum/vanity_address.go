package ethereum

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/naiba/nb/model"
)

// EthereumAddressGenerator wraps SecpKeyGenerator and hex-encodes the 20-byte
// address for the matcher.
type EthereumAddressGenerator struct {
	*SecpKeyGenerator
}

// EthereumAddressData holds the private-key seed and raw address. EIP-55 and
// the *ecdsa.PrivateKey are computed lazily so the hot loop doesn't pay.
type EthereumAddressData struct {
	seed      [32]byte
	addrBytes [20]byte
}

func (d *EthereumAddressData) Address() string {
	return common.Address(d.addrBytes).Hex()
}

func (d *EthereumAddressData) PrivateKey() (*ecdsa.PrivateKey, error) {
	return crypto.ToECDSA(d.seed[:])
}

func NewEthereumAddressGenerator() (*EthereumAddressGenerator, error) {
	kg, err := NewSecpKeyGenerator()
	if err != nil {
		return nil, err
	}
	return &EthereumAddressGenerator{SecpKeyGenerator: kg}, nil
}

func newEthereumAddressGeneratorFromSeed(seed [32]byte) *EthereumAddressGenerator {
	return &EthereumAddressGenerator{SecpKeyGenerator: newSecpKeyGeneratorFromSeed(seed)}
}

func (g *EthereumAddressGenerator) Generate() (string, interface{}, error) {
	seed, addr, err := g.Next()
	if err != nil {
		return "", nil, err
	}
	var hexBuf [40]byte
	hex.Encode(hexBuf[:], addr[:])
	return string(hexBuf[:]), &EthereumAddressData{
		seed:      seed,
		addrBytes: addr,
	}, nil
}

func VanityAddress(config *model.VanityConfig) error {
	log.Printf("REMINDER: Ethereum addresses only contain hexadecimal characters (0-9, a-f, A-F)")

	if err := validateHexContains(config.Contains); err != nil {
		return err
	}

	if config.Mask != nil {
		log.Printf("Mask: 0x%x", config.Mask)
		log.Printf("MaskValue: 0x%x", config.MaskValue)
	}

	generator, err := NewEthereumAddressGenerator()
	if err != nil {
		return err
	}

	logSearchEstimate(curveOrderBigInt, config.Threads)

	// Generator returns lowercase hex; WithChecksum defers EIP-55 until the
	// lowercase prefilter hits.
	searcher := model.NewVanitySearcher(config, generator).WithChecksum(EIP55Checksum)

	result, err := searcher.Search(context.Background())
	if err != nil {
		return err
	}

	data := result.Data.(*EthereumAddressData)
	privateKey, err := data.PrivateKey()
	if err != nil {
		return err
	}
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))

	log.Printf("Address: %s", data.Address())
	log.Printf("Private Key (hex): %s", privateKeyHex)
	log.Printf("Private Key (with 0x prefix): 0x%s", privateKeyHex)

	return nil
}
