package sandwich

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// TestFormatDetailed_HotMint_Truncation 模拟热门 mint 场景:
// 30 fronts × 30 backs = 900 对候选都在非 user 池 (D fail),外加 50 笔同池 related。
// 验证:
//   - Related transactions 截断到 relatedLimit 条 + 展示总数
//   - Detection pair detail 截断到 pairDetailLimit 条 + 聚合 B/D/E fail 计数
//   - 输出行数有合理上限 (不再是上千条)
func TestFormatDetailed_HotMint_Truncation(t *testing.T) {
	const (
		userW     = "11111111111111111111111111111121"
		userPool  = "11111111111111111111111111111131"
		otherPool = "11111111111111111111111111111132"
		mintX     = "11111111111111111111111111111114"
		mintY     = "11111111111111111111111111111115"
	)

	mk := func(sig string, slot uint64, tx int, fp, owner, inM, outM string, in, out int64, trader bool) Swap {
		return Swap{
			Signature: sig, Slot: slot, TxIndex: tx, FeePayer: fp,
			Owner: owner, IsTrader: trader,
			InMint: inM, OutMint: outM,
			InAmount: big.NewInt(in), OutAmount: big.NewInt(out),
		}
	}

	// user: 在 userPool 卖 mintX → mintY
	userTrader := mk("sigUser", 100, 500, userW, userW, mintX, mintY, 1000, 10, true)
	userP := mk("sigUser", 100, 500, userW, userPool, mintY, mintX, 10, 1000, false)

	swaps := []Swap{userTrader, userP}
	flows := map[string][]Swap{"sigUser": {userTrader, userP}}

	// 30 fronts: 都在 otherPool, 和 user 同方向, 不同 bot (不同 owner),金额在容差内。
	for i := 0; i < 30; i++ {
		bot := fmt.Sprintf("bot1111111111111111111111111111%03d", i)
		sig := fmt.Sprintf("sigF%d", i)
		fTrader := mk(sig, 100, 100+i, bot, bot, mintX, mintY, 500, 6, true)
		fPool := mk(sig, 100, 100+i, bot, otherPool, mintY, mintX, 6, 500, false)
		swaps = append(swaps, fTrader, fPool)
	}
	// 30 backs: 都在 otherPool, 反方向。
	for i := 0; i < 30; i++ {
		bot := fmt.Sprintf("bot1111111111111111111111111111%03d", i)
		sig := fmt.Sprintf("sigB%d", i)
		bTrader := mk(sig, 100, 600+i, bot, bot, mintY, mintX, 8, 700, true)
		bPool := mk(sig, 100, 600+i, bot, otherPool, mintX, mintY, 700, 8, false)
		swaps = append(swaps, bTrader, bPool)
	}
	// 50 笔真正同 user 池的 arb tx (RelatedPoolTxs 应该吃到它们)
	for i := 0; i < 50; i++ {
		bot := fmt.Sprintf("arb1111111111111111111111111111%03d", i)
		sig := fmt.Sprintf("sigArb%d", i)
		aTrader := mk(sig, 100, 800+i, bot, bot, mintY, mintX, 5, 450, true)
		aPool := mk(sig, 100, 800+i, bot, userPool, mintX, mintY, 450, 5, false)
		swaps = append(swaps, aTrader, aPool)
		flows[sig] = []Swap{aTrader, aPool}
	}

	r := DetectDetailed(userTrader.Signature, userW, mintX, swaps)
	r.TxFlows = flows
	// relatedSigs 就按 RelatedPoolTxs 的 signature 集合 (实际 Analyze 是这样做的)
	sigOrder := []string{userTrader.Signature}
	seen := map[string]bool{userTrader.Signature: true}
	for _, s := range r.Verdict.RelatedPoolTxs {
		if !seen[s.Signature] {
			sigOrder = append(sigOrder, s.Signature)
			seen[s.Signature] = true
		}
	}
	r.RelatedSigs = sigOrder

	out := FormatDetailed(r)
	if testing.Verbose() {
		fmt.Println(out)
	}

	// 核心断言: attacker-centric 下 30 个 bot 各有 front+back, 都在 otherPool
	// → 全部归类为 otherPoolOnly,以一行统计收尾
	if !strings.Contains(out, "attackers (same trader with both front+back): 30") {
		t.Errorf("expected 30 attackers; output snippet: %s", snippet(out, "Detection"))
	}
	if !strings.Contains(out, "30 trader(s) had frontrun+backrun but on pools OUTSIDE yours") {
		t.Errorf("expected other-pool aggregation line; output snippet: %s",
			snippet(out, "Detection"))
	}
	// Related 段截断 (50 笔 → 20 + "30 more hidden")
	if !strings.Contains(out, "more hidden") {
		t.Errorf("Related transactions not truncated; output snippet: %s",
			snippet(out, "Related transactions"))
	}

	// 行数上限: relatedLimit=20 × ~10 行 (完整 tx block: header + trader + pool) + 杂项 ≈ 250 行。
	// 重构后 30 个 otherPool attacker 只产出 1 行聚合, 不再是 2400 对笛卡尔积。
	// 核心保证: 不爆炸到上千行级别。
	lines := strings.Count(out, "\n")
	if lines > 300 {
		t.Errorf("output too long (%d lines), truncation might be broken", lines)
	}
}

