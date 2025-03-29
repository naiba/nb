package solana

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"slices"

	"github.com/btcsuite/btcutil/base58"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type txContext struct {
	Signature string
	Account   string
	Idx       int
	Amount    *big.Int
}

func CheckSandwichAttack(ctx context.Context, rpcUrl string, signatureString string, userAddress string, tokenAddress string, maxCheckTxCount int) error {
	addressToCheck := solana.MustPublicKeyFromBase58(userAddress)
	tokenToCheck := solana.MustPublicKeyFromBase58(tokenAddress)
	signature := solana.MustSignatureFromBase58(signatureString)

	tokenAccountToCheck, _, err := solana.FindAssociatedTokenAddress(
		addressToCheck,
		tokenToCheck,
	)
	if err != nil {
		return err
	}

	maxSupportedTransactionVersion := uint64(0)

	rpcClient := rpc.New(rpcUrl)
	getTxRet, err := rpcClient.GetTransaction(
		ctx,
		signature,
		&rpc.GetTransactionOpts{
			MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
		},
	)
	if err != nil {
		return err
	}

	tx, err := getTxRet.Transaction.GetTransaction()
	if err != nil {
		return err
	}

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
		return err
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
		return fmt.Errorf("user transaction not found in block")
	}

	log.Printf("User token balance change: %v", balanceChange)
	log.Println(">>>>>>>>> Related transactions <<<<<<<<<")
	for _, relatedTx := range relatedTxs {
		var desc string
		if relatedTx.Idx == userTxIdx {
			desc = "(user)"
		}
		log.Printf("idx: %d%s, amount: %v, account: %s", relatedTx.Idx, desc, relatedTx.Amount, relatedTx.Account)
	}

	return nil
}
