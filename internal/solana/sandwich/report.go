package sandwich

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Format 渲染裸 Verdict (简化版,用于不需要资金流明细的场景)。
// 默认 CLI 使用 FormatDetailed;此函数保留供外部程序化消费 Verdict 时使用。
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

// FormatDetailed 是默认 CLI 输出,让用户看懂完整资金流和判定逻辑。
//
// 布局:
//
//	Verdict / Attacker / Loss 概要
//	━━ Related transactions on your pools (N, incl. yours) ━━
//	    按时序排好的所有碰到 user 池的 tx (user 自己作为 [YOU] 在对应位置)。
//	    每笔 tx 展开: trader 层 + 走过的每个 pool。地址和 sig 全量,不缩写。
//	━━ Detection ━━
//	    attacker-centric 聚合判定
//	━━ Reasons ━━
//	    结论依据
func FormatDetailed(r DetailedReport) string {
	var b strings.Builder
	v := r.Verdict

	fmt.Fprintf(&b, "========== Sandwich Attack Analysis ==========\n")
	fmt.Fprintf(&b, "Verdict: %s\n", v.Level)
	if v.Attacker != "" {
		fmt.Fprintf(&b, "Attacker: %s\n", v.Attacker)
	}
	if v.LossEstimate != nil {
		fmt.Fprintf(&b, "Loss estimate: %s (user.OutMint atomic units; AMM v2 assumption, ~±30%%)\n",
			v.LossEstimate.String())
	}

	// 相关 tx: user 自己 + 所有碰到 user 池的其他 tx, 按时序一起排。
	// user 自己标 [YOU], 其他按 FRONT/BACK/other 标。每笔 tx 完整展开。
	writeRelatedTimeline(&b, r, v)

	// 判定过程
	if !v.UserSwap.IsMultiMint && v.UserSwap.Signature != "" {
		writeDetectionSummary(&b, r)
	}

	// 理由
	if len(v.Reasons) > 0 {
		fmt.Fprintf(&b, "\n━━ Reasons ━━\n")
		for _, reason := range v.Reasons {
			fmt.Fprintf(&b, "  • %s\n", reason)
		}
	}

	fmt.Fprintf(&b, "==============================================\n")
	return b.String()
}

// writeRelatedTimeline 把 user 自己和所有碰到 user 池的其他 tx 一起按 (slot, txIndex)
// 时序铺出来。每笔 tx 展开 trader 层 (如有) + 走过的所有 pool (每个 pool 一段)。
// user 池标 "[yours]"。所有地址和 sig 完整,不缩写。
func writeRelatedTimeline(b *strings.Builder, r DetailedReport, v Verdict) {
	if len(r.RelatedSigs) == 0 {
		fmt.Fprintf(b, "\n━━ Related transactions on your pools ━━\n")
		fmt.Fprintf(b, "  (none within the scan window — only your own tx touched your pool set)\n")
		return
	}

	userPoolSet := toSet(r.UserPools)

	// 截断时必须保证 user 自己的 tx 保留: user 可能排在列表中间 (例如 40 条 related
	// 里 user 在 20+), 简单 slice 会把 [YOU] 藏掉。做法: 如果 user sig 不在前 relatedLimit
	// 里, 替换掉其中一条 (丢最后一条, 保 [YOU])。
	shown := r.RelatedSigs
	truncated := 0
	if len(shown) > relatedLimit {
		truncated = len(shown) - relatedLimit
		shown = append([]string(nil), r.RelatedSigs[:relatedLimit]...)
		userSig := v.UserSwap.Signature
		if userSig != "" {
			inShown := false
			for _, s := range shown {
				if s == userSig {
					inShown = true
					break
				}
			}
			if !inShown {
				// 找 user sig 在原列表中的位置, 若存在则替换 shown 最后一条
				userExists := false
				for _, s := range r.RelatedSigs {
					if s == userSig {
						userExists = true
						break
					}
				}
				if userExists {
					shown[len(shown)-1] = userSig
				}
			}
		}
	}
	fmt.Fprintf(b, "\n━━ Related transactions on your pools (%d, incl. yours) ━━\n", len(r.RelatedSigs))
	for _, sig := range shown {
		role := classifyRole(sig, r, v)
		writeTxBlock(b, sig, r.TxFlows[sig], role, userPoolSet)
	}
	if truncated > 0 {
		fmt.Fprintf(b, "\n  … (%d more hidden; lower --slots to narrow the window)\n", truncated)
	}
}

