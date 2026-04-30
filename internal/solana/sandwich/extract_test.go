package sandwich

import (
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// mkBalance 构造一条 TokenBalance，避免测试里重复样板代码。
func mkBalance(accIdx uint16, owner, mint, amount string, decimals uint8) rpc.TokenBalance {
	o := solana.MustPublicKeyFromBase58(owner)
	return rpc.TokenBalance{
		AccountIndex: accIdx,
		Mint:         solana.MustPublicKeyFromBase58(mint),
		Owner:        &o,
		UiTokenAmount: &rpc.UiTokenAmount{
			Amount:   amount,
			Decimals: decimals,
		},
	}
}

// 占位地址（合法 base58 PublicKey）
const (
	addrA = "11111111111111111111111111111112"
	addrB = "11111111111111111111111111111113"
	mintX = "11111111111111111111111111111114"
	mintY = "11111111111111111111111111111115"
)

func TestExtractFlows_AlignsByAccountIndex(t *testing.T) {
	// Pre 数组位置 0 是 acc#5, 位置 1 是 acc#3
	// Post 数组位置 0 是 acc#3, 位置 1 是 acc#5
	// 若按数组位置配对会算错 delta
	pre := []rpc.TokenBalance{
		mkBalance(5, addrA, mintX, "1000", 6),
		mkBalance(3, addrB, mintX, "2000", 6),
	}
	post := []rpc.TokenBalance{
		mkBalance(3, addrB, mintX, "2500", 6),
		mkBalance(5, addrA, mintX, "800", 6),
	}

	flows := extractFlows(pre, post)

	byIdx := map[uint16]*TokenFlow{}
	for i := range flows {
		byIdx[flows[i].AccountIndex] = &flows[i]
	}

	if got := byIdx[5].Delta.Int64(); got != -200 {
		t.Errorf("acc#5 delta: got %d, want -200", got)
	}
	if got := byIdx[3].Delta.Int64(); got != 500 {
		t.Errorf("acc#3 delta: got %d, want 500", got)
	}
}

func TestExtractFlows_SkipsZeroDelta(t *testing.T) {
	pre := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "100", 6)}
	post := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "100", 6)}

	flows := extractFlows(pre, post)
	if len(flows) != 0 {
		t.Errorf("expected 0 flows (zero delta filtered), got %d", len(flows))
	}
}

func TestExtractFlows_PostOnlyMeansATACreated(t *testing.T) {
	pre := []rpc.TokenBalance{}
	post := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "500", 6)}

	flows := extractFlows(pre, post)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	if flows[0].Delta.Cmp(big.NewInt(500)) != 0 {
		t.Errorf("post-only delta: got %s, want 500", flows[0].Delta)
	}
}

func TestExtractFlows_PreOnlyMeansAccountClosed(t *testing.T) {
	pre := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "500", 6)}
	post := []rpc.TokenBalance{}

	flows := extractFlows(pre, post)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	if flows[0].Delta.Cmp(big.NewInt(-500)) != 0 {
		t.Errorf("pre-only delta: got %s, want -500", flows[0].Delta)
	}
}

func TestExtractFlows_PreservesOwnerAndMint(t *testing.T) {
	pre := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "100", 6)}
	post := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "50", 6)}

	flows := extractFlows(pre, post)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	if flows[0].Owner != addrA {
		t.Errorf("Owner: got %q, want %q", flows[0].Owner, addrA)
	}
	if flows[0].Mint != mintX {
		t.Errorf("Mint: got %q, want %q", flows[0].Mint, mintX)
	}
}

func TestAmountToBigInt_NilReturnsZero(t *testing.T) {
	got := amountToBigInt(nil)
	if got.Sign() != 0 {
		t.Errorf("nil input: got %s, want 0", got)
	}
}

func TestAmountToBigInt_LargeValueSurvivesUint64(t *testing.T) {
	// 超过 uint64 max (1.8e19)，验证 big.Int 路径正确
	huge := "99999999999999999999999"
	u := &rpc.UiTokenAmount{Amount: huge, Decimals: 6}
	got := amountToBigInt(u)
	expected, _ := new(big.Int).SetString(huge, 10)
	if got.Cmp(expected) != 0 {
		t.Errorf("large value: got %s, want %s", got, expected)
	}
}

