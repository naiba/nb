package solana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

type ixData struct {
	programIdIdx int
	data         string
	accounts     []uint64
}

type messageHeader struct {
	requiredSignatures       uint64
	readonlySignedAccounts   uint64
	readonlyUnsignedAccounts uint64
}

func DecodeTransactionByteByByte(
	ctx context.Context,
	rpcUrl string,
	txBase64 string,
	parseALT bool,
	noSignature bool,
) (string, error) {
	txBytes, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return txBase64, err
	}

	if noSignature {
		txBytes = append([]byte{0}, txBytes...)
	}

	hexStr := hex.EncodeToString(txBytes)
	fmt.Printf("transaction length: %d, hex: %s", len(txBytes), hexStr)

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
		fmt.Println(signature, "Signature:", i)
	}

	messageVersionHex := readN(1)
	messageVersion, _ := strconv.ParseUint(messageVersionHex, 16, 8)
	fmt.Println(messageVersionHex, "Message version (1 for v0, 0 for legacy):", messageVersion-127)

	var messageHeader messageHeader

	messageHeaderNumRequiredSignaturesHex := readN(1)
	messageHeaderNumRequiredSignatures, _ := strconv.ParseUint(messageHeaderNumRequiredSignaturesHex, 16, 8)
	fmt.Println(messageHeaderNumRequiredSignaturesHex, "Message header - Required signatures length:", messageHeaderNumRequiredSignatures)
	messageHeader.requiredSignatures = messageHeaderNumRequiredSignatures

	messageHeaderNumReadonlySignedAccountsHex := readN(1)
	messageHeaderNumReadonlySignedAccounts, _ := strconv.ParseUint(messageHeaderNumReadonlySignedAccountsHex, 16, 8)
	fmt.Println(messageHeaderNumReadonlySignedAccountsHex, "Message header - Readonly signed accounts length:", messageHeaderNumReadonlySignedAccounts)
	messageHeader.readonlyUnsignedAccounts = messageHeaderNumReadonlySignedAccounts

	messageHeaderNumReadonlyUnsignedAccountsHex := readN(1)
	messageHeaderNumReadonlyUnsignedAccounts, _ := strconv.ParseUint(messageHeaderNumReadonlyUnsignedAccountsHex, 16, 8)
	fmt.Println(messageHeaderNumReadonlyUnsignedAccountsHex, "Message header - Readonly unsigned accounts length:", messageHeaderNumReadonlyUnsignedAccounts)
	messageHeader.readonlySignedAccounts = messageHeaderNumReadonlyUnsignedAccounts

	staticAccounts := []string{}
	staticAccountKeyLen := readLen()
	fmt.Println(staticAccountKeyLen.hex, "Static accounts length:", staticAccountKeyLen.len)

	for i := 0; i < staticAccountKeyLen.len; i++ {
		keyHex := readN(32)
		keyBytes, _ := hex.DecodeString(keyHex)
		key := base58.Encode(keyBytes)
		staticAccounts = append(staticAccounts, key)
		fmt.Println(keyHex, "Static account:", i, key)
	}

	recentBlockhashHex := readN(32)
	recentBlockhashBytes, _ := hex.DecodeString(recentBlockhashHex)
	recentBlockhash := base58.Encode(recentBlockhashBytes)
	fmt.Println(recentBlockhashHex, "Recent blockhash:", recentBlockhash)

	instructionLen := readLen()
	fmt.Println(instructionLen.hex, "Instruction length:", instructionLen.len)

	instructions := make([]ixData, instructionLen.len)

	for i := 0; i < instructionLen.len; i++ {
		fmt.Println("--------", "Instruction:", i, "--------")

		programIdIndexHex := readN(1)
		programIdIndex, err := strconv.ParseUint(programIdIndexHex, 16, 8)
		if err != nil {
			return txBase64, errors.Join(fmt.Errorf("program id index: %s", programIdIndexHex), err)
		}
		fmt.Println(programIdIndexHex, "Program ID index:", programIdIndex, staticAccounts[programIdIndex])
		instructions[i].programIdIdx = int(programIdIndex)

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
		instructions[i].accounts = ixKeysIdx

		dataLenHex := readLen()
		fmt.Println(dataLenHex.hex, "Data length:", dataLenHex.len)
		data := readN(dataLenHex.len)
		fmt.Println(data, "Data")
		instructions[i].data = data
	}

	addressLookupTableLenHex := readN(1)
	addressLookupTableLen, _ := strconv.ParseUint(addressLookupTableLenHex, 16, 8)
	fmt.Println(addressLookupTableLenHex, "Address lookup table length:", addressLookupTableLen)

	var alts []altData

	for i := 0; i < int(addressLookupTableLen); i++ {
		address := readN(32)
		addressBytes, err := hex.DecodeString(address)
		if err != nil {
			return txBase64, err
		}
		fmt.Println(address, "Address:", i, base58.Encode(addressBytes))

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

	if parseALT && len(alts) > 0 {
		rpcClient := rpc.New(rpcUrl)
		for i := 0; i < len(alts); i++ {
			info, err := rpcClient.GetAccountInfo(
				ctx,
				solana.MustPublicKeyFromBase58(alts[i].address),
			)
			if err != nil {
				return txBase64, err
			}

			tableContent, err := lookup.DecodeAddressLookupTableState(info.GetBinary())
			if err != nil {
				return txBase64, err
			}
			alts[i].addresses = tableContent.Addresses
		}
		fmt.Println(("-------- Address Lookup Table Addresses --------"))
		keyIndex := len(staticAccounts)
		var writeableAccounts, readonlyAccounts []string
		for _, alt := range alts {
			for i := 0; i < len(alt.writeable); i++ {
				fmt.Printf("%d %s @ %s\n", keyIndex, alt.addresses[alt.writeable[i]], alt.address)
				keyIndex++
				writeableAccounts = append(writeableAccounts, alt.addresses[alt.writeable[i]].String())
			}
		}
		for _, alt := range alts {
			for i := 0; i < len(alt.readonly); i++ {
				fmt.Printf("%d %s @ %s\n", keyIndex, alt.addresses[alt.readonly[i]], alt.address)
				keyIndex++
				readonlyAccounts = append(readonlyAccounts, alt.addresses[alt.readonly[i]].String())
			}
		}

		allAccounts := make([]string, len(staticAccounts)+len(writeableAccounts)+len(readonlyAccounts))
		copy(allAccounts, staticAccounts)
		copy(allAccounts[len(staticAccounts):], writeableAccounts)
		copy(allAccounts[len(staticAccounts)+len(writeableAccounts):], readonlyAccounts)

		writeableAltIdxStart, readonlyAltIdxStart := uint64(len(staticAccounts)), uint64(len(staticAccounts)+len(writeableAccounts))
		seenAccounts := make(map[string]struct{})

		fmt.Println("-------- Parsed Instruction Accounts [T] AddressLookupTable [W] writable [S] signer --------")
		for i := 0; i < len(instructions); i++ {
			fmt.Printf("Instruction %d\n", i)
			programIdAddress, programIdMeta := getAccountAddressAndMeta(uint64(instructions[i].programIdIdx), writeableAltIdxStart, readonlyAltIdxStart, &messageHeader, allAccounts)
			seenAccounts[programIdAddress] = struct{}{}
			fmt.Printf("  Program ID: %d %s %s\n", instructions[i].programIdIdx, programIdAddress, programIdMeta)
			fmt.Printf("  Data: %s\n", instructions[i].data)
			for j := 0; j < len(instructions[i].accounts); j++ {
				accountIndex := instructions[i].accounts[j]
				accountAddress, accountMeta := getAccountAddressAndMeta(accountIndex, writeableAltIdxStart, readonlyAltIdxStart, &messageHeader, allAccounts)
				seenAccounts[accountAddress] = struct{}{}
				fmt.Printf("    Account %d: %s %s\n", accountIndex, accountAddress, accountMeta)
			}
		}

		var unusedAccounts []string
		accountsSeenCount := make(map[string]int)
		for i := 0; i < len(allAccounts); i++ {
			accountAddress := allAccounts[i]
			accountsSeenCount[accountAddress]++
			if _, ok := seenAccounts[accountAddress]; !ok {
				unusedAccounts = append(unusedAccounts, accountAddress)
			}
		}

		if len(unusedAccounts) > 0 {
			fmt.Println("-------- [WARN] Unused Accounts --------")
			for _, account := range unusedAccounts {
				fmt.Printf("  %s\n", account)
			}
		}

		var repeatedAccounts []string
		for account, count := range accountsSeenCount {
			if count > 1 {
				repeatedAccounts = append(repeatedAccounts, account)
			}
		}
		if len(repeatedAccounts) > 0 {
			fmt.Println("-------- [WARN] Repeated Accounts --------")
			for _, account := range repeatedAccounts {
				fmt.Printf("  %s\n", account)
			}
		}
	}

	var finalData bytes.Buffer
	finalData.WriteByte(byte(messageHeader.requiredSignatures))
	for i := signatureLen; i < messageHeader.requiredSignatures; i++ {
		finalData.Write(make([]byte, 64))
	}
	finalData.Write(txBytes[1+signatureLen*64:])

	return base64.StdEncoding.EncodeToString(finalData.Bytes()), nil
}

