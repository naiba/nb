package sandwich

import (
	"math/big"
	"sort"

	"github.com/gagliardetto/solana-go/rpc"
)

// extractFlows 按 AccountIndex 对齐 Pre/Post，计算每个账户的 delta。
//
// 修复旧 check_sandwich_attack.go 的 bug：
// 旧代码用 PreTokenBalances[balanceChangeIdx] 按数组位置取配对，
// 但 RPC 返回的 Pre/Post 两数组长度和顺序不保证一致，会算错 delta。
// 这里改为按 AccountIndex 字段对齐。
func extractFlows(pre, post []rpc.TokenBalance) []TokenFlow {
	preByIdx := make(map[uint16]rpc.TokenBalance, len(pre))
	for _, b := range pre {
		preByIdx[b.AccountIndex] = b
	}
	postByIdx := make(map[uint16]rpc.TokenBalance, len(post))
	for _, b := range post {
		postByIdx[b.AccountIndex] = b
	}

	flows := make([]TokenFlow, 0, len(preByIdx)+len(postByIdx))

	emit := func(idx uint16, b rpc.TokenBalance, delta *big.Int) {
		if delta.Sign() == 0 {
			return
		}
		owner := ""
		if b.Owner != nil {
			owner = b.Owner.String()
		}
		flows = append(flows, TokenFlow{
			AccountIndex: idx,
			Owner:        owner,
			Mint:         b.Mint.String(),
			Delta:        delta,
		})
	}

	for idx, p := range preByIdx {
		q, ok := postByIdx[idx]
		if !ok {
			// 账户被 close：delta = -pre
			preAmt := amountToBigInt(p.UiTokenAmount)
			emit(idx, p, new(big.Int).Neg(preAmt))
			continue
		}
		preAmt := amountToBigInt(p.UiTokenAmount)
		postAmt := amountToBigInt(q.UiTokenAmount)
		emit(idx, q, new(big.Int).Sub(postAmt, preAmt))
	}
	for idx, q := range postByIdx {
		if _, inPre := preByIdx[idx]; inPre {
			continue
		}
		// 新 ATA：delta = +post
		emit(idx, q, amountToBigInt(q.UiTokenAmount))
	}

	// 按 AccountIndex 排序，让下游日志和测试的输出确定性，便于调试。
	sort.Slice(flows, func(i, j int) bool {
		return flows[i].AccountIndex < flows[j].AccountIndex
	})
	return flows
}

// amountToBigInt 解析 UiTokenAmount.Amount（原子单位的十进制整数字符串）为 *big.Int。
//
// Contract: 对 nil/解析失败返回 big.NewInt(0)，不返回 error。
// 这是保守策略：错误 tx 的余额视为"无变化"好过让整个分析崩溃。
// 下游 extractFlows 会用 Sign() 过滤掉 zero-delta，所以静默 0 不会污染结果。
func amountToBigInt(u *rpc.UiTokenAmount) *big.Int {
	if u == nil {
		return big.NewInt(0)
	}
	n, ok := new(big.Int).SetString(u.Amount, 10)
	if !ok {
		return big.NewInt(0)
	}
	return n
}

// WrappedSolMint 是 wrapped SOL 的 mint 地址，用于把 native SOL 的 lamport 变化
// 合成为 TokenFlow，使得 native SOL 配对的 swap 能被 ExtractSwaps 正确识别为双边。
const WrappedSolMint = "So11111111111111111111111111111111111111112"

