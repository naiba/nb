package sandwich

import (
	"math/big"
	"testing"
)

// mkSwap 测试辅助
func mkSwap(sig string, txIdx int, feePayer, owner, inMint, outMint string,
	inAmt, outAmt int64, isTrader bool) Swap {
	return Swap{
		Signature: sig,
		Slot:      100,
		TxIndex:   txIdx,
		FeePayer:  feePayer,
		Owner:     owner,
		IsTrader:  isTrader,
		InMint:    inMint,
		OutMint:   outMint,
		InAmount:  big.NewInt(inAmt),
		OutAmount: big.NewInt(outAmt),
	}
}

const (
	userAddr = "11111111111111111111111111111121"
	poolA    = "11111111111111111111111111111131"
	poolB    = "11111111111111111111111111111132"
	botAddr  = "11111111111111111111111111111141"
)

// userMint 复用 extract_test.go 中的 mintX（同 package 可见）
func userMintValue() string { return mintX }

func TestDetect_CleanWhenNoOthers(t *testing.T) {
	userSwap := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	poolSwap := mkSwap("sigUser", 50, userAddr, poolA, mintY, userMintValue(), 10, 1000, false)

	v := Detect("sigUser", userAddr, userMintValue(), []Swap{userSwap, poolSwap})
	if v.Level != NotSandwiched {
		t.Errorf("expected NotSandwiched, got %s", v.Level)
	}
}

func TestDetect_Sandwiched_AllConditionsHit(t *testing.T) {
	// A+B+C+D 全中：同一 bot 前后反向 + 共享 poolA + user 同方向
	userSwap := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	userPool := mkSwap("sigUser", 50, userAddr, poolA, mintY, userMintValue(), 10, 1000, false)

	// front: bot 同方向（卖 mintX 买 mintY）via poolA
	frontTrader := mkSwap("sigFront", 48, botAddr, botAddr, userMintValue(), mintY, 500, 6, true)
	frontPool := mkSwap("sigFront", 48, botAddr, poolA, mintY, userMintValue(), 6, 500, false)

	// back: bot 反方向（买 mintX 卖 mintY）via poolA
	backTrader := mkSwap("sigBack", 55, botAddr, botAddr, mintY, userMintValue(), 8, 700, true)
	backPool := mkSwap("sigBack", 55, botAddr, poolA, userMintValue(), mintY, 700, 8, false)

	swaps := []Swap{frontTrader, frontPool, userSwap, userPool, backTrader, backPool}
	v := Detect("sigUser", userAddr, userMintValue(), swaps)
	if v.Level != Sandwiched {
		t.Fatalf("expected Sandwiched, got %s", v.Level)
	}
	if v.Attacker != botAddr {
		t.Errorf("attacker: got %s, want %s", v.Attacker, botAddr)
	}
	if v.FrontRun == nil || v.BackRun == nil {
		t.Errorf("FrontRun/BackRun should be set")
	}
}

func TestDetect_DifferentPool_NotSandwiched(t *testing.T) {
	// A+B+C 中但 D 不中：attacker 用 poolB 而 user 用 poolA
	// D 为 gating 条件：attacker 在别的池做自己的买卖，与 user 无关，不算夹子。
	userSwap := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	userPool := mkSwap("sigUser", 50, userAddr, poolA, mintY, userMintValue(), 10, 1000, false)

	frontTrader := mkSwap("sigFront", 48, botAddr, botAddr, userMintValue(), mintY, 500, 6, true)
	frontPool := mkSwap("sigFront", 48, botAddr, poolB, mintY, userMintValue(), 6, 500, false)

	backTrader := mkSwap("sigBack", 55, botAddr, botAddr, mintY, userMintValue(), 8, 700, true)
	backPool := mkSwap("sigBack", 55, botAddr, poolB, userMintValue(), mintY, 700, 8, false)

	swaps := []Swap{frontTrader, frontPool, userSwap, userPool, backTrader, backPool}
	v := Detect("sigUser", userAddr, userMintValue(), swaps)
	if v.Level != NotSandwiched {
		t.Errorf("expected NotSandwiched (attacker on different pool), got %s", v.Level)
	}
}