// fakeTx 是测试辅助，构造最小的输入组合。
type fakeTx struct {
	sig      string
	slot     uint64
	txIndex  int
	feePayer string
	pre      []rpc.TokenBalance
	post     []rpc.TokenBalance
}

func extractSwapsFromFake(f fakeTx) []Swap {
	return ExtractSwaps(f.sig, f.slot, f.txIndex, f.feePayer, f.pre, f.post)
}

func TestExtractSwaps_TraderSwapInAndOut(t *testing.T) {
	// trader addrA 用 mintY 换 mintX：付 100 Y，得 50 X
	pre := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "0", 6),
		mkBalance(2, addrA, mintY, "100", 6),
	}
	post := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "50", 6),
		mkBalance(2, addrA, mintY, "0", 6),
	}
	swaps := extractSwapsFromFake(fakeTx{
		sig: "sig1", slot: 100, txIndex: 5, feePayer: addrA,
		pre: pre, post: post,
	})

	if len(swaps) != 1 {
		t.Fatalf("expected 1 swap, got %d", len(swaps))
	}
	s := swaps[0]
	if s.Owner != addrA {
		t.Errorf("owner: got %s, want %s", s.Owner, addrA)
	}
	if !s.IsTrader {
		t.Errorf("IsTrader should be true (owner == feePayer)")
	}
	if s.InMint != mintY {
		t.Errorf("InMint: got %s, want %s", s.InMint, mintY)
	}
	if s.InAmount.Int64() != 100 {
		t.Errorf("InAmount: got %s, want 100", s.InAmount)
	}
	if s.OutMint != mintX {
		t.Errorf("OutMint: got %s, want %s", s.OutMint, mintX)
	}
	if s.OutAmount.Int64() != 50 {
		t.Errorf("OutAmount: got %s, want 50", s.OutAmount)
	}
}

func TestExtractSwaps_PoolIdentifiedByTwoMints(t *testing.T) {
	// trader addrA 单边 -mintY, pool addrB 双 mint (+Y -X)
	pre := []rpc.TokenBalance{
		mkBalance(1, addrA, mintY, "100", 6),
		mkBalance(2, addrB, mintX, "10000", 6),
		mkBalance(3, addrB, mintY, "5000", 6),
	}
	post := []rpc.TokenBalance{
		mkBalance(1, addrA, mintY, "0", 6),
		mkBalance(2, addrB, mintX, "9950", 6),
		mkBalance(3, addrB, mintY, "5100", 6),
	}
	// trader 的 mintX 新建在 post
	post = append(post, mkBalance(4, addrA, mintX, "50", 6))

	swaps := extractSwapsFromFake(fakeTx{
		sig: "sig2", slot: 100, txIndex: 6, feePayer: addrA,
		pre: pre, post: post,
	})

	byOwner := map[string]Swap{}
	for _, s := range swaps {
		byOwner[s.Owner] = s
	}
	if len(byOwner) != 2 {
		t.Fatalf("expected 2 swaps (trader+pool), got %d: %+v", len(byOwner), swaps)
	}

	trader := byOwner[addrA]
	if !trader.IsTrader {
		t.Errorf("addrA should be trader (fee_payer)")
	}
	if trader.InMint != mintY || trader.OutMint != mintX {
		t.Errorf("trader direction wrong: in=%s out=%s", trader.InMint, trader.OutMint)
	}

	pool := byOwner[addrB]
	if pool.IsTrader {
		t.Errorf("addrB should be pool (2 mints in same tx)")
	}
	// Pool 从池子视角：收到 Y，付出 X
	if pool.InMint != mintX || pool.OutMint != mintY {
		t.Errorf("pool direction wrong: in=%s out=%s", pool.InMint, pool.OutMint)
	}
}

func TestExtractSwaps_MultiMintMarked(t *testing.T) {
	pre := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "100", 6),
		mkBalance(2, addrA, mintY, "200", 6),
	}
	mintZ := "11111111111111111111111111111116"
	post := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "0", 6),
		mkBalance(2, addrA, mintY, "0", 6),
		mkBalance(3, addrA, mintZ, "50", 6),
	}
	swaps := extractSwapsFromFake(fakeTx{
		sig: "sig3", slot: 100, txIndex: 7, feePayer: addrA,
		pre: pre, post: post,
	})

	if len(swaps) != 1 {
		t.Fatalf("expected 1 swap, got %d", len(swaps))
	}
	if !swaps[0].IsMultiMint {
		t.Errorf("expected IsMultiMint=true for 3-mint owner")
	}
}

