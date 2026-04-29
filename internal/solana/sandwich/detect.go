package sandwich

import (
	"fmt"
	"math/big"
	"sort"
)

// Detect 在 []Swap 中识别夹子模式，输出二分 Verdict。
//
// 仅当严格夹击（同一 trader 在 user 前后做反向操作，且至少一边碰到 user 的池）
// 命中时才判定为 Sandwiched；其他所有情况统一为 NotSandwiched。
// 相关的 pool-touching 交易在 RelatedPoolTxs 中原样暴露，供人工判断。
func Detect(userSig, userAddr, userMint string, swaps []Swap) Verdict {
	sort.Slice(swaps, func(i, j int) bool {
		if swaps[i].Slot != swaps[j].Slot {
			return swaps[i].Slot < swaps[j].Slot
		}
		return swaps[i].TxIndex < swaps[j].TxIndex
	})

	userSwap, userPools, ok := findUserSwap(userSig, userAddr, swaps)
	if !ok {
		return Verdict{
			Level:   NotSandwiched,
			Reasons: []string{"user swap not found in scanned blocks"},
		}
	}

	if userSwap.IsMultiMint {
		return Verdict{
			Level:    NotSandwiched,
			UserSwap: userSwap,
			Reasons:  []string{"user tx is a multi-hop route (Jupiter etc.), sandwich detection not supported"},
		}
	}

	related := findRelatedPoolTxs(userSwap, userPools, swaps)

	// Sandwiched: A+B+C+D 全中
	if front, back, ok := findSandwichPair(userSwap, userPools, swaps); ok {
		return Verdict{
			Level:    Sandwiched,
			UserSwap: userSwap,
			FrontRun: &front,
			BackRun:  &back,
			Attacker: attackerID(front),
			Reasons: []string{fmt.Sprintf("front-run at tx#%d and back-run at tx#%d by attacker %s on the same pool as user",
				front.TxIndex, back.TxIndex, firstN(attackerID(front), 8))},
			RelatedPoolTxs: related,
		}
	}

	return Verdict{
		Level:          NotSandwiched,
		UserSwap:       userSwap,
		Reasons:        []string{"no front+back pair by same trader on user's pools"},
		RelatedPoolTxs: related,
	}
}

// findUserSwap 定位 user 的 trader swap 并收集其对手 pool 集合。
func findUserSwap(userSig, userAddr string, swaps []Swap) (Swap, map[string]bool, bool) {
	var userSwap Swap
	found := false
	pools := make(map[string]bool)
	for _, s := range swaps {
		if s.Signature == userSig {
			if s.Owner == userAddr && s.IsTrader {
				userSwap = s
				found = true
			} else if !s.IsTrader {
				pools[s.Owner] = true
			}
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

// findSandwichPair 查找同一 trader 在 user 前后反向操作、且对手池与 user 共享的 (front, back) 对。
// D 条件（同池）是 gating：不满足时不算 Sandwiched，上游返回 NotSandwiched。
//
// 条件:
//
//	A: front.TxIndex < user.TxIndex < back.TxIndex，且 front/back 都是 trader swap
//	B: front 和 back 同一 trader（Owner 或 FeePayer 匹配）
//	C: front 同方向（in/out mint 与 user 相同），back 反方向
//	D: front 或 back 的对手池与 user 的对手池集合有交集（gating）
func findSandwichPair(user Swap, userPools map[string]bool, swaps []Swap) (Swap, Swap, bool) {
	var fronts, backs []Swap
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
	for _, f := range fronts {
		for _, b := range backs {
			if !sameTrader(f, b) {
				continue
			}
			if sharesCounterpartyPool(f, b, userPools, swaps) {
				return f, b, true
			}
		}
	}
	return Swap{}, Swap{}, false
}

func sameTrader(a, b Swap) bool {
	if a.Owner != "" && a.Owner == b.Owner {
		return true
	}
	if a.FeePayer != "" && a.FeePayer == b.FeePayer {
		return true
	}
	return false
}

func sharesCounterpartyPool(f, b Swap, userPools map[string]bool, swaps []Swap) bool {
	for p := range counterpartyPools(f, swaps) {
		if userPools[p] {
			return true
		}
	}
	for p := range counterpartyPools(b, swaps) {
		if userPools[p] {
			return true
		}
	}
	return false
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
