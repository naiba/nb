package solana

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	lookup "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
)

type altData struct {
	address   string
	addresses solana.PublicKeySlice
	writeable []uint64
	readonly  []uint64
}

func DecodeTransactionByteByByte(
	ctx context.Context,
	rpcUrl string,
	txBase64 string,
	parseALT bool,
) error {
	// Fill fake signature
	txBytes, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return err
	}
	txBytes = fillDummySignature(txBytes)

	hexStr := hex.EncodeToString(txBytes)
	log.Printf("transaction length: %d, hex: %s", len(txBytes), hexStr)

	readIndex := 0

	readN := func(n int) string {
		result := hexStr[readIndex : readIndex+n*2]
		readIndex += n * 2
		return result
	}

	// https://github.com/anza-xyz/agave/blob/v2.1.13/short-vec/src/lib.rs
	readLen := func() struct {
		hex string
		len int
	} {
		length := 0
		hexVal := ""
		i := 0
		for {
			byteHex := readN(1)
			byteVal, _ := strconv.ParseUint(byteHex, 16, 8)
			length |= int(byteVal&0x7f) << (7 * i)
			hexVal += byteHex
			if byteVal&0x80 == 0 {
				break
			}
			i++
		}
		return struct {
			hex string
			len int
		}{hexVal, length}
	}

	signatureLenHex := readN(1)
	signatureLen, _ := strconv.ParseUint(signatureLenHex, 16, 8)
	fmt.Println(signatureLenHex, "Signature length:", signatureLen)

	for i := 0; i < int(signatureLen); i++ {
		signature := readN(64)
		fmt.Println(signature, "Signature:", i+1)
	}

	messageVersionHex := readN(1)
	messageVersion, _ := strconv.ParseUint(messageVersionHex, 16, 8)
	fmt.Println(messageVersionHex, "Message version (1 for v0, 0 for legacy):", messageVersion-127)

	messageHeaderNumRequiredSignaturesHex := readN(1)
	messageHeaderNumRequiredSignatures, _ := strconv.ParseUint(messageHeaderNumRequiredSignaturesHex, 16, 8)
	fmt.Println(messageHeaderNumRequiredSignaturesHex, "Message header - Required signatures length:", messageHeaderNumRequiredSignatures)

	messageHeaderNumReadonlySignedAccountsHex := readN(1)
	messageHeaderNumReadonlySignedAccounts, _ := strconv.ParseUint(messageHeaderNumReadonlySignedAccountsHex, 16, 8)
	fmt.Println(messageHeaderNumReadonlySignedAccountsHex, "Message header - Readonly signed accounts length:", messageHeaderNumReadonlySignedAccounts)

	messageHeaderNumReadonlyUnsignedAccountsHex := readN(1)
	messageHeaderNumReadonlyUnsignedAccounts, _ := strconv.ParseUint(messageHeaderNumReadonlyUnsignedAccountsHex, 16, 8)
	fmt.Println(messageHeaderNumReadonlyUnsignedAccountsHex, "Message header - Readonly unsigned accounts length:", messageHeaderNumReadonlyUnsignedAccounts)

	staticAccounts := []string{}
	staticAccountKeyLen := readLen()
	fmt.Println(staticAccountKeyLen.hex, "Static accounts length:", staticAccountKeyLen.len)

	for i := 0; i < staticAccountKeyLen.len; i++ {
		keyHex := readN(32)
		keyBytes, _ := hex.DecodeString(keyHex)
		key := base58.Encode(keyBytes)
		staticAccounts = append(staticAccounts, key)
		fmt.Println(keyHex, "Static account:", i+1, key)
	}

	recentBlockhashHex := readN(32)
	recentBlockhashBytes, _ := hex.DecodeString(recentBlockhashHex)
	recentBlockhash := base58.Encode(recentBlockhashBytes)
	fmt.Println(recentBlockhashHex, "Recent blockhash:", recentBlockhash)

	instructionLen := readLen()
	fmt.Println(instructionLen.hex, "Instruction length:", instructionLen.len)

	for i := 0; i < instructionLen.len; i++ {
		fmt.Println("--------", "Instruction:", i+1, "--------")

		programIdIndexHex := readN(1)
		programIdIndex, _ := strconv.ParseUint(programIdIndexHex, 16, 8)
		fmt.Println(programIdIndexHex, "Program ID index:", programIdIndex, staticAccounts[programIdIndex])

		accountIndexLen := readLen()
		fmt.Println(accountIndexLen.hex, "Account index length:", accountIndexLen.len)

		ixKeysIdxHex := ""
		ixKeysIdx := []uint64{}

		for j := 0; j < accountIndexLen.len; j++ {
			accountIndexHex := readN(1)
			accountIndex, _ := strconv.ParseUint(accountIndexHex, 16, 8)
			ixKeysIdx = append(ixKeysIdx, accountIndex)
			ixKeysIdxHex += accountIndexHex
		}
		fmt.Println(ixKeysIdxHex, "Account indices:", ixKeysIdx)

		dataLenHex := readLen()
		fmt.Println(dataLenHex.hex, "Data length:", dataLenHex.len)
		data := readN(dataLenHex.len)
		fmt.Println(data, "Data")
	}

	addressLookupTableLenHex := readN(1)
	addressLookupTableLen, _ := strconv.ParseUint(addressLookupTableLenHex, 16, 8)
	fmt.Println(addressLookupTableLenHex, "Address lookup table length:", addressLookupTableLen)

	var alts []altData

	for i := 0; i < int(addressLookupTableLen); i++ {
		address := readN(32)
		addressBytes, err := hex.DecodeString(address)
		if err != nil {
			return err
		}
		fmt.Println(address, "Address:", i+1, base58.Encode(addressBytes))

		writeableIndexLen := readLen()
		fmt.Println(writeableIndexLen.hex, "Writeable index length:", writeableIndexLen.len)

		writeableIndexHex := ""
		writeableIndex := []uint64{}

		for j := 0; j < writeableIndexLen.len; j++ {
			idxHex := readN(1)
			idx, _ := strconv.ParseUint(idxHex, 16, 8)
			writeableIndex = append(writeableIndex, idx)
			writeableIndexHex += idxHex
		}
		fmt.Println(writeableIndexHex, "Writeable indices:", writeableIndex)

		readonlyIndexLen := readLen()
		fmt.Println(readonlyIndexLen.hex, "Readonly index length:", readonlyIndexLen.len)

		readonlyIndexHex := ""
		readonlyIndex := []uint64{}

		for j := 0; j < readonlyIndexLen.len; j++ {
			idxHex := readN(1)
			idx, _ := strconv.ParseUint(idxHex, 16, 8)
			readonlyIndex = append(readonlyIndex, idx)
			readonlyIndexHex += idxHex
		}
		fmt.Println(readonlyIndexHex, "Readonly indices:", readonlyIndex)

		alts = append(alts, altData{
			address:   base58.Encode(addressBytes),
			writeable: writeableIndex,
			readonly:  readonlyIndex,
		})
	}

	if parseALT {
		rpcClient := rpc.New(rpcUrl)
		for _, alt := range alts {
			info, err := rpcClient.GetAccountInfo(
				ctx,
				solana.MustPublicKeyFromBase58(alt.address),
			)
			if err != nil {
				return err
			}

			tableContent, err := lookup.DecodeAddressLookupTableState(info.GetBinary())
			if err != nil {
				return err
			}
			alt.addresses = tableContent.Addresses
		}
		log.Println(("-------- Address Lookup Table Addresses --------"))
		keyIndex := len(staticAccounts)
		for _, alt := range alts {
			for i := 0; i < len(alt.writeable); i++ {
				log.Printf("%d %s @ %s\n", keyIndex, alt.addresses[alt.writeable[i]], alt.address)
				keyIndex++
			}
		}
		for _, alt := range alts {
			for i := 0; i < len(alt.readonly); i++ {
				log.Printf("%d %s @ %s\n", keyIndex, alt.addresses[alt.readonly[i]], alt.address)
				keyIndex++
			}
		}
	}
	return nil
}