func TestDetect_Sandwiched_MatchByFeePayer(t *testing.T) {
	// 不同 owner，但 fee_payer 相同也应匹配（条件 B 第二分支）
	userSwap := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	userPool := mkSwap("sigUser", 50, userAddr, poolA, mintY, userMintValue(), 10, 1000, false)

	otherOwner := "11111111111111111111111111111151"
	frontTrader := mkSwap("sigFront", 48, botAddr, otherOwner, userMintValue(), mintY, 500, 6, true)
	frontPool := mkSwap("sigFront", 48, botAddr, poolA, mintY, userMintValue(), 6, 500, false)

	backTrader := mkSwap("sigBack", 55, botAddr, botAddr, mintY, userMintValue(), 8, 700, true)
	backPool := mkSwap("sigBack", 55, botAddr, poolA, userMintValue(), mintY, 700, 8, false)

	swaps := []Swap{frontTrader, frontPool, userSwap, userPool, backTrader, backPool}
	v := Detect("sigUser", userAddr, userMintValue(), swaps)
	if v.Level != Sandwiched {
		t.Fatalf("expected Sandwiched (match by FeePayer), got %s", v.Level)
	}
}

func TestEstimateLoss_ConstantProduct(t *testing.T) {
	// pool 前状态: x=10000 mintX, y=100 mintY
	// user 用 1000 mintX 买 mintY, 无 front 时理论 dy = preY - preX*preY/(preX+userIn)
	//                                              = 100 - 10000*100/11000 = 100 - 909090/10000
	//   整数运算: 1000000 / 11000 = 90; idealOut = 100 - 90 = 10
	//   actual = 8 → loss = 2
	preX := big.NewInt(10000)
	preY := big.NewInt(100)
	userIn := big.NewInt(1000)
	userActualOut := big.NewInt(8)

	loss := estimateLoss(preX, preY, userIn, userActualOut)
	if loss == nil {
		t.Fatal("expected non-nil loss")
	}
	if loss.Int64() != 2 {
		t.Errorf("loss: got %d, want 2", loss.Int64())
	}
}

func TestEstimateLoss_NegativeReturnsNil(t *testing.T) {
	// actual > ideal → 返回 nil
	preX := big.NewInt(10000)
	preY := big.NewInt(100)
	userIn := big.NewInt(1000)
	userActualOut := big.NewInt(50)

	loss := estimateLoss(preX, preY, userIn, userActualOut)
	if loss != nil {
		t.Errorf("expected nil (actual > ideal), got %s", loss)
	}
}

func TestEstimateLoss_NilOrZeroInputs(t *testing.T) {
	// 任意 nil 或非正数输入返回 nil
	cases := []struct {
		name               string
		preX, preY, in, out *big.Int
	}{
		{"nil preX", nil, big.NewInt(100), big.NewInt(10), big.NewInt(5)},
		{"zero preX", big.NewInt(0), big.NewInt(100), big.NewInt(10), big.NewInt(5)},
		{"zero userIn", big.NewInt(100), big.NewInt(100), big.NewInt(0), big.NewInt(5)},
	}
	for _, c := range cases {
		if got := estimateLoss(c.preX, c.preY, c.in, c.out); got != nil {
			t.Errorf("%s: expected nil, got %s", c.name, got)
		}
	}
}