func TestExtractSwaps_SingleSideSkipped(t *testing.T) {
	pre := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "100", 6)}
	post := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "50", 6)}

	swaps := extractSwapsFromFake(fakeTx{
		sig: "sig4", slot: 100, txIndex: 8, feePayer: addrA,
		pre: pre, post: post,
	})

	if len(swaps) != 0 {
		t.Errorf("expected 0 swaps (single-side transfer), got %d", len(swaps))
	}
}

func TestExtractSwaps_SameSignTwoMintsIsMultiMint(t *testing.T) {
	// addrA 同时减少 mintX 和 mintY（例如 LP add 或路由付款）→ MultiMint
	pre := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "100", 6),
		mkBalance(2, addrA, mintY, "200", 6),
	}
	post := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "0", 6),
		mkBalance(2, addrA, mintY, "0", 6),
	}
	swaps := extractSwapsFromFake(fakeTx{
		sig: "sigSame", slot: 100, txIndex: 10, feePayer: addrA,
		pre: pre, post: post,
	})
	if len(swaps) != 1 {
		t.Fatalf("expected 1 swap, got %d", len(swaps))
	}
	if !swaps[0].IsMultiMint {
		t.Errorf("same-sign 2 mints should mark IsMultiMint=true")
	}
}

func TestExtractSwaps_MultiAtaSameMintAggregated(t *testing.T) {
	// addrA 持有两个 mintX 账户(临时 + 正式 ATA)：acc#1 +500, acc#3 -300
	// 聚合后 mintX delta = +200, 配合 mintY -100 形成标准 swap
	pre := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "0", 6),
		mkBalance(2, addrA, mintY, "100", 6),
		mkBalance(3, addrA, mintX, "300", 6),
	}
	post := []rpc.TokenBalance{
		mkBalance(1, addrA, mintX, "500", 6),
		mkBalance(2, addrA, mintY, "0", 6),
		mkBalance(3, addrA, mintX, "0", 6),
	}
	swaps := extractSwapsFromFake(fakeTx{
		sig: "sigAgg", slot: 100, txIndex: 11, feePayer: addrA,
		pre: pre, post: post,
	})
	if len(swaps) != 1 {
		t.Fatalf("expected 1 swap, got %d", len(swaps))
	}
	s := swaps[0]
	if s.IsMultiMint {
		t.Errorf("expected aggregated mintX delta = +200 + mintY -100 → standard swap, not MultiMint")
	}
	if s.InMint != mintY || s.InAmount.Int64() != 100 {
		t.Errorf("InMint/Amount wrong: %s/%s", s.InMint, s.InAmount)
	}
	if s.OutMint != mintX || s.OutAmount.Int64() != 200 {
		t.Errorf("OutMint/Amount wrong: %s/%s", s.OutMint, s.OutAmount)
	}
}

func TestExtractLamportFlows_FeeDeducted(t *testing.T) {
	// fee_payer idx=0, pre=100, post=90, fee=5 → delta = (90-100) + 5 = -5
	accountKeys := []string{addrA, addrB}
	pre := []uint64{100, 200}
	post := []uint64{90, 220}
	fee := uint64(5)
	flows := extractLamportFlows(accountKeys, pre, post, fee, 0)

	// addrA: -5 (不是 -10)
	// addrB: +20
	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}
	byOwner := map[string]*big.Int{}
	for i := range flows {
		byOwner[flows[i].Owner] = flows[i].Delta
	}
	if byOwner[addrA].Int64() != -5 {
		t.Errorf("fee_payer delta: got %s, want -5", byOwner[addrA])
	}
	if byOwner[addrB].Int64() != 20 {
		t.Errorf("non-fee-payer delta: got %s, want 20", byOwner[addrB])
	}
}