// writeTxBlock 完整展示一笔 tx: 头部 (role + slot#tx + sig + fp) + trader 层 + 每个 pool 层。
// 为了让读者确认 front/user/back 的 slot 顺序, 所有地址和 sig 不缩写。
func writeTxBlock(b *strings.Builder, sig string, swaps []Swap, role string, userPools map[string]bool) {
	// header 用一笔代表 swap 取 slot/txIndex/feePayer。trader 优先,否则第一条。
	if len(swaps) == 0 {
		fmt.Fprintf(b, "\n  [%s] sig=%s\n", role, sig)
		fmt.Fprintf(b, "      (no swap flow extracted)\n")
		return
	}
	rep := swaps[0]
	for _, s := range swaps {
		if s.IsTrader {
			rep = s
			break
		}
	}
	fmt.Fprintf(b, "\n  [%s] slot=%d  tx=%d\n", role, rep.Slot, rep.TxIndex)
	fmt.Fprintf(b, "      sig=%s\n", sig)
	fmt.Fprintf(b, "      fp= %s\n", rep.FeePayer)

	// 拆 trader / pool
	var traderSwaps, poolSwaps []Swap
	for _, s := range swaps {
		if s.IsTrader {
			traderSwaps = append(traderSwaps, s)
		} else {
			poolSwaps = append(poolSwaps, s)
		}
	}
	sort.Slice(poolSwaps, func(i, j int) bool { return poolSwaps[i].Owner < poolSwaps[j].Owner })

	for _, s := range traderSwaps {
		writeOwnerBlock(b, s, "trader", s.Owner, false)
	}
	for _, s := range poolSwaps {
		isUserPool := userPools[s.Owner]
		label := "pool"
		if isUserPool {
			label = "pool [yours]"
		}
		writeOwnerBlock(b, s, label, s.Owner, isUserPool)
	}
}

// writeOwnerBlock 打印单个 owner (trader 或某个 pool) 在这笔 tx 内的 in/out。
func writeOwnerBlock(b *strings.Builder, s Swap, label, owner string, _ bool) {
	fmt.Fprintf(b, "      %s  %s\n", label, owner)
	if s.IsMultiMint {
		fmt.Fprintf(b, "          [MULTI_HOP] 3+ mints or same-sign pair, direction not representable\n")
		return
	}
	if s.InAmount != nil && s.InAmount.Sign() > 0 {
		fmt.Fprintf(b, "          -%s %s\n", s.InAmount.String(), s.InMint)
	}
	if s.OutAmount != nil && s.OutAmount.Sign() > 0 {
		fmt.Fprintf(b, "          +%s %s\n", s.OutAmount.String(), s.OutMint)
	}
}

// attackerDetailLimit 是每类 attacker 最多展开的条数 (sandwich / same-pool-but-not-cancel)。
// "不同池" 的 attacker 只统计总数, 不展开 —— 他们对 user 没影响。
const attackerDetailLimit = 5

// relatedLimit 是 Related transactions 段最多展示的同池 tx 数。
// 热门 mint 上同池可能有几十到几百笔同向/反向交易,全列会刷屏,超出的用计数收尾。
const relatedLimit = 20