// TestDetect_RealCase_4qEwXsCw 用真实 slot 416279211-416279213 的关键 swap 作 fixture。
//
// 数据来源：真实 tx 4qEwXsCw9o2ooFh9ByWeMSUjV9VAyD6L3eGoKWJRAXVxcW7gUVNUo1pLsod89jDJpGgVgihJrKQtXXTGx3dLqEv5
// 关键观察：attacker 用 HWpBChLL（pump.fun bonding curve），user 用 HLnpSz9h（Raydium），
// 两池完全不同，不构成夹击。期望 Verdict=NotSandwiched。
func TestDetect_RealCase_4qEwXsCw(t *testing.T) {
	const (
		userWallet   = "PupGDp3HgcHUxC22HhiDL1s7UgYKcbf8YLE9tvXnk7f"
		attacker     = "DPwH9aWrYGSBqMiPNLMW4Hti1k1xgq64t781eycfLCp5"
		tokenMint    = "B3bJNmEdfMNGMf4D4P8GErzkbkdmuz1KFmn92iAhRCgC"
		wsol         = "So11111111111111111111111111111111111111112"
		userPoolA    = "HLnpSz9hxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx11"
		attackerPool = "HWpBChLLxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx1"
	)

	// Front-run: attacker 在 user 前一 slot 同方向卖 mintX → wSOL via attackerPool
	frontTrader := Swap{
		Signature: "3CHPVrxnRaPcixoqD3EpimRx6neWAxA8xtYxdM3SydX7MXccVPB3ZPKpUuVK8EL6NELHUE3QYrzxMvVuAip3nfwo",
		Slot:      416279211, TxIndex: 51, FeePayer: attacker,
		Owner: attacker, IsTrader: true,
		InMint: tokenMint, OutMint: wsol,
		InAmount: big.NewInt(497202364975), OutAmount: big.NewInt(1748977120),
	}
	frontPool := Swap{
		Signature: frontTrader.Signature,
		Slot:      416279211, TxIndex: 51, FeePayer: attacker,
		Owner: attackerPool, IsTrader: false,
		InMint: wsol, OutMint: tokenMint,
		InAmount: big.NewInt(1748977120), OutAmount: big.NewInt(497202364975),
	}

	// User: 卖 mintX → wSOL via userPoolA
	userTrader := Swap{
		Signature: "4qEwXsCw9o2ooFh9ByWeMSUjV9VAyD6L3eGoKWJRAXVxcW7gUVNUo1pLsod89jDJpGgVgihJrKQtXXTGx3dLqEv5",
		Slot:      416279212, TxIndex: 1389, FeePayer: userWallet,
		Owner: userWallet, IsTrader: true,
		InMint: tokenMint, OutMint: wsol,
		InAmount: big.NewInt(643026862062), OutAmount: big.NewInt(1122852696),
	}
	userPool := Swap{
		Signature: userTrader.Signature,
		Slot:      416279212, TxIndex: 1389, FeePayer: userWallet,
		Owner: userPoolA, IsTrader: false,
		InMint: wsol, OutMint: tokenMint,
		InAmount: big.NewInt(1122852696), OutAmount: big.NewInt(643026862062),
	}

	// Back-run: attacker 在 user 后一 slot 反方向买 mintX via attackerPool
	backTrader := Swap{
		Signature: "3uB8Q3T6L76XXxS1ZjHhuZ17z5sg2BqPvoZsTT6cJoRdiMMFFaFC5jznviCfTjfgrmKUiPup9CAkfqXbFhJ5PRUx",
		Slot:      416279213, TxIndex: 84, FeePayer: attacker,
		Owner: attacker, IsTrader: true,
		InMint: wsol, OutMint: tokenMint,
		InAmount: big.NewInt(2000000000), OutAmount: big.NewInt(556182404736),
	}
	backPool := Swap{
		Signature: backTrader.Signature,
		Slot:      416279213, TxIndex: 84, FeePayer: attacker,
		Owner: attackerPool, IsTrader: false,
		InMint: tokenMint, OutMint: wsol,
		InAmount: big.NewInt(556182404736), OutAmount: big.NewInt(2000000000),
	}

	swaps := []Swap{frontTrader, frontPool, userTrader, userPool, backTrader, backPool}
	v := Detect(userTrader.Signature, userWallet, tokenMint, swaps)

	if v.Level != NotSandwiched {
		t.Fatalf("expected NotSandwiched (attacker uses different pool HWpBChLL vs user HLnpSz9h), got %s (reasons: %v)", v.Level, v.Reasons)
	}
	if v.FrontRun != nil || v.BackRun != nil {
		t.Errorf("FrontRun/BackRun should be nil for non-Sandwiched")
	}
}

