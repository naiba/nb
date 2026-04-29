// internal/solana/sandwich/analyze.go
package sandwich

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
)

// Analyze is the CLI entry: fetch -> extract swaps -> detect -> print report.
//
// slotsRange specifies how many slots on each side of the user slot to scan;
// pass 0 to only scan the user slot.
func Analyze(ctx context.Context, rpcURL, userSig, userAddr, userMint string, slotsRange int) error {
	fc, err := FetchContext(ctx, rpcURL, userSig, userAddr, userMint, slotsRange)
	if err != nil {
		return err
	}

	log.Printf("========== csa ==========")
	log.Printf("user slot: %d", fc.UserSlot)
	log.Printf("user ATA:  %s", fc.UserTokenAcc)
	if len(fc.SkippedSlots) > 0 {
		log.Printf("skipped slots (rpc unavailable): %v", fc.SkippedSlots)
	}

	// Aggregate all relevant swaps from fetched blocks.
	var allSwaps []Swap
	for slot, blk := range fc.Blocks {
		for i, tx := range blk.Transactions {
			if tx.Meta == nil || tx.Meta.Err != nil {
				continue
			}
			ptx, err := tx.GetTransaction()
			if err != nil || ptx == nil {
				continue
			}
			if !touchesMint(tx.Meta, userMint) {
				continue
			}
			feePayer := ""
			if len(ptx.Message.AccountKeys) > 0 {
				feePayer = ptx.Message.AccountKeys[0].String()
			}
			if len(ptx.Signatures) == 0 {
				continue
			}
			sig := base58.Encode(ptx.Signatures[0][:])
			accountKeys := accountKeysAsStrings(ptx.Message.AccountKeys)
			allSwaps = append(allSwaps,
				ExtractSwapsFull(sig, slot, i, feePayer,
					accountKeys, tx.Meta.PreBalances, tx.Meta.PostBalances,
					tx.Meta.Fee, 0,
					tx.Meta.PreTokenBalances, tx.Meta.PostTokenBalances)...)
		}
	}

	// The user tx may not appear in Blocks if its slot's getBlock was skipped.
	// Fill it in from fc.UserTx so Detect can still locate the user swap.
	if !containsUserTx(allSwaps, userSig) {
		// Guard against nil Meta (panic risk) and failed user tx (align with
		// spec §7: user tx failed on-chain → NotSandwiched, nothing to analyze).
		if fc.UserTx.Meta == nil {
			return fmt.Errorf("user tx has no metadata")
		}
		if fc.UserTx.Meta.Err != nil {
			fmt.Println(Format(Verdict{
				Level:   NotSandwiched,
				Reasons: []string{"user tx failed on-chain; no slippage to analyze"},
			}))
			return nil
		}
		ptx, err := fc.UserTx.Transaction.GetTransaction()
		if err != nil {
			return fmt.Errorf("decode user tx: %w", err)
		}
		feePayer := ""
		if len(ptx.Message.AccountKeys) > 0 {
			feePayer = ptx.Message.AccountKeys[0].String()
		}
		// TxIndex is unknown here; use -1. Detect sorts primarily by Slot so this
		// is acceptable when no in-block neighbors exist for the user slot.
		accountKeys := accountKeysAsStrings(ptx.Message.AccountKeys)
		allSwaps = append(allSwaps,
			ExtractSwapsFull(userSig, fc.UserSlot, -1, feePayer,
				accountKeys, fc.UserTx.Meta.PreBalances, fc.UserTx.Meta.PostBalances,
				fc.UserTx.Meta.Fee, 0,
				fc.UserTx.Meta.PreTokenBalances, fc.UserTx.Meta.PostTokenBalances)...)
	}

	verdict := Detect(userSig, userAddr, userMint, allSwaps)

	// Attach loss estimate only for Sandwiched verdicts with a front-run reference.
	if verdict.Level == Sandwiched && verdict.FrontRun != nil {
		verdict.LossEstimate = tryEstimateLoss(fc, verdict)
	}

	fmt.Println(Format(verdict))
	return nil
}

