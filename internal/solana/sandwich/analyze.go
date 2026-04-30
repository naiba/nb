// internal/solana/sandwich/analyze.go
package sandwich

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"

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
			accountKeys := resolvedAccountKeysAsStrings(ptx, tx.Meta)
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
		accountKeys := resolvedAccountKeysAsStrings(ptx, fc.UserTx.Meta)
		allSwaps = append(allSwaps,
			ExtractSwapsFull(userSig, fc.UserSlot, -1, feePayer,
				accountKeys, fc.UserTx.Meta.PreBalances, fc.UserTx.Meta.PostBalances,
				fc.UserTx.Meta.Fee, 0,
				fc.UserTx.Meta.PreTokenBalances, fc.UserTx.Meta.PostTokenBalances)...)
	}

	report := DetectDetailed(userSig, userAddr, userMint, allSwaps)

	// Attach loss estimate only for Sandwiched verdicts with a front-run reference.
	if report.Verdict.Level == Sandwiched && report.Verdict.FrontRun != nil {
		report.Verdict.LossEstimate = tryEstimateLoss(fc, report.Verdict)
	}

	// 组装资金流展示所需的 TxFlows 和 RelatedSigs。TxFlows 按 signature 分组,只保留
	// 我们打算展示的 tx (user + 所有 front/back 候选 + RelatedPoolTxs),避免把几百条
	// 同 mint 的无关 tx 全部铺出来刷屏。
	report.TxFlows, report.RelatedSigs = buildTxFlows(userSig, allSwaps, report)

	fmt.Println(FormatDetailed(report))
	return nil
}

// buildTxFlows 基于 allSwaps (按 signature 分组) 和 detect 的中间结果,挑出要展示的 tx,
// 返回其完整 Swap 列表和按展示顺序排好的 signature 列表。
//
// 展示集合 = {user tx} ∪ {RelatedPoolTxs 中 tx}。
// 候选 front/back 但不在 user 池上的 tx (attacker 在独立池做套利) 不展示资金流 —— 他们
// 的资金不经过 user 池, 列出来会让用户混淆 "这些钱和我有关吗"。这些 tx 在 Detection 段
// 仍以 signature + pool 缩写形式出现, 足够解释为什么没判夹击。
//
// 顺序按 tx 的最小 (Slot, TxIndex) 升序; user tx 自然按时序排入。
func buildTxFlows(userSig string, allSwaps []Swap, r DetailedReport) (map[string][]Swap, []string) {
	include := map[string]bool{userSig: true}
	for _, s := range r.Verdict.RelatedPoolTxs {
		include[s.Signature] = true
	}

	flows := make(map[string][]Swap, len(include))
	type sigKey struct {
		slot    uint64
		txIndex int
	}
	firstPos := map[string]sigKey{}

	for _, s := range allSwaps {
		if !include[s.Signature] {
			continue
		}
		flows[s.Signature] = append(flows[s.Signature], s)
		k := sigKey{s.Slot, s.TxIndex}
		if cur, ok := firstPos[s.Signature]; !ok || k.slot < cur.slot ||
			(k.slot == cur.slot && k.txIndex < cur.txIndex) {
			firstPos[s.Signature] = k
		}
	}

	order := make([]string, 0, len(flows))
	for sig := range flows {
		order = append(order, sig)
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := firstPos[order[i]], firstPos[order[j]]
		if a.slot != b.slot {
			return a.slot < b.slot
		}
		return a.txIndex < b.txIndex
	})
	return flows, order
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
	// 修复: 同一 PDA owner 可能持有同一 mint 的多个 vault(罕见但存在,如某些 CLMM
	// 或 router 临时 ATA),旧实现后写覆盖会只取最后一个 vault 的余额。改为累加以
	// 得到该 owner 在该 mint 下的总持仓。
	type key struct{ owner, mint string }
	amts := map[key]*big.Int{}
	for _, b := range tx.Meta.PreTokenBalances {
		if b.Owner == nil || b.UiTokenAmount == nil {
			continue
		}
		k := key{b.Owner.String(), b.Mint.String()}
		amt := amountToBigInt(b.UiTokenAmount)
		if existing, ok := amts[k]; ok {
			amts[k] = new(big.Int).Add(existing, amt)
		} else {
			amts[k] = amt
		}
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
	accountKeys := resolvedAccountKeysAsStrings(ptx, tx.Meta)
	return ExtractSwapsFull(sig, slot, idx, feePayer,
		accountKeys, tx.Meta.PreBalances, tx.Meta.PostBalances,
		tx.Meta.Fee, 0,
		tx.Meta.PreTokenBalances, tx.Meta.PostTokenBalances)
}