func TestDetect_RelatedPoolTxsCollected(t *testing.T) {
	// 构造：user 在 poolA 卖，其他人 arb 也走 poolA，应该被 RelatedPoolTxs 捕获
	userSwap := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	userPool := mkSwap("sigUser", 50, userAddr, poolA, mintY, userMintValue(), 10, 1000, false)

	// arb 在 user 后用 poolA
	arbTrader := mkSwap("sigArb", 55, botAddr, botAddr, mintY, userMintValue(), 5, 450, true)
	arbPool := mkSwap("sigArb", 55, botAddr, poolA, userMintValue(), mintY, 450, 5, false)

	// 无关交易在 poolB，不应该被收集
	noiseTrader := mkSwap("sigNoise", 60, "wallet3", "wallet3", userMintValue(), mintY, 100, 1, true)
	noisePool := mkSwap("sigNoise", 60, "wallet3", poolB, mintY, userMintValue(), 1, 100, false)

	swaps := []Swap{userSwap, userPool, arbTrader, arbPool, noiseTrader, noisePool}
	v := Detect("sigUser", userAddr, userMintValue(), swaps)

	// Expected 2: user 自己 tx 的 poolA 作为 "user's pool" 前置，加上后面 arb 在 poolA 的一条
	if len(v.RelatedPoolTxs) != 2 {
		t.Fatalf("expected 2 RelatedPoolTxs (user pool + arb), got %d: %+v", len(v.RelatedPoolTxs), v.RelatedPoolTxs)
	}
	if v.RelatedPoolTxs[0].Signature != "sigUser" || v.RelatedPoolTxs[0].Owner != poolA {
		t.Errorf("expected first entry to be user's poolA, got sig=%s owner=%s",
			v.RelatedPoolTxs[0].Signature, v.RelatedPoolTxs[0].Owner)
	}
	if v.RelatedPoolTxs[1].Signature != "sigArb" {
		t.Errorf("expected arb in RelatedPoolTxs[1], got %s", v.RelatedPoolTxs[1].Signature)
	}
}

func TestDetect_RelatedPoolTxs_IncludesArbWithoutTraderSwap(t *testing.T) {
	// arb bot 的 tx 里只有 pool swaps（trader 层被 extract 跳过），
	// 但如果某个 pool 是 user 的对手池，依然要列出。
	userSwap := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	userPool := mkSwap("sigUser", 50, userAddr, poolA, mintY, userMintValue(), 10, 1000, false)

	// arb bot 的 tx：没有 trader swap（被 extract 跳过），只有两个 pool swap
	arbPoolA := Swap{
		Signature: "sigArb", Slot: 100, TxIndex: 55,
		FeePayer: botAddr, Owner: poolA, IsTrader: false,
		InMint: userMintValue(), OutMint: mintY,
		InAmount: big.NewInt(500), OutAmount: big.NewInt(5),
	}
	arbPoolB := Swap{
		Signature: "sigArb", Slot: 100, TxIndex: 55,
		FeePayer: botAddr, Owner: poolB, IsTrader: false,
		InMint: mintY, OutMint: userMintValue(),
		InAmount: big.NewInt(5), OutAmount: big.NewInt(500),
	}

	swaps := []Swap{userSwap, userPool, arbPoolA, arbPoolB}
	v := Detect("sigUser", userAddr, userMintValue(), swaps)

	// Expected 2: user 自己 tx 的 poolA 前置 + arb 在 poolA 的 pool swap
	if len(v.RelatedPoolTxs) != 2 {
		t.Fatalf("expected 2 RelatedPoolTxs (user's pool + arb), got %d", len(v.RelatedPoolTxs))
	}
	if v.RelatedPoolTxs[0].Signature != "sigUser" {
		t.Errorf("expected user's own pool first, got %s", v.RelatedPoolTxs[0].Signature)
	}
	arb := v.RelatedPoolTxs[1]
	if arb.Signature != "sigArb" {
		t.Errorf("expected sigArb, got %s", arb.Signature)
	}
	// 代表是 pool 视角（因为没 trader swap），Owner 应是 poolA（user 的池）
	if arb.Owner != poolA {
		t.Errorf("representative should be poolA (user's pool), got %s", arb.Owner)
	}
	if arb.IsTrader {
		t.Errorf("representative should be pool swap (IsTrader=false)")
	}
}
