package sandwich

import (
	"fmt"
	"strings"
)

// Format renders a Verdict as a human-readable report.
func Format(v Verdict) string {
	var b strings.Builder

	fmt.Fprintf(&b, "========== Sandwich Attack Analysis ==========\n")
	fmt.Fprintf(&b, "Verdict:    %s\n", v.Level)
	if v.Attacker != "" {
		fmt.Fprintf(&b, "Attacker:   %s\n", v.Attacker)
	}

	fmt.Fprintf(&b, "\n-- User Swap --\n")
	writeSwap(&b, v.UserSwap)

	if v.FrontRun != nil {
		fmt.Fprintf(&b, "\n-- Front-run --\n")
		writeSwap(&b, *v.FrontRun)
	}
	if v.BackRun != nil {
		fmt.Fprintf(&b, "\n-- Back-run --\n")
		writeSwap(&b, *v.BackRun)
	}

	if v.LossEstimate != nil {
		fmt.Fprintf(&b, "\nLoss estimate: %s (in user.OutMint atomic units, ~±30%% precision; CLMM/stable pools may differ more)\n",
			v.LossEstimate.String())
	}

	if len(v.RelatedPoolTxs) > 0 {
		fmt.Fprintf(&b, "\n-- Swaps on user's pools --\n")
		for _, s := range v.RelatedPoolTxs {
			writeRelatedSwap(&b, s, v.UserSwap)
		}
	}

	if len(v.Reasons) > 0 {
		fmt.Fprintf(&b, "\n-- Reasons --\n")
		for _, r := range v.Reasons {
			fmt.Fprintf(&b, "  - %s\n", r)
		}
	}
	fmt.Fprintf(&b, "==============================================\n")
	return b.String()
}

func writeSwap(b *strings.Builder, s Swap) {
	fmt.Fprintf(b, "  sig:     %s\n", s.Signature)
	fmt.Fprintf(b, "  slot#tx: %d#%d\n", s.Slot, s.TxIndex)
	fmt.Fprintf(b, "  trader:  %s (fee_payer=%s)\n", s.Owner, s.FeePayer)
	if s.IsMultiMint {
		fmt.Fprintf(b, "  direction: MULTI_HOP (3+ mints)\n")
		return
	}
	fmt.Fprintf(b, "  spent:   %s of %s\n", s.InAmount.String(), s.InMint)
	fmt.Fprintf(b, "  got:     %s of %s\n", s.OutAmount.String(), s.OutMint)
}

// writeRelatedSwap 统一以"发起人视角"展示一条与 user 池相关的 swap。
//
// 无论底层 Swap 是 trader 视角（owner=发起人钱包）还是 pool 视角（owner=pool PDA），
// 这里都归一化为"fp 花 X 换 Y"，pool 视角下会反转 In/Out。
// label:
//   [you]   - 这笔 tx 就是 user 自己的
//   [other] - 其他人的 tx（arb bot、另一个 swap 用户等）
func writeRelatedSwap(b *strings.Builder, s Swap, user Swap) {
	label := "[other]"
	if s.Signature == user.Signature {
		label = "[you]"
	}
	fmt.Fprintf(b, "  - %s slot#tx=%d#%d  fp=%s\n", label, s.Slot, s.TxIndex, s.FeePayer)
	fmt.Fprintf(b, "    sig=%s\n", s.Signature)
	if s.IsMultiMint {
		fmt.Fprintf(b, "    direction: MULTI_HOP\n")
		return
	}
	// Swap.In = owner 失去的, Swap.Out = owner 得到的
	// 以发起人（fp/trader）视角：
	//   IsTrader=true  → owner 就是发起人, spent=In, got=Out
	//   IsTrader=false → owner 是 pool, 反转: 发起人 spent=pool.Out(pool 得到), got=pool.In(pool 失去)
	spentMint, spentAmt := s.InMint, s.InAmount
	gotMint, gotAmt := s.OutMint, s.OutAmount
	if !s.IsTrader {
		spentMint, spentAmt = s.OutMint, s.OutAmount
		gotMint, gotAmt = s.InMint, s.InAmount
	}
	fmt.Fprintf(b, "    spent %s of %s → got %s of %s\n",
		spentAmt.String(), spentMint, gotAmt.String(), gotMint)
	if !s.IsTrader {
		fmt.Fprintf(b, "    via pool=%s\n", s.Owner)
	}
}