func snippet(s, anchor string) string {
	i := strings.Index(s, anchor)
	if i < 0 {
		return "(anchor not found)"
	}
	end := i + 400
	if end > len(s) {
		end = len(s)
	}
	return s[i:end]
}

// TestSandwich_ClassicVsAtypical 验证 classic (front/user/back 同 slot) 与
// atypical (跨 slot) 的区分标签。
func TestSandwich_ClassicVsAtypical(t *testing.T) {
	const (
		userW    = "11111111111111111111111111111121"
		userPool = "11111111111111111111111111111131"
		botA     = "11111111111111111111111111111141" // classic 同 slot bot
		botB     = "11111111111111111111111111111151" // atypical 跨 slot bot
		mintX    = "11111111111111111111111111111114"
		mintY    = "11111111111111111111111111111115"
	)
	mk := func(sig string, slot uint64, tx int, fp, owner, inM, outM string, in, out int64, trader bool) Swap {
		return Swap{
			Signature: sig, Slot: slot, TxIndex: tx, FeePayer: fp,
			Owner: owner, IsTrader: trader,
			InMint: inM, OutMint: outM,
			InAmount: big.NewInt(in), OutAmount: big.NewInt(out),
		}
	}

	// user 在 slot 100 tx 50
	userT := mk("sigUser", 100, 50, userW, userW, mintX, mintY, 1000, 10, true)
	userP := mk("sigUser", 100, 50, userW, userPool, mintY, mintX, 10, 1000, false)
	// botA: classic, front+user+back 全 slot 100
	aF := mk("sigAF", 100, 48, botA, botA, mintX, mintY, 500, 6, true)
	aFp := mk("sigAF", 100, 48, botA, userPool, mintY, mintX, 6, 500, false)
	aB := mk("sigAB", 100, 55, botA, botA, mintY, mintX, 8, 700, true)
	aBp := mk("sigAB", 100, 55, botA, userPool, mintX, mintY, 700, 8, false)
	// botB: atypical, front 在 slot 98, back 在 slot 102
	bF := mk("sigBF", 98, 20, botB, botB, mintX, mintY, 500, 6, true)
	bFp := mk("sigBF", 98, 20, botB, userPool, mintY, mintX, 6, 500, false)
	bB := mk("sigBB", 102, 30, botB, botB, mintY, mintX, 8, 700, true)
	bBp := mk("sigBB", 102, 30, botB, userPool, mintX, mintY, 700, 8, false)

	swaps := []Swap{userT, userP, aF, aFp, aB, aBp, bF, bFp, bB, bBp}
	r := DetectDetailed("sigUser", userW, mintX, swaps)

	// 两个 attacker 都应被 IsSandwich=true (同池 + 金额闭合)
	var gotClassic, gotAtypical bool
	for _, a := range r.Attackers {
		if !a.IsSandwich {
			continue
		}
		if a.IsClassic {
			gotClassic = true
			if a.AttackerKey != botA {
				t.Errorf("classic attacker expected %s, got %s", botA, a.AttackerKey)
			}
		} else {
			gotAtypical = true
			if a.AttackerKey != botB {
				t.Errorf("atypical attacker expected %s, got %s", botB, a.AttackerKey)
			}
		}
	}
	if !gotClassic {
		t.Errorf("expected one classic sandwich attacker (botA)")
	}
	if !gotAtypical {
		t.Errorf("expected one atypical sandwich attacker (botB)")
	}

	// Verdict 应挑 classic 作为代表 (排序优先 IsClassic)
	if r.Verdict.Level != Sandwiched || r.Verdict.Attacker != botA {
		t.Errorf("Verdict should pick classic botA, got level=%s attacker=%s",
			r.Verdict.Level, r.Verdict.Attacker)
	}

	// Format 里应有两种 tag
	r.TxFlows = map[string][]Swap{
		"sigAF": {aF, aFp}, "sigUser": {userT, userP}, "sigAB": {aB, aBp},
		"sigBF": {bF, bFp}, "sigBB": {bB, bBp},
	}
	r.RelatedSigs = []string{"sigBF", "sigAF", "sigUser", "sigAB", "sigBB"}
	out := FormatDetailed(r)
	if testing.Verbose() {
		fmt.Println(out)
	}
	if !strings.Contains(out, "[⚠ SANDWICH (classic") {
		t.Errorf("missing classic tag\n%s", out)
	}
	if !strings.Contains(out, "[⚠ SANDWICH (atypical") {
		t.Errorf("missing atypical tag\n%s", out)
	}
}

