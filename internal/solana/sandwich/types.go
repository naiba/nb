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