// writeDetectionSummary 按 attacker 维度展示检测结果。
//
// 三类 attacker:
//
//	sandwichers    IsSandwich=true        → 展开详情, 最多 attackerDetailLimit 条
//	samePoolOnly   SharedUserPool != "" 但金额不闭合 → 展开详情, 最多 attackerDetailLimit 条
//	otherPoolOnly  SharedUserPool == ""   → 只统计数量 (他们在别的池,与 user 无关)
func writeDetectionSummary(b *strings.Builder, r DetailedReport) {
	fmt.Fprintf(b, "\n━━ Detection ━━\n")
	fmt.Fprintf(b, "  user pools:\n")
	for _, p := range r.UserPools {
		fmt.Fprintf(b, "    %s\n", p)
	}
	fmt.Fprintf(b, "  candidate frontrun swaps (before user, same direction):  %d\n", len(r.CandidateFronts))
	fmt.Fprintf(b, "  candidate backrun swaps  (after user, reverse direction): %d\n", len(r.CandidateBacks))
	fmt.Fprintf(b, "  attackers (same trader with both front+back): %d\n", len(r.Attackers))

	if len(r.Attackers) == 0 {
		fmt.Fprintln(b, "  → no trader had both a frontrun and backrun around your swap")
		return
	}

	var sandwichers, samePoolOnly, otherPoolOnly []AttackerEvidence
	for _, a := range r.Attackers {
		switch {
		case a.IsSandwich:
			sandwichers = append(sandwichers, a)
		case a.SharedUserPool != "":
			samePoolOnly = append(samePoolOnly, a)
		default:
			otherPoolOnly = append(otherPoolOnly, a)
		}
	}

	if len(sandwichers) > 0 {
		// 进一步细分 classic (同 slot) vs atypical (跨 slot)。
		var classic, atypical []AttackerEvidence
		for _, a := range sandwichers {
			if a.IsClassic {
				classic = append(classic, a)
			} else {
				atypical = append(atypical, a)
			}
		}
		if len(classic) > 0 {
			writeAttackerGroup(b, classic, "⚠ SANDWICH (classic, same-slot bundle)")
		}
		if len(atypical) > 0 {
			writeAttackerGroup(b, atypical, "⚠ SANDWICH (atypical, cross-slot race)")
		}
	}
	if len(samePoolOnly) > 0 {
		writeAttackerGroup(b, samePoolOnly,
			"same pool as yours, amounts do NOT cancel (probably accidental, not a sandwich)")
	}
	if len(otherPoolOnly) > 0 {
		fmt.Fprintf(b,
			"\n  %d trader(s) had frontrun+backrun but on pools OUTSIDE yours — not your sandwich, skipped.\n",
			len(otherPoolOnly))
	}
}

func writeAttackerGroup(b *strings.Builder, group []AttackerEvidence, tag string) {
	show := group
	truncated := 0
	if len(show) > attackerDetailLimit {
		truncated = len(show) - attackerDetailLimit
		show = show[:attackerDetailLimit]
	}
	for _, a := range show {
		writeAttackerEvidence(b, a, tag)
	}
	if truncated > 0 {
		fmt.Fprintf(b, "  … (%d more omitted)\n", truncated)
	}
}

// writeAttackerEvidence 每个 attacker 一块 profile,汇总 front/back 聚合量 + 闭合比 + 利润。
// 地址 / mint / sig 全量, 不缩写。
func writeAttackerEvidence(b *strings.Builder, a AttackerEvidence, tag string) {
	fmt.Fprintf(b, "\n  [%s]\n", tag)
	fmt.Fprintf(b, "    attacker (via %s): %s\n", a.AttackerKeyVia, a.AttackerKey)

	// frontrun 聚合
	if len(a.Frontruns) > 0 {
		f0 := a.Frontruns[0]
		fmt.Fprintf(b, "    frontrun (%d tx):\n", len(a.Frontruns))
		if a.FrontSpentInMint != nil && a.FrontSpentInMint.Sign() > 0 {
			fmt.Fprintf(b, "        -%s %s\n", a.FrontSpentInMint.String(), f0.InMint)
		}
		if a.FrontGotOutMint != nil && a.FrontGotOutMint.Sign() > 0 {
			fmt.Fprintf(b, "        +%s %s\n", a.FrontGotOutMint.String(), f0.OutMint)
		}
		for _, f := range a.Frontruns {
			fmt.Fprintf(b, "        · slot=%d tx=%d sig=%s\n", f.Slot, f.TxIndex, f.Signature)
		}
	}
	// backrun 聚合
	if len(a.Backruns) > 0 {
		b0 := a.Backruns[0]
		fmt.Fprintf(b, "    backrun (%d tx):\n", len(a.Backruns))
		if a.BackSpentOutMint != nil && a.BackSpentOutMint.Sign() > 0 {
			fmt.Fprintf(b, "        -%s %s\n", a.BackSpentOutMint.String(), b0.InMint)
		}
		if a.BackGotInMint != nil && a.BackGotInMint.Sign() > 0 {
			fmt.Fprintf(b, "        +%s %s\n", a.BackGotInMint.String(), b0.OutMint)
		}
		for _, bk := range a.Backruns {
			fmt.Fprintf(b, "        · slot=%d tx=%d sig=%s\n", bk.Slot, bk.TxIndex, bk.Signature)
		}
	}
	if a.SharedUserPool != "" {
		fmt.Fprintf(b, "    shared pool with you: %s\n", a.SharedUserPool)
	}

	// 金额闭合比 (aggregate 口径)
	if a.FrontGotOutMint != nil && a.FrontGotOutMint.Sign() > 0 &&
		a.BackSpentOutMint != nil && a.BackSpentOutMint.Sign() > 0 {
		ratio := new(big.Int).Mul(a.BackSpentOutMint, big.NewInt(100))
		ratio.Quo(ratio, a.FrontGotOutMint)
		fmt.Fprintf(b, "    closure: Σback.sold / Σfront.bought = %s%% (require 30%%~300%%) %s\n",
			ratio.String(), mark(a.AmountsCancel))
	}

	// 利润
	if a.IsSandwich && a.NetProfitInMint != nil && a.NetProfitInMint.Sign() > 0 && len(a.Frontruns) > 0 {
		fmt.Fprintf(b, "    net profit ≈ +%s %s (Σback.got - Σfront.spent)\n",
			a.NetProfitInMint.String(), a.Frontruns[0].InMint)
	}
}