// TestClassifyRole_SelfVsOther 验证 fp 与 user 相同的其他 tx 标 [self],不是 [other]。
// 典型场景: user 用 Jupiter 几秒内连续 swap, 多笔同 fp 的 tx 都走同一池。
func TestClassifyRole_SelfVsOther(t *testing.T) {
	const (
		userW    = "11111111111111111111111111111121"
		otherW   = "11111111111111111111111111111141"
		userPool = "11111111111111111111111111111131"
		mintX    = "11111111111111111111111111111114"
		mintY    = "11111111111111111111111111111115"
	)

	userT := Swap{Signature: "sigUser", Slot: 100, TxIndex: 50, FeePayer: userW, Owner: userW, IsTrader: true, InMint: mintX, OutMint: mintY, InAmount: big.NewInt(1000), OutAmount: big.NewInt(10)}
	userP := Swap{Signature: "sigUser", Slot: 100, TxIndex: 50, FeePayer: userW, Owner: userPool, IsTrader: false, InMint: mintY, OutMint: mintX, InAmount: big.NewInt(10), OutAmount: big.NewInt(1000)}
	// user 本人的下一笔单 (同 fp)
	selfT := Swap{Signature: "sigSelf", Slot: 100, TxIndex: 52, FeePayer: userW, Owner: userW, IsTrader: true, InMint: mintX, OutMint: mintY, InAmount: big.NewInt(100), OutAmount: big.NewInt(1)}
	selfP := Swap{Signature: "sigSelf", Slot: 100, TxIndex: 52, FeePayer: userW, Owner: userPool, IsTrader: false, InMint: mintY, OutMint: mintX, InAmount: big.NewInt(1), OutAmount: big.NewInt(100)}
	// 真 · 其他人的单 (不同 fp)
	otherT := Swap{Signature: "sigOther", Slot: 100, TxIndex: 55, FeePayer: otherW, Owner: otherW, IsTrader: true, InMint: mintX, OutMint: mintY, InAmount: big.NewInt(50), OutAmount: big.NewInt(1)}
	otherP := Swap{Signature: "sigOther", Slot: 100, TxIndex: 55, FeePayer: otherW, Owner: userPool, IsTrader: false, InMint: mintY, OutMint: mintX, InAmount: big.NewInt(1), OutAmount: big.NewInt(50)}

	r := DetectDetailed("sigUser", userW, mintX, []Swap{userT, userP, selfT, selfP, otherT, otherP})
	r.TxFlows = map[string][]Swap{
		"sigUser":  {userT, userP},
		"sigSelf":  {selfT, selfP},
		"sigOther": {otherT, otherP},
	}
	r.RelatedSigs = []string{"sigUser", "sigSelf", "sigOther"}

	out := FormatDetailed(r)
	if testing.Verbose() {
		fmt.Println(out)
	}

	want := map[string]string{
		"sigUser":  "[YOU]",
		"sigSelf":  "[self]",
		"sigOther": "[other]",
	}
	for sig, label := range want {
		// 粗略断言: 输出里 sig 前的那行应含 label
		if !strings.Contains(out, label) {
			t.Errorf("expected %s for %s in output\n----\n%s", label, sig, out)
		}
	}
}