// touchesMint returns true when the tx's token balances reference the given mint.
// Used as a cheap pre-filter before doing full swap extraction.
func touchesMint(meta *rpc.TransactionMeta, mint string) bool {
	target := solana.MustPublicKeyFromBase58(mint)
	for _, b := range meta.PreTokenBalances {
		if b.Mint.Equals(target) {
			return true
		}
	}
	for _, b := range meta.PostTokenBalances {
		if b.Mint.Equals(target) {
			return true
		}
	}
	return false
}

func containsUserTx(swaps []Swap, sig string) bool {
	for _, s := range swaps {
		if s.Signature == sig {
			return true
		}
	}
	return false
}

// tryEstimateLoss derives preX/preY from the front-run tx's pool pre-balances
// and feeds them into estimateLoss. Returns nil on any failure (the Sandwiched
// verdict itself is unaffected; only the loss number is missing).
func tryEstimateLoss(fc *Context, v Verdict) *big.Int {
	if v.FrontRun == nil {
		return nil
	}
	blk, ok := fc.Blocks[v.FrontRun.Slot]
	if !ok {
		return nil
	}
	if v.FrontRun.TxIndex < 0 || v.FrontRun.TxIndex >= len(blk.Transactions) {
		return nil
	}
	tx := blk.Transactions[v.FrontRun.TxIndex]
	if tx.Meta == nil {
		return nil
	}

	// Index pre balances by (owner, mint) for O(1) lookup below.
	type key struct{ owner, mint string }
	amts := map[key]*big.Int{}
	for _, b := range tx.Meta.PreTokenBalances {
		if b.Owner == nil || b.UiTokenAmount == nil {
			continue
		}
		amts[key{b.Owner.String(), b.Mint.String()}] = amountToBigInt(b.UiTokenAmount)
	}

	// Find a counterparty pool in the front-run tx that holds both user.InMint
	// and user.OutMint; that's the pool used to feed estimateLoss.
	frontSwaps := extractSwapsFromTx(tx, v.FrontRun.Slot, v.FrontRun.TxIndex)
	for owner := range counterpartyPools(*v.FrontRun, frontSwaps) {
		x := amts[key{owner, v.UserSwap.InMint}]
		y := amts[key{owner, v.UserSwap.OutMint}]
		if x != nil && y != nil {
			return estimateLoss(x, y, v.UserSwap.InAmount, v.UserSwap.OutAmount)
		}
	}
	return nil
}

// extractSwapsFromTx is a small helper that mirrors the extraction logic inside
// Analyze but for a single rpc.TransactionWithMeta at a known slot/index.
func extractSwapsFromTx(tx rpc.TransactionWithMeta, slot uint64, idx int) []Swap {
	if tx.Meta == nil {
		return nil
	}
	ptx, err := tx.GetTransaction()
	if err != nil || ptx == nil {
		return nil
	}
	feePayer := ""
	if len(ptx.Message.AccountKeys) > 0 {
		feePayer = ptx.Message.AccountKeys[0].String()
	}
	if len(ptx.Signatures) == 0 {
		return nil
	}
	sig := base58.Encode(ptx.Signatures[0][:])
	accountKeys := accountKeysAsStrings(ptx.Message.AccountKeys)
	return ExtractSwapsFull(sig, slot, idx, feePayer,
		accountKeys, tx.Meta.PreBalances, tx.Meta.PostBalances,
		tx.Meta.Fee, 0,
		tx.Meta.PreTokenBalances, tx.Meta.PostTokenBalances)
}

// accountKeysAsStrings 把 solana.PublicKeySlice 转为 []string，
// 供 ExtractSwapsFull 使用。
func accountKeysAsStrings(keys solana.PublicKeySlice) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.String()
	}
	return out
}