// extractLamportFlows 把 tx 中每个账户的 lamport 变化合成为 wSOL mint 的 TokenFlow。
//
// 修复 bug: 过去只看 Pre/PostTokenBalances（SPL 侧),
// 当 user 用 native SOL 作为对手资产时（绝大多数 Solana swap），SOL 落在 wallet 的
// 原生 lamport balance 而非 wSOL ATA 上，user 只出现 -mintX 单边变动，
// classifySwapDirection 会把它当成单边转账跳过，findUserSwap 也就找不到 user swap。
// 这里把每个账户的 lamport delta 合成 wSOL flow，与 token flow 合并后再分组,
// 让 user 被识别为完整的双边 swap。
//
// 参数:
//
//	accountKeys  - tx.Message.AccountKeys 按索引排列的 base58 地址
//	preBalances  - tx.Meta.PreBalances，每个索引一个 lamport 值
//	postBalances - tx.Meta.PostBalances
//	fee          - tx.Meta.Fee，从 fee_payer 的变化里扣掉
//	feePayerIdx  - fee_payer 在 accountKeys 中的索引（通常是 0）
func extractLamportFlows(accountKeys []string, preBalances, postBalances []uint64,
	fee uint64, feePayerIdx int) []TokenFlow {

	n := len(accountKeys)
	if len(preBalances) < n {
		n = len(preBalances)
	}
	if len(postBalances) < n {
		n = len(postBalances)
	}

	flows := make([]TokenFlow, 0, n)
	for i := 0; i < n; i++ {
		post := new(big.Int).SetUint64(postBalances[i])
		pre := new(big.Int).SetUint64(preBalances[i])
		delta := new(big.Int).Sub(post, pre)
		if i == feePayerIdx && fee > 0 {
			// fee_payer 的 delta 加回 fee（fee 不是 swap 的一部分,
			// 否则会把手续费误算为 swap 的"损失"）
			delta.Add(delta, new(big.Int).SetUint64(fee))
		}
		if delta.Sign() == 0 {
			continue
		}
		flows = append(flows, TokenFlow{
			AccountIndex: uint16(i),
			Owner:        accountKeys[i], // native SOL 的 "owner" 就是账户本身
			Mint:         WrappedSolMint,
			Delta:        delta,
		})
	}
	return flows
}

// ExtractSwaps 从一笔 tx 的 token balance 变化中提取 []Swap。
// 一笔 tx 可能产生多个 Swap（trader + 各 pool）。
//
// 注意：此签名不处理 native SOL。对于 native SOL 对手资产的 swap，
// 请使用 ExtractSwapsFull。本函数保留作为旧接口兼容。
func ExtractSwaps(signature string, slot uint64, txIndex int, feePayer string,
	pre, post []rpc.TokenBalance) []Swap {

	flows := extractFlows(pre, post)
	if len(flows) == 0 {
		return nil
	}
	return groupFlowsToSwaps(signature, slot, txIndex, feePayer, flows)
}

// ExtractSwapsFull 是 ExtractSwaps 的完整版本，额外处理 native SOL 的 lamport 变化。
//
// 对于 token account 的 balance 变化，走现有 extractFlows 路径（按 AccountIndex 对齐）。
// 对于 native SOL，用 extractLamportFlows 合成 wSOL mint 的 flows，与 token flows 合并。
func ExtractSwapsFull(signature string, slot uint64, txIndex int, feePayer string,
	accountKeys []string, preBalances, postBalances []uint64, fee uint64, feePayerIdx int,
	preTB, postTB []rpc.TokenBalance) []Swap {

	flows := extractFlows(preTB, postTB)
	flows = append(flows, extractLamportFlows(accountKeys, preBalances, postBalances, fee, feePayerIdx)...)

	if len(flows) == 0 {
		return nil
	}
	return groupFlowsToSwaps(signature, slot, txIndex, feePayer, flows)
}