// TestFormatDetailed_Sandwiched_Profile 命中场景: 同一 bot 在 user 池上 front+back 金额闭合。
// 输出应展示 attacker profile(聚合金额 + 共享池 + 粗略利润)。
func TestFormatDetailed_Sandwiched_Profile(t *testing.T) {
	const (
		userW    = "11111111111111111111111111111121"
		userPool = "11111111111111111111111111111131"
		bot      = "11111111111111111111111111111141"
		mintX    = "11111111111111111111111111111114"
		mintY    = "11111111111111111111111111111115"
	)
	mk := func(sig string, tx int, fp, owner, inM, outM string, in, out int64, trader bool) Swap {
		return Swap{
			Signature: sig, Slot: 100, TxIndex: tx, FeePayer: fp,
			Owner: owner, IsTrader: trader,
			InMint: inM, OutMint: outM,
			InAmount: big.NewInt(in), OutAmount: big.NewInt(out),
		}
	}

	userT := mk("sigUser", 50, userW, userW, mintX, mintY, 1000, 10, true)
	userP := mk("sigUser", 50, userW, userPool, mintY, mintX, 10, 1000, false)
	frontT := mk("sigFront", 48, bot, bot, mintX, mintY, 500, 6, true)
	frontP := mk("sigFront", 48, bot, userPool, mintY, mintX, 6, 500, false)
	backT := mk("sigBack", 55, bot, bot, mintY, mintX, 8, 700, true)
	backP := mk("sigBack", 55, bot, userPool, mintX, mintY, 700, 8, false)

	swaps := []Swap{frontT, frontP, userT, userP, backT, backP}
	r := DetectDetailed("sigUser", userW, mintX, swaps)
	r.TxFlows = map[string][]Swap{
		"sigFront": {frontT, frontP},
		"sigUser":  {userT, userP},
		"sigBack":  {backT, backP},
	}
	r.RelatedSigs = []string{"sigFront", "sigUser", "sigBack"}

	out := FormatDetailed(r)
	if testing.Verbose() {
		fmt.Println(out)
	}

	mustContain := []string{
		"Verdict: SANDWICHED",
		// fixture 里 front/user/back 都在 slot=100, 属于 classic 同 slot bundle
		"[⚠ SANDWICH (classic",
		"attacker (via owner):",
		"frontrun (1 tx):",
		"backrun (1 tx):",
		"shared pool with you:",
		"closure:",
		"net profit",
		// user / front / back 都在时间线里
		"[YOU]",
		"[FRONT]",
		"[BACK]",
		"pool [yours]",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q\n----\n%s", s, out)
		}
	}
}

// TestFormatDetailed_Smoke_RealCase 跑 4qEwXsCw 的 fixture,断言关键片段存在,
// 同时当带 -v 时能看到完整输出供人工核验。
func TestFormatDetailed_Smoke_RealCase(t *testing.T) {
	const (
		userWallet   = "PupGDp3HgcHUxC22HhiDL1s7UgYKcbf8YLE9tvXnk7f"
		attacker     = "DPwH9aWrYGSBqMiPNLMW4Hti1k1xgq64t781eycfLCp5"
		tokenMint    = "B3bJNmEdfMNGMf4D4P8GErzkbkdmuz1KFmn92iAhRCgC"
		wsol         = "So11111111111111111111111111111111111111112"
		userPoolA    = "HLnpSz9hxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx11"
		attackerPool = "HWpBChLLxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx1"
	)

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
	r := DetectDetailed(userTrader.Signature, userWallet, tokenMint, swaps)
	// 模拟 Analyze 里的 buildTxFlows: 只放 user + RelatedPoolTxs。此 fixture 中
	// attacker 两笔都在自己独立池 (HWpBChLL),不在 user 池 (HLnpSz9h),所以 RelatedPoolTxs
	// 只包含 user 自己 tx 的 user pool swap;attacker 的两笔 tx 不出现在资金流段。
	r.TxFlows = map[string][]Swap{
		userTrader.Signature: {userTrader, userPool},
	}
	r.RelatedSigs = []string{userTrader.Signature}

	out := FormatDetailed(r)
	if testing.Verbose() {
		fmt.Println(out)
	}

	// 关键断言: attacker-centric 输出
	mustContain := []string{
		"NOT_SANDWICHED",
		// user 自己现在是 "Related transactions" 段的 [YOU]
		"Related transactions on your pools (1, incl. yours)",
		"[YOU]",
		// 完整地址 / mint
		"PupGDp3HgcHUxC22HhiDL1s7UgYKcbf8YLE9tvXnk7f",
		"B3bJNmEdfMNGMf4D4P8GErzkbkdmuz1KFmn92iAhRCgC",
		"So11111111111111111111111111111111111111112",
		"HLnpSz9hxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx11",
		// 完整 sig
		"4qEwXsCw9o2ooFh9ByWeMSUjV9VAyD6L3eGoKWJRAXVxcW7gUVNUo1pLsod89jDJpGgVgihJrKQtXXTGx3dLqEv5",
		"pool [yours]",
		"━━ Detection ━━",
		"candidate frontrun swaps",
		"candidate backrun swaps",
		"attackers (same trader with both front+back): 1",
		"pools OUTSIDE yours",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q\n----\n%s", s, out)
		}
	}

	// 反向断言: attacker 的具体金额不应出现(他在你池外, 被聚合成一行统计)
	for _, unwanted := range []string{
		"1748977120",   // front 的 wSOL 金额
		"497202364975", // front 的 mintX 金额
		"556182404736", // back 的 mintX 金额
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("attacker amount %q should NOT appear (attacker on other pool, should only be counted)\n----\n%s", unwanted, out)
		}
	}
}