func TestExtractSwapsFull_NativeSolSwap(t *testing.T) {
	// user (addrA) 卖 mintX 得 SOL：
	// token balance: acc#1 -500 mintX
	// lamports:      acc#0 (user wallet) pre=1_000_000 post=1_500_000 fee=5000
	// fee_payer = addrA
	//
	// 期望 user swap: In=mintX (500), Out=wSOL (500_000 + 5000 = 505_000)
	accountKeys := []string{addrA, addrB}
	preBal := []uint64{1_000_000, 0}
	postBal := []uint64{1_500_000, 0}
	fee := uint64(5000)

	preTB := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "500", 6)}
	postTB := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "0", 6)}

	swaps := ExtractSwapsFull("sigNative", 100, 3, addrA, accountKeys, preBal, postBal, fee, 0, preTB, postTB)

	var userSwap *Swap
	for i := range swaps {
		if swaps[i].Owner == addrA {
			userSwap = &swaps[i]
			break
		}
	}
	if userSwap == nil {
		t.Fatalf("expected a swap for addrA; got swaps=%+v", swaps)
	}
	if userSwap.IsMultiMint {
		t.Errorf("expected standard swap, got MultiMint")
	}
	if userSwap.InMint != mintX || userSwap.InAmount.Int64() != 500 {
		t.Errorf("In: got %s/%s, want mintX/500", userSwap.InMint, userSwap.InAmount)
	}
	// 500_000 lamport(收到 SOL) + 5000(fee_payer 扣回的 fee) = 505_000
	if userSwap.OutMint != WrappedSolMint || userSwap.OutAmount.Int64() != 505_000 {
		t.Errorf("Out: got %s/%s, want wSOL/505000", userSwap.OutMint, userSwap.OutAmount)
	}
}

func TestExtractSwaps_NonFeePayerSingleMintSkipped(t *testing.T) {
	// fee_payer 是 addrB, addrA 只出现单侧转账
	pre := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "100", 6)}
	post := []rpc.TokenBalance{mkBalance(1, addrA, mintX, "0", 6)}
	swaps := extractSwapsFromFake(fakeTx{
		sig: "sigNonFp", slot: 100, txIndex: 12, feePayer: addrB,
		pre: pre, post: post,
	})
	// addrA 虽被 classifyIsTrader 判为 trader（规则 3），
	// 但单边转账被 classifySwapDirection 跳过
	if len(swaps) != 0 {
		t.Errorf("single-side transfer by non-feePayer should be skipped, got %d swaps", len(swaps))
	}
}

// TestResolvedAccountKeys_V0WithLoadedAddresses 验证 v0 tx 的账户键解析顺序:
// static ++ writable loaded ++ readonly loaded。顺序错了会让 TokenBalance.AccountIndex
// 索引到错误的 owner,导致 extractLamportFlows / flows 的 owner 错位。
func TestResolvedAccountKeys_V0WithLoadedAddresses(t *testing.T) {
	staticA := solana.MustPublicKeyFromBase58(addrA)
	staticB := solana.MustPublicKeyFromBase58(addrB)
	writableX := solana.MustPublicKeyFromBase58(mintX)
	readonlyY := solana.MustPublicKeyFromBase58(mintY)

	ptx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: solana.PublicKeySlice{staticA, staticB},
		},
	}
	meta := &rpc.TransactionMeta{
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: solana.PublicKeySlice{writableX},
			ReadOnly: solana.PublicKeySlice{readonlyY},
		},
	}

	keys := resolvedAccountKeysAsStrings(ptx, meta)

	want := []string{addrA, addrB, mintX, mintY}
	if len(keys) != len(want) {
		t.Fatalf("len: got %d, want %d; keys=%v", len(keys), len(want), keys)
	}
	for i, w := range want {
		if keys[i] != w {
			t.Errorf("keys[%d]: got %s, want %s", i, keys[i], w)
		}
	}

	// 无 LoadedAddresses 时只应返回 static keys (legacy tx 场景)。
	noLoaded := &rpc.TransactionMeta{}
	keys2 := resolvedAccountKeysAsStrings(ptx, noLoaded)
	if len(keys2) != 2 || keys2[0] != addrA || keys2[1] != addrB {
		t.Errorf("legacy: got %v, want [%s %s]", keys2, addrA, addrB)
	}

	// meta=nil 不应 panic,回落到仅 static。
	keys3 := resolvedAccountKeysAsStrings(ptx, nil)
	if len(keys3) != 2 {
		t.Errorf("nil meta: got len %d, want 2", len(keys3))
	}
}