func fillDummySignature(txBytes []byte) []byte {
	if txBytes[0] != 1 {
		log.Print("signature not found, filling with dummy")
		bytes64 := make([]byte, 64)
		newBytes := append([]byte{1}, bytes64...)
		newBytes = append(newBytes, txBytes...)
		return newBytes
	}
	return txBytes
}

func DecodeTransaction(
	ctx context.Context,
	rpcUrl string,
	txBase64 string,
	parseALT bool,
) error {
	data, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return err
	}
	data = fillDummySignature(data)

	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(data))
	if err != nil {
		return err
	}

	if parseALT {
		rpcClient := rpc.New(rpcUrl)
		err := FillAddressLookupTable(ctx, rpcClient, tx)
		if err != nil {
			return err
		}
	}

	log.Print(tx.String())
	return nil
}

func FillAddressLookupTable(ctx context.Context, rpcClient *rpc.Client, tx *solana.Transaction) error {
	tblKeys := tx.Message.GetAddressTableLookups().GetTableIDs()
	if len(tblKeys) == 0 {
		return nil
	}
	resolutions := make(map[solana.PublicKey]solana.PublicKeySlice)
	for _, key := range tblKeys {
		info, err := rpcClient.GetAccountInfo(
			ctx,
			key,
		)
		if err != nil {
			return err
		}

		tableContent, err := lookup.DecodeAddressLookupTableState(info.GetBinary())
		if err != nil {
			return err
		}

		resolutions[key] = tableContent.Addresses
	}

	err := tx.Message.SetAddressTables(resolutions)
	if err != nil {
		return err
	}

	err = tx.Message.ResolveLookups()
	if err != nil {
		return err
	}

	return nil
}
