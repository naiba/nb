// internal/solana/sandwich/types.go
package sandwich

import (
	"math/big"

	"github.com/gagliardetto/solana-go/rpc"
)

// TokenFlow 表示一个账户在一笔 tx 中对某个 mint 的净余额变化。
type TokenFlow struct {
	AccountIndex uint16
	Owner        string
	Mint         string
	Delta        *big.Int // post - pre, 原子单位(含 decimals)
}

// Swap 表示一笔 tx 中按 owner 分组的一个"参与方"视图。
// 一笔 tx 可以产生多个 Swap（trader + 各 pool）。
type Swap struct {
	Signature string
	Slot      uint64
	TxIndex   int    // 在 block 内的位置，与 Slot 组合作为排序键
	FeePayer  string // signer[0]

	Owner    string
	IsTrader bool // true=交易者, false=pool vault owner

	InMint    string
	InAmount  *big.Int
	OutMint   string
	OutAmount *big.Int

	IsMultiMint bool // 3+ mint 变化或同号 2 个 mint，In/Out 不填
}

// Context 是 fetch 阶段的输出，供后续纯函数阶段使用。
type Context struct {
	UserSig      string
	UserSlot     uint64
	UserAddr     string
	UserMint     string
	UserTokenAcc string                         // user ATA
	Blocks       map[uint64]*rpc.GetBlockResult // slot -> block
	UserTx       *rpc.GetTransactionResult
	SkippedSlots []uint64
}

// VerdictLevel 只做二分：Sandwiched 或 NotSandwiched。
// 严格 Sandwiched（同 trader 前后对冲 + 共享 user 池）之外的所有情况统一为 NotSandwiched，
// 相关交易列在 RelatedPoolTxs，用户从原始数据自行判断。
type VerdictLevel int

const (
	NotSandwiched VerdictLevel = iota
	Sandwiched
)

func (v VerdictLevel) String() string {
	switch v {
	case Sandwiched:
		return "SANDWICHED"
	case NotSandwiched:
		return "NOT_SANDWICHED"
	}
	return "UNKNOWN"
}

// Verdict 是 detect 阶段输出的最终判定。
type Verdict struct {
	Level        VerdictLevel
	UserSwap     Swap
	FrontRun     *Swap
	BackRun      *Swap
	Attacker     string   // fee_payer 或 owner
	LossEstimate *big.Int // user.OutMint 计价，可 nil
	Reasons      []string
	// RelatedPoolTxs 是所有碰到 user 池的其他 tx（按 signature 维度扫描）。
	// 无论 Verdict 级别如何都填充，便于用户直接审阅所有相关活动。
	RelatedPoolTxs []Swap
}

// AttackerEvidence 按单个 attacker (同 owner 或 feePayer 归并) 聚合其 frontrun + backrun
// 活动,并给出同池判定 + 金额闭合判定 + 粗略利润。
//
// 设计参考 EVM 版 (chain-toolkit/src/func/analyze_evm_swap_tx.rs) 的 sender 聚合逻辑:
// 按攻击者实体而非 (front, back) 笛卡尔对展示,每个 attacker 一条 profile,避免噪声。
//
// 方向口径说明 (以 user 为基准):
//
//	user 卖 user.InMint 买 user.OutMint
//	attacker frontrun 同方向: 卖 user.InMint, 买 user.OutMint
//	attacker backrun  反方向: 卖 user.OutMint, 买 user.InMint
//
// 所以聚合量为:
//
//	FrontSpentInMint = Σ frontruns.InAmount  (attacker 在 user.InMint 侧投入)
//	FrontGotOutMint  = Σ frontruns.OutAmount (attacker 抢进的 user.OutMint)
//	BackSpentOutMint = Σ backruns.InAmount   (attacker 甩出的 user.OutMint)
//	BackGotInMint    = Σ backruns.OutAmount  (attacker 收回的 user.InMint)
type AttackerEvidence struct {
	AttackerKey    string // Owner 优先, 为空时回落到 FeePayer
	AttackerKeyVia string // "owner" / "feePayer"

	Frontruns []Swap
	Backruns  []Swap

	// SharedUserPool 是 {任一 front 的对手池} ∩ {任一 back 的对手池} ∩ userPools 的一个元素。
	// 非空即 "同池硬条件" 通过 —— 这是参考 EVM 做法: 不同池直接过滤掉,不再进入判定。
	SharedUserPool string

	// 聚合金额
	FrontSpentInMint *big.Int
	FrontGotOutMint  *big.Int
	BackSpentOutMint *big.Int
	BackGotInMint    *big.Int

	// AmountsCancel: BackSpentOutMint ∈ [30%, 300%] × FrontGotOutMint
	// 判定是 aggregate 级别,同一 attacker 的多笔 front 和多笔 back 总量对比。
	AmountsCancel bool

	// IsSandwich = SharedUserPool != "" && AmountsCancel。
	IsSandwich bool

	// IsClassic 表示 front / user / back 全部在 user 的同一 slot 内 (典型 Jito bundle 节奏).
	// false 则为跨 slot 的 "atypical" 夹击 (公网 mempool 抢跑, 非 bundle 原子化保障)。
	// 仅在 IsSandwich=true 时有意义。
	IsClassic bool

	// NetProfitInMint = BackGotInMint - FrontSpentInMint (user.InMint 计价,粗略)。
	// 仅在 IsSandwich=true 且 > 0 时填。CLMM/多跳/手续费会让这个估算偏离真实利润。
	NetProfitInMint *big.Int
}

// DetailedReport 是默认 csa 输出需要的全部信息。相比裸 Verdict,它额外包含:
//   - 相关 tx 的完整 Swap 列表(trader + 所有 pool 视角),供展示资金流
//   - 按 attacker 聚合的 AttackerEvidence 列表,代替旧的 pair-level 笛卡尔积评估,
//     让用户一眼看出 "有几个玩家在你前后做了反向操作,各在哪些池,闭合与否"。
type DetailedReport struct {
	Verdict Verdict

	// TxFlows: signature -> 该 tx 所有 Swap(含 trader 和 pool 视角)。
	// 用于按 owner 分组渲染完整资金流。
	TxFlows map[string][]Swap

	// RelatedSigs 按展示顺序排好: user 前的 tx (slot,txIndex 升序) → user → user 后的 tx。
	// 只包含 Format 要展示的 tx,不等于所有扫描到的 tx。
	RelatedSigs []string

	// UserPools 是 user 自己 tx 中碰到的对手池 owner 集合(去重,排序)。
	UserPools []string

	// 所有 A+C 命中的 trader swap(保留给 Detection 段做总量统计)。
	CandidateFronts []Swap
	CandidateBacks  []Swap

	// Attackers 按 attacker 聚合的证据,sort: IsSandwich=true 优先,其次 SharedUserPool!="" ,
	// 再按最早 front 的 (slot, txIndex) 升序。
	Attackers []AttackerEvidence
}
