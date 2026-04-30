package sandwich

import (
	"fmt"
	"math/big"
	"sort"
)

// 金额抵消容差: back.InAmount 应落在 [front.OutAmount * minRatio, front.OutAmount * maxRatio] 内。
// 真实 sandwich bot 倾向在 back-run 把前面抢到的筹码基本全部甩出,部分留存或 swap 手续费会让
// 闭合比例偏离 1:1,但偏离不应超过一个量级。30%–300% 能覆盖主流 MEV bot 的行为,更窄会漏检。
//
// 分子/分母放大 100 倍,避免引入浮点数。
const (
	amountCancelMinNum = 30  // 分子: 30 / 100 = 0.3
	amountCancelMaxNum = 300 // 分子: 300 / 100 = 3.0
	amountCancelDenom  = 100
)

// Detect 在 []Swap 中识别夹子模式,输出二分 Verdict。这是 DetectDetailed 的简化入口,
// 保留稳定签名供外部调用;不需要诊断信息时使用本函数,否则用 DetectDetailed。
func Detect(userSig, userAddr, userMint string, swaps []Swap) Verdict {
	return DetectDetailed(userSig, userAddr, userMint, swaps).Verdict
}

// DetectDetailed 返回完整诊断报告, 包含:
//   - Verdict 本身
//   - 用户池集合
//   - 所有 front/back 候选 (A+C 通过的 trader swap)
//   - 每对候选的 B/D/E 评估结果 (ConditionCheck)
//
// 注意 TxFlows / RelatedSigs 字段由上游 (Analyze) 填充,因为完整资金流需要原始 []Swap
// 按 signature 分组,而 Detect 层只持有已扁平化的 []Swap —— 分组可以在这里做,但
// "挑选要展示的相关 tx" 依赖 fetch 范围和 user tx 信息,不属于 Detect 的职责。
func DetectDetailed(userSig, userAddr, userMint string, swaps []Swap) DetailedReport {
	sort.Slice(swaps, func(i, j int) bool {
		if swaps[i].Slot != swaps[j].Slot {
			return swaps[i].Slot < swaps[j].Slot
		}
		return swaps[i].TxIndex < swaps[j].TxIndex
	})

	report := DetailedReport{}

	userSwap, userPools, ok := findUserSwap(userSig, userAddr, swaps)
	if !ok {
		report.Verdict = Verdict{
			Level:   NotSandwiched,
			Reasons: []string{"user swap not found in scanned blocks"},
		}
		return report
	}

	report.UserPools = sortedKeys(userPools)

	// RelatedPoolTxs 对 MultiMint 也要填充: Jupiter 多跳路由虽然无法判定严格夹击,
	// 但其他 tx 碰到 user 任一对手池依然值得暴露给人工核验。
	related := findRelatedPoolTxs(userSwap, userPools, swaps)

	if userSwap.IsMultiMint {
		report.Verdict = Verdict{
			Level:    NotSandwiched,
			UserSwap: userSwap,
			Reasons: []string{
				"user tx is a multi-hop route (Jupiter etc.), strict sandwich detection not supported",
				"related pool activity listed below for manual review",
			},
			RelatedPoolTxs: related,
		}
		return report
	}

	// 生成 A+C 候选, 按 attacker 聚合, 评估同池 + 金额闭合 (aggregate 口径)。
	fronts, backs := collectCandidates(userSwap, swaps)
	report.CandidateFronts = fronts
	report.CandidateBacks = backs
	report.Attackers = evaluateAttackers(userSwap, fronts, backs, userPools, swaps)

	// 第一个 IsSandwich=true 的 attacker 即为 Sandwiched 结论。
	// Attackers 已按 IsSandwich 优先排序,所以第一个就是最早命中的。
	for _, a := range report.Attackers {
		if a.IsSandwich {
			front := a.Frontruns[0]
			back := a.Backruns[len(a.Backruns)-1]
			report.Verdict = Verdict{
				Level:          Sandwiched,
				UserSwap:       userSwap,
				FrontRun:       &front,
				BackRun:        &back,
				Attacker:       a.AttackerKey,
				LossEstimate:   nil, // tryEstimateLoss 在 Analyze 层后补
				Reasons:        attackerReasons(a),
				RelatedPoolTxs: related,
			}
			return report
		}
	}

	report.Verdict = Verdict{
		Level:          NotSandwiched,
		UserSwap:       userSwap,
		Reasons:        []string{"no front+back pair by same trader sharing user's pool with matching amounts"},
		RelatedPoolTxs: related,
	}
	return report
}