// groupFlowsToSwaps 把 []TokenFlow 按 Owner 分组，每个 owner 生成一个 Swap。
func groupFlowsToSwaps(signature string, slot uint64, txIndex int, feePayer string,
	flows []TokenFlow) []Swap {

	// 按 Owner 分组
	byOwner := make(map[string][]TokenFlow)
	for _, f := range flows {
		byOwner[f.Owner] = append(byOwner[f.Owner], f)
	}

	// 统计每个 Owner 在此 tx 出现的 token account 数和不同 mint 数，用于 IsTrader 判定
	ownerAccCount := make(map[string]int)
	ownerMintSet := make(map[string]map[string]struct{})
	for _, f := range flows {
		ownerAccCount[f.Owner]++
		if _, ok := ownerMintSet[f.Owner]; !ok {
			ownerMintSet[f.Owner] = make(map[string]struct{})
		}
		ownerMintSet[f.Owner][f.Mint] = struct{}{}
	}

	swaps := make([]Swap, 0, len(byOwner))
	for owner, ofs := range byOwner {
		s := Swap{
			Signature: signature,
			Slot:      slot,
			TxIndex:   txIndex,
			FeePayer:  feePayer,
			Owner:     owner,
			IsTrader:  classifyIsTrader(owner, feePayer, ownerAccCount[owner], len(ownerMintSet[owner])),
		}
		// 若 Owner 只有单向转账（纯 transfer，不是 swap），跳过——
		// 避免把转手续费、空投领取等无关 tx 当成 swap 污染下游。
		if !classifySwapDirection(&s, ofs) {
			continue
		}
		swaps = append(swaps, s)
	}

	// 按 Owner 排序，与 extractFlows 的 deterministic 风格保持一致，
	// 便于测试断言和调试日志比对。
	sort.Slice(swaps, func(i, j int) bool {
		return swaps[i].Owner < swaps[j].Owner
	})
	return swaps
}

// classifyIsTrader 启发式判定 Owner 是 trader 还是 pool。
// 规则:
//  1. Owner == FeePayer → trader（强信号）
//  2. Owner 在此 tx 有 2+ 个 token account 且涉及 2+ 个不同 mint → pool（AMM pool 有多个 vault）
//  3. 其他 → trader
func classifyIsTrader(owner, feePayer string, accCount, mintCount int) bool {
	if owner == feePayer {
		return true
	}
	if accCount >= 2 && mintCount >= 2 {
		return false
	}
	return true
}

// classifySwapDirection 根据 flows 填充 In/Out 字段，或标记 MultiMint。
// 同一 owner 对同一 mint 可能有多个 ATA（例如 wSOL temp + ATA），先聚合。
//
// 返回值语义：
//   - true  = 该 Owner 构成有效 swap（含 MultiMint），调用方应保留该 Swap。
//   - false = 非 swap（单边转账/手续费/空投等），调用方应跳过。
func classifySwapDirection(s *Swap, flows []TokenFlow) bool {
	mintDelta := make(map[string]*big.Int)
	for _, f := range flows {
		if existing, ok := mintDelta[f.Mint]; ok {
			mintDelta[f.Mint] = new(big.Int).Add(existing, f.Delta)
		} else {
			mintDelta[f.Mint] = new(big.Int).Set(f.Delta)
		}
	}

	var negMints, posMints []string
	for m, d := range mintDelta {
		switch d.Sign() {
		case -1:
			negMints = append(negMints, m)
		case 1:
			posMints = append(posMints, m)
		}
	}

	// 1 进 1 出 = 标准 swap
	if len(negMints) == 1 && len(posMints) == 1 {
		s.InMint = negMints[0]
		s.InAmount = new(big.Int).Abs(mintDelta[negMints[0]])
		s.OutMint = posMints[0]
		s.OutAmount = new(big.Int).Set(mintDelta[posMints[0]])
		return true
	}

	// 单边变动（纯转账/手续费/空投），不是 swap
	if len(negMints)+len(posMints) < 2 {
		return false
	}

	// 其余情况（3+ mints、同号 2 mints）标记 MultiMint。
	// 注意：全部聚合为零的场景会被上面 len(negMints)+len(posMints) < 2 分支先行过滤，
	// 不会进入此处，所以不要在注释里把它列为 MultiMint 情形。
	s.IsMultiMint = true
	return true
}