func getAccountAddressAndMeta(accountIndex, writeableAltIdxStart, readonlyAltIdxStart uint64, messageHeader *messageHeader, accounts []string) (string, string) {
	var writable, signer bool
	if accountIndex < messageHeader.requiredSignatures {
		signer = true
		writable = accountIndex < messageHeader.requiredSignatures-messageHeader.readonlySignedAccounts
	} else {
		if accountIndex < writeableAltIdxStart {
			writable = accountIndex < writeableAltIdxStart-messageHeader.readonlyUnsignedAccounts
		} else if accountIndex < readonlyAltIdxStart {
			writable = true
		}
	}
	return accounts[accountIndex], getAccountMetaLabel(writable, signer, accountIndex >= writeableAltIdxStart)
}

func getAccountMetaLabel(writable bool, signer bool, fromAlt bool) string {
	var writableStr, signerStr, fromAltStr string
	if writable {
		writableStr = "W"
	}
	if signer {
		signerStr = "S"
	}
	if fromAlt {
		fromAltStr = "T"
	}
	return fmt.Sprintf("[%s%s%s]", writableStr, signerStr, fromAltStr)
}

func DecodeTransaction(
	ctx context.Context,
	rpcUrl string,
	txBase64 string,
	parseALT bool,
	noSignature bool,
) (string, error) {
	data, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return txBase64, err
	}

	if noSignature {
		data = append([]byte{0}, data...)
	}

	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(data))
	if err != nil {
		return txBase64, err
	}

	if parseALT {
		rpcClient := rpc.New(rpcUrl)
		err := FillAddressLookupTable(ctx, rpcClient, tx)
		if err != nil {
			return txBase64, err
		}
	}

	for i := len(tx.Signatures); i < int(tx.Message.Header.NumRequiredSignatures); i++ {
		tx.Signatures = append(tx.Signatures, solana.Signature{})
	}

	return tx.MustToBase64(), err
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

func Simulate(rpc string, txBase64 string) error {
	reqData := make(map[string]interface{})
	reqData["jsonrpc"] = "2.0"
	reqData["id"] = 1
	reqData["method"] = "simulateTransaction"
	reqData["params"] = []interface{}{
		txBase64,
		map[string]interface{}{
			"encoding":               "base64",
			"sigVerify":              false,
			"replaceRecentBlockhash": true,
		},
	}
	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("failed to marshal request data: %w", err)
	}
	resp, err := http.Post(rpc, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	fmt.Println(jsonPretty(respBody))
	return nil
}

func jsonPretty(data []byte) string {
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return string(data)
	}
	if prettyData, err := json.MarshalIndent(parsed, "", "  "); err == nil {
		return string(prettyData)
	}
	return string(data)
}