// sortedKeys 返回 map 的 keys 升序列表,用于展示稳定化。
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// findUserSwap 定位 user 的 trader swap 并收集其对手 pool 集合。
//
// 两遍扫描: 第一遍收集 user 对手池(必须完整遍历,user tx 中 trader/pool swap 顺序不定);
// 第二遍定位 trader swap 后立刻 break。pool 集合要完整,所以不能合成一遍提前 break。
func findUserSwap(userSig, userAddr string, swaps []Swap) (Swap, map[string]bool, bool) {
	pools := make(map[string]bool)
	for _, s := range swaps {
		if s.Signature == userSig && !s.IsTrader {
			pools[s.Owner] = true
		}
	}
	var userSwap Swap
	found := false
	for _, s := range swaps {
		if s.Signature == userSig && s.IsTrader && s.Owner == userAddr {
			userSwap = s
			found = true
			break
		}
	}
	return userSwap, pools, found
}

// counterpartyPools 返回某个 trader swap 所在 tx 中 IsTrader=false 的 Owner 集合。
func counterpartyPools(s Swap, allSwaps []Swap) map[string]bool {
	out := make(map[string]bool)
	for _, x := range allSwaps {
		if x.Signature == s.Signature && !x.IsTrader {
			out[x.Owner] = true
		}
	}
	return out
}

// findRelatedPoolTxs 按 signature 维度扫描，列出所有与 user 池相关的交易。
//
// 判定标准：该 tx 下某个 pool 的 Owner ∈ user 对手池集合。
// 即便该 tx 的 trader 层没产出 swap（比如 arb bot 净流动为单边），
// pool 层的 swap 依然会暴露"碰了 user 池"的事实，故纳入展示。
//
// 展示代表选择：trader swap 优先（方向清晰），没有则选一个 user 池的 pool swap（方向是 pool 视角）。
// 每个 signature 最多列出一条。
//
// 返回列表开头是 user 自己 tx 中的 user 池 pool swaps（让用户一眼看到走了哪些池），
// 接着是其他 tx 的代表。report 层根据 Signature == user.Signature 标注 "user's pool"。
func findRelatedPoolTxs(user Swap, userPools map[string]bool, swaps []Swap) []Swap {
	var related []Swap

	// 前置：user 自己 tx 中的对手池 pool swaps，按 Owner 稳定排序方便阅读
	for _, s := range swaps {
		if s.Signature == user.Signature && !s.IsTrader && userPools[s.Owner] {
			related = append(related, s)
		}
	}

	bySig := map[string][]Swap{}
	sigOrder := []string{}
	for _, s := range swaps {
		if s.Signature == user.Signature {
			continue
		}
		if _, ok := bySig[s.Signature]; !ok {
			sigOrder = append(sigOrder, s.Signature)
		}
		bySig[s.Signature] = append(bySig[s.Signature], s)
	}

	for _, sig := range sigOrder {
		group := bySig[sig]
		touched := false
		for _, s := range group {
			if !s.IsTrader && userPools[s.Owner] {
				touched = true
				break
			}
		}
		if !touched {
			continue
		}
		var rep *Swap
		for i := range group {
			if group[i].IsTrader && !group[i].IsMultiMint {
				rep = &group[i]
				break
			}
		}
		if rep == nil {
			for i := range group {
				if !group[i].IsTrader && userPools[group[i].Owner] {
					rep = &group[i]
					break
				}
			}
		}
		if rep != nil {
			related = append(related, *rep)
		}
	}
	return related
}

func isAfter(s, ref Swap) bool {
	if s.Slot != ref.Slot {
		return s.Slot > ref.Slot
	}
	return s.TxIndex > ref.TxIndex
}