func mark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

// classifyRole 为资金流段的 tx 标注角色。此处只可能出现 user + RelatedPoolTxs 中的 tx
// (见 buildTxFlows)。
//
//	YOU    user 自己 (UserSwap.Signature)
//	FRONT  Sandwiched 判定命中的 front-run 代表 tx
//	BACK   Sandwiched 判定命中的 back-run 代表 tx
//	self   fp 和 user 相同的其他 tx —— user 本人几秒内的连续单,
//	       标出来让读者不会误以为是陌生 attacker (Jupiter 用户常连续刷几笔)
//	other  真 · 其他人的同池交易 (arb bot / 另一个 swap 用户)
func classifyRole(sig string, r DetailedReport, v Verdict) string {
	if v.UserSwap.Signature != "" && sig == v.UserSwap.Signature {
		return "YOU"
	}
	if v.FrontRun != nil && sig == v.FrontRun.Signature {
		return "FRONT"
	}
	if v.BackRun != nil && sig == v.BackRun.Signature {
		return "BACK"
	}
	if v.UserSwap.FeePayer != "" {
		if swaps, ok := r.TxFlows[sig]; ok && len(swaps) > 0 && swaps[0].FeePayer == v.UserSwap.FeePayer {
			return "self"
		}
	}
	return "other"
}

func toSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, s := range list {
		m[s] = true
	}
	return m
}

// shortAddr 截断 base58 地址,保留前 8 字符 + "..."。空字符串直接返回。
// 仅用于简化版 Format (向后兼容路径);FormatDetailed 已不再调用。
func shortAddr(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "…"
}

func shortSig(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:12] + "…" + s[len(s)-4:]
}

func shortMint(s string) string {
	// wSOL 特殊处理让人一眼识别
	if s == WrappedSolMint {
		return "wSOL"
	}
	return shortAddr(s)
}

func shortList(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = shortAddr(x)
	}
	return out
}

func writeSwap(b *strings.Builder, s Swap) {
	fmt.Fprintf(b, "  sig:     %s\n", s.Signature)
	fmt.Fprintf(b, "  slot#tx: %d#%d\n", s.Slot, s.TxIndex)
	fmt.Fprintf(b, "  trader:  %s (fee_payer=%s)\n", s.Owner, s.FeePayer)
	if s.IsMultiMint {
		fmt.Fprintf(b, "  direction: MULTI_HOP (3+ mints)\n")
		return
	}
	if s.InAmount != nil {
		fmt.Fprintf(b, "  spent:   %s of %s\n", s.InAmount.String(), s.InMint)
	}
	if s.OutAmount != nil {
		fmt.Fprintf(b, "  got:     %s of %s\n", s.OutAmount.String(), s.OutMint)
	}
}

// writeRelatedSwap 统一以 "发起人视角" 展示一条与 user 池相关的 swap(简化版 Format 使用)。
//
// label:
//   [you]   - 这笔 tx 就是 user 自己的
//   [other] - 其他人的 tx
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
