package solana

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"slices"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
	"github.com/shopspring/decimal"
)

type txContext struct {
	Signature string
	Account   string
	Idx       int
	Amount    *big.Int
}

// findAssociatedTokenAddressForProgram 根据指定的 token program ID 计算 associated token address
func findAssociatedTokenAddressForProgram(
	walletAddress solana.PublicKey,
	splTokenMintAddress solana.PublicKey,
	tokenProgramID solana.PublicKey,
) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress([][]byte{
		walletAddress[:],
		tokenProgramID[:],
		splTokenMintAddress[:],
	},
		solana.SPLAssociatedTokenAccountProgramID,
	)
}

func CheckSandwichAttack(ctx context.Context, rpcUrl string, signatureString string, userAddress string, tokenAddress string, maxCheckTxCount int) error {
	addressToCheck := solana.MustPublicKeyFromBase58(userAddress)
	tokenToCheck := solana.MustPublicKeyFromBase58(tokenAddress)
	signature := solana.MustSignatureFromBase58(signatureString)

	maxSupportedTransactionVersion := uint64(0)

	rpcClient := rpc.New(rpcUrl)

	// 获取 token mint 的账户信息，判断是否为 Token2022
	mintAccountInfo, err := rpcClient.GetAccountInfo(ctx, tokenToCheck)
	if err != nil {
		return fmt.Errorf("failed to get token mint account info: %w", err)
	}
	if mintAccountInfo.Value == nil {
		return fmt.Errorf("token mint account not found")
	}

	// 根据 token owner 判断是否需要使用 Token2022
	var tokenAccountToCheck solana.PublicKey
	isToken2022 := mintAccountInfo.Value.Owner.Equals(solana.Token2022ProgramID)
	if isToken2022 {
		// 使用 Token2022 计算 associated token address
		tokenAccountToCheck, _, err = findAssociatedTokenAddressForProgram(
			addressToCheck,
			tokenToCheck,
			solana.Token2022ProgramID,
		)
		if err != nil {
			return fmt.Errorf("failed to find associated token address for Token2022: %w", err)
		}
	} else {
		// 使用标准 Token 程序计算 associated token address
		tokenAccountToCheck, _, err = solana.FindAssociatedTokenAddress(
			addressToCheck,
			tokenToCheck,
		)
		if err != nil {
			return fmt.Errorf("failed to find associated token address: %w", err)
		}
	}

	getTxRet, err := rpcClient.GetTransaction(
		ctx,
		signature,
		&rpc.GetTransactionOpts{
			MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get transaction request: %w", err)
	}

	tx, err := getTxRet.Transaction.GetTransaction()
	if err != nil {
		return fmt.Errorf("failed to get transaction decode: %w", err)
	}

	// 打印核心参数
	log.Println("========== 交易检查核心参数 ==========")
	log.Printf("区块号 (Slot): %d", getTxRet.Slot)
	log.Printf("用户地址: %s", addressToCheck.String())
	log.Printf("Token Mint 地址: %s", tokenToCheck.String())
	log.Printf("Token 类型: %s", func() string {
		if isToken2022 {
			return "Token2022"
		}
		return "Token (标准)"
	}())
	log.Printf("用户 ATA 账户: %s", tokenAccountToCheck.String())
	log.Printf("交易签名: %s", signatureString)
	log.Println("=====================================")

	var addressIdx int = -1
	for i, acc := range tx.Message.AccountKeys {
		if acc.Equals(tokenAccountToCheck) {
			addressIdx = i
			break
		}
	}
	if addressIdx == -1 {
		return fmt.Errorf("address not found in transaction")
	}

	var balanceChange *big.Int
	var postTokenBalance *big.Int
	for _, b := range getTxRet.Meta.PostTokenBalances {
		if b.AccountIndex == uint16(addressIdx) && b.Mint.Equals(tokenToCheck) {
			postTokenBalance = decimal.RequireFromString(b.UiTokenAmount.Amount).Mul(decimal.New(10, int32(b.UiTokenAmount.Decimals))).BigInt()
			break
		}
	}
	if postTokenBalance == nil {
		return fmt.Errorf("token balance not found in transaction")
	}
	for _, b := range getTxRet.Meta.PreTokenBalances {
		if b.AccountIndex == uint16(addressIdx) && b.Mint.Equals(tokenToCheck) {
			preBalance := decimal.RequireFromString(b.UiTokenAmount.Amount).Mul(decimal.New(10, int32(b.UiTokenAmount.Decimals))).BigInt()
			balanceChange = new(big.Int).Sub(postTokenBalance, preBalance)
			break
		}
	}

	blockRet, err := rpcClient.GetBlockWithOpts(ctx, getTxRet.Slot, &rpc.GetBlockOpts{
		MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
		Encoding:                       solana.EncodingBase64,
	})
	if err != nil {
		return fmt.Errorf("failed to get block: %w", err)
	}

	var relatedTxs []txContext
	var userTxIdx int = -1

	for i := 0; i < len(blockRet.Transactions); i++ {
		t := blockRet.Transactions[i]
		tx := t.MustGetTransaction()
		if t.Meta.Err != nil {
			continue
		}
		isUserTx := tx.Signatures[0].Equals(signature)
		if isUserTx {
			if userTxIdx != -1 {
				return fmt.Errorf("user transaction found multiple times in block")
			}
			userTxIdx = i
		}
		balanceChangeIdx := slices.IndexFunc(t.Meta.PostTokenBalances, func(b rpc.TokenBalance) bool {
			if !isUserTx {
				return b.Mint.Equals(tokenToCheck)
			}
			return b.AccountIndex == uint16(addressIdx) && b.Mint.Equals(tokenToCheck)
		})
		if balanceChangeIdx != -1 {
			amount := decimal.RequireFromString(t.Meta.PostTokenBalances[balanceChangeIdx].UiTokenAmount.Amount).
				Sub(decimal.RequireFromString(t.Meta.PreTokenBalances[balanceChangeIdx].UiTokenAmount.Amount)).
				Mul(decimal.New(10, int32(t.Meta.PostTokenBalances[balanceChangeIdx].UiTokenAmount.Decimals))).
				BigInt()
			relatedTxs = append(relatedTxs, txContext{
				Signature: base58.Encode(tx.Signatures[0][:]),
				Account:   tx.Message.AccountKeys[t.Meta.PostTokenBalances[balanceChangeIdx].AccountIndex].String(),
				Idx:       i,
				Amount:    amount,
			})
		}
	}

	if userTxIdx == -1 {
		return fmt.Errorf("user transaction not found in block: %d", getTxRet.Slot)
	}

	log.Printf("用户交易在区块中的索引: %d (共 %d 笔交易)", userTxIdx, len(blockRet.Transactions))
	log.Printf("用户 Token 余额变化: %v", balanceChange)
	log.Println(">>>>>>>>> Related transactions <<<<<<<<<")
	for _, relatedTx := range relatedTxs {
		var desc string
		if relatedTx.Idx == userTxIdx {
			desc = "(user)"
		}
		log.Printf("idx: %d%s, amount: %v, account: %s, signature: %s", relatedTx.Idx, desc, relatedTx.Amount, relatedTx.Account, relatedTx.Signature)
	}

	return nil
}