func isBefore(s, ref Swap) bool {
	if s.Slot != ref.Slot {
		return s.Slot < ref.Slot
	}
	return s.TxIndex < ref.TxIndex
}

func attackerID(s Swap) string {
	if s.FeePayer != "" {
		return s.FeePayer
	}
	return s.Owner
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// collectCandidates 从 swaps 里挑出满足 A+C 的 trader swap:
//
//	A (时序): front 在 user 之前, back 在 user 之后
//	C (方向): front 同方向 (in/out mint 与 user 相同), back 反方向
//
// 跳过 MultiMint 和 user 自己的 signature。返回的 fronts/backs 顺序与 swaps 一致
// (即已按 Slot,TxIndex 升序)。
func collectCandidates(user Swap, swaps []Swap) (fronts, backs []Swap) {
	for _, s := range swaps {
		if !s.IsTrader || s.IsMultiMint || s.Signature == user.Signature {
			continue
		}
		if isBefore(s, user) {
			if s.InMint == user.InMint && s.OutMint == user.OutMint {
				fronts = append(fronts, s)
			}
		} else if isAfter(s, user) {
			if s.InMint == user.OutMint && s.OutMint == user.InMint {
				backs = append(backs, s)
			}
		}
	}
	return
}

// evaluateAttackers 参考 EVM 版 analyze_evm_swap_tx.rs 的 sender 聚合思路:
// 按 attacker key (Owner/FeePayer 经 union-find 合并) 把 fronts/backs 分组,每个
// attacker 独立判定同池 + 金额闭合。相比 fronts × backs 笛卡尔积 + 逐对 B/D/E
// 展示,attacker-centric 输出密度降一个量级,且更贴近真实 MEV 语义 ——
// 一个 bot 的多笔 frontrun 和多笔 backrun 本就是一次夹击的组成部分,逐对看没意义。
//
// 判定 (保留金额闭合, 改成 aggregate 口径):
//
//	SharedUserPool = {任一 front 对手池} ∩ {任一 back 对手池} ∩ userPools 非空
//	AmountsCancel  = Σback.InAmount ∈ [30%, 300%] × Σfront.OutAmount (user.OutMint 侧)
//	IsSandwich     = SharedUserPool != "" && AmountsCancel
//
// 返回 slice 已排序: IsSandwich 优先, 再按 SharedUserPool != "" 优先,
// 最后按最早 front 的 (slot, txIndex) 升序。
func evaluateAttackers(user Swap, fronts, backs []Swap, userPools map[string]bool, swaps []Swap) []AttackerEvidence {
	if len(fronts) == 0 || len(backs) == 0 {
		return nil
	}

	// Union-find 把 Owner 和 FeePayer 合并成 attacker 组。这样
	//   front(owner=A, fp=X) 和 back(owner=B, fp=X) 会被归到同一个 attacker (通过 fp=X 桥接);
	//   front(owner=A, fp=A) 和 back(owner=A, fp=A) 也自然归一起。
	// 对应原来的 sameTrader(): owner 相同 or feePayer 相同。
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if x == "" {
			return ""
		}
		if p, ok := parent[x]; ok && p != x {
			r := find(p)
			parent[x] = r
			return r
		}
		parent[x] = x
		return x
	}
	union := func(a, b string) {
		if a == "" || b == "" {
			return
		}
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	addKeys := func(s Swap) string {
		primary := s.Owner
		if primary == "" {
			primary = s.FeePayer
		}
		if primary == "" {
			return ""
		}
		_ = find(primary)
		if s.Owner != "" && s.FeePayer != "" && s.Owner != s.FeePayer {
			union(s.Owner, s.FeePayer)
		}
		return find(primary)
	}

	type group struct {
		root     string
		ev       *AttackerEvidence
		firstKey string // 第一次看到时的 Owner (或 FeePayer),用于展示
		firstVia string
	}
	groups := map[string]*group{}
	get := func(root string, s Swap) *group {
		if g, ok := groups[root]; ok {
			return g
		}
		key := s.Owner
		via := "owner"
		if key == "" {
			key = s.FeePayer
			via = "feePayer"
		}
		g := &group{
			root:     root,
			firstKey: key,
			firstVia: via,
			ev: &AttackerEvidence{
				AttackerKey:      key,
				AttackerKeyVia:   via,
				FrontSpentInMint: big.NewInt(0),
				FrontGotOutMint:  big.NewInt(0),
				BackSpentOutMint: big.NewInt(0),
				BackGotInMint:    big.NewInt(0),
			},
		}
		groups[root] = g
		return g
	}

	for _, s := range fronts {
		r := addKeys(s)
		if r == "" {
			continue
		}
		g := get(r, s)
		g.ev.Frontruns = append(g.ev.Frontruns, s)
		if s.InAmount != nil {
			g.ev.FrontSpentInMint.Add(g.ev.FrontSpentInMint, s.InAmount)
		}
		if s.OutAmount != nil {
			g.ev.FrontGotOutMint.Add(g.ev.FrontGotOutMint, s.OutAmount)
		}
	}
	for _, s := range backs {
		r := addKeys(s)
		if r == "" {
			continue
		}
		g := get(r, s)
		g.ev.Backruns = append(g.ev.Backruns, s)
		if s.InAmount != nil {
			g.ev.BackSpentOutMint.Add(g.ev.BackSpentOutMint, s.InAmount)
		}
		if s.OutAmount != nil {
			g.ev.BackGotInMint.Add(g.ev.BackGotInMint, s.OutAmount)
		}
	}

	// 只保留两边都有的 attacker(单边不构成夹击候选)
	out := make([]AttackerEvidence, 0, len(groups))
	for _, g := range groups {
		if len(g.ev.Frontruns) == 0 || len(g.ev.Backruns) == 0 {
			continue
		}

		// 找 SharedUserPool
		fPools := map[string]bool{}
		for _, f := range g.ev.Frontruns {
			for p := range counterpartyPools(f, swaps) {
				fPools[p] = true
			}
		}
	outer:
		for _, b := range g.ev.Backruns {
			for p := range counterpartyPools(b, swaps) {
				if fPools[p] && userPools[p] {
					g.ev.SharedUserPool = p
					break outer
				}
			}
		}

		g.ev.AmountsCancel = amountsRoughlyCancelSum(g.ev.FrontGotOutMint, g.ev.BackSpentOutMint)
		g.ev.IsSandwich = g.ev.SharedUserPool != "" && g.ev.AmountsCancel

		// IsClassic: 存在一对 (front, back) 满足 front.Slot == user.Slot == back.Slot。
		// Jito bundle 会把整组 tx 打进同一 slot 保原子性;跨 slot 的是公网抢跑,非经典。
		// 只要一对满足就算 classic —— 同一 attacker 可能同时跑多对,只要有 1 对 bundled 就是。
		for _, f := range g.ev.Frontruns {
			if f.Slot != user.Slot {
				continue
			}
			for _, bk := range g.ev.Backruns {
				if bk.Slot == user.Slot {
					g.ev.IsClassic = true
					break
				}
			}
			if g.ev.IsClassic {
				break
			}
		}

		if g.ev.IsSandwich {
			profit := new(big.Int).Sub(g.ev.BackGotInMint, g.ev.FrontSpentInMint)
			if profit.Sign() > 0 {
				g.ev.NetProfitInMint = profit
			}
		}

		out = append(out, *g.ev)
	}

	sort.Slice(out, func(i, j int) bool {
		// IsSandwich 优先,其次 IsClassic (classic > atypical),再 SharedUserPool 非空,最后时序。
		if out[i].IsSandwich != out[j].IsSandwich {
			return out[i].IsSandwich
		}
		if out[i].IsClassic != out[j].IsClassic {
			return out[i].IsClassic
		}
		ai := out[i].SharedUserPool != ""
		aj := out[j].SharedUserPool != ""
		if ai != aj {
			return ai
		}
		fi := out[i].Frontruns[0]
		fj := out[j].Frontruns[0]
		if fi.Slot != fj.Slot {
			return fi.Slot < fj.Slot
		}
		return fi.TxIndex < fj.TxIndex
	})
	return out
}

// amountsRoughlyCancelSum 是 aggregate 版 amountsRoughlyCancel: 作用于 Σ front.Out 和 Σ back.In。
func amountsRoughlyCancelSum(frontOutSum, backInSum *big.Int) bool {
	if frontOutSum == nil || backInSum == nil {
		return false
	}
	if frontOutSum.Sign() <= 0 || backInSum.Sign() <= 0 {
		return false
	}
	lhs := new(big.Int).Mul(backInSum, big.NewInt(amountCancelDenom))
	lo := new(big.Int).Mul(frontOutSum, big.NewInt(amountCancelMinNum))
	hi := new(big.Int).Mul(frontOutSum, big.NewInt(amountCancelMaxNum))
	return lhs.Cmp(lo) >= 0 && lhs.Cmp(hi) <= 0
}

// attackerReasons 产生 Sandwiched verdict 的 reason 列表。地址不缩写, 便于 grep / 核对。
func attackerReasons(a AttackerEvidence) []string {
	kind := "atypical (cross-slot, public mempool race)"
	if a.IsClassic {
		kind = "classic (same-slot, likely Jito bundle)"
	}
	return []string{
		fmt.Sprintf("%s sandwich: attacker %s (matched via %s) ran %d frontrun(s) and %d backrun(s) on shared pool %s",
			kind, a.AttackerKey, a.AttackerKeyVia,
			len(a.Frontruns), len(a.Backruns),
			a.SharedUserPool),
	}
}

// amountsRoughlyCancel 校验 back.InAmount 与 front.OutAmount 大致抵消。
//
// 真实夹子中 bot 的 back-run 是为了卖出 front-run 抢进的筹码,所以
// back.InAmount (bot 卖出量) 应与 front.OutAmount (bot 抢进量) 同量级。
// 持仓累积、re-entry、partial fill 等偏离会让比例偏移 1:1,但不应超出一个量级。
//
// 容差 [30%, 300%] 由 amountCancelMinNum/amountCancelMaxNum 定义,用整数运算避免浮点。
func amountsRoughlyCancel(front, back Swap) bool {
	if front.OutAmount == nil || back.InAmount == nil {
		return false
	}
	if front.OutAmount.Sign() <= 0 || back.InAmount.Sign() <= 0 {
		return false
	}
	// 判 back.InAmount ∈ [front.OutAmount * min/denom, front.OutAmount * max/denom]
	// 等价 back.InAmount * denom ∈ [front.OutAmount * min, front.OutAmount * max]
	lhs := new(big.Int).Mul(back.InAmount, big.NewInt(amountCancelDenom))
	lo := new(big.Int).Mul(front.OutAmount, big.NewInt(amountCancelMinNum))
	hi := new(big.Int).Mul(front.OutAmount, big.NewInt(amountCancelMaxNum))
	return lhs.Cmp(lo) >= 0 && lhs.Cmp(hi) <= 0
}

// estimateLoss 按 constant product (x + dx)(y - dy) = xy 粗略估算夹子损失。
//
// 精度 ±30%，仅适用于 AMM v2 型池；CLMM/stable swap 偏差更大。
//
// 参数:
//
//	preX - front-run 前 pool 里 user.InMint 侧的余额
//	preY - front-run 前 pool 里 user.OutMint 侧的余额
//	userIn - user 实际付出的 InMint 数量
//	userActualOut - user 实际获得的 OutMint 数量
//
// 返回:
//
//	loss = idealOut - userActualOut，若 <= 0 或输入无效返回 nil
func estimateLoss(preX, preY, userIn, userActualOut *big.Int) *big.Int {
	if preX == nil || preY == nil || userIn == nil || userActualOut == nil {
		return nil
	}
	if preX.Sign() <= 0 || preY.Sign() <= 0 || userIn.Sign() <= 0 {
		return nil
	}
	// idealOut = preY - preX*preY / (preX + userIn)
	newX := new(big.Int).Add(preX, userIn)
	xy := new(big.Int).Mul(preX, preY)
	remainY := new(big.Int).Quo(xy, newX)
	idealOut := new(big.Int).Sub(preY, remainY)

	loss := new(big.Int).Sub(idealOut, userActualOut)
	if loss.Sign() <= 0 {
		return nil
	}
	return loss
}
