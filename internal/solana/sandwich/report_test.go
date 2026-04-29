package sandwich

import (
	"math/big"
	"strings"
	"testing"
)

func TestFormat_NotSandwichedOutput(t *testing.T) {
	v := Verdict{
		Level: NotSandwiched,
		UserSwap: Swap{
			Signature: "sigUser",
			InMint:    mintY,
			OutMint:   userMintValue(),
			InAmount:  big.NewInt(10),
			OutAmount: big.NewInt(1000),
		},
		Reasons: []string{"no related activity found"},
	}
	out := Format(v)
	if !strings.Contains(out, "NOT_SANDWICHED") {
		t.Errorf("output missing NOT_SANDWICHED level:\n%s", out)
	}
	if !strings.Contains(out, "no related activity found") {
		t.Errorf("output missing reason:\n%s", out)
	}
}

func TestFormat_SandwichedIncludesAttackerAndPair(t *testing.T) {
	front := mkSwap("sigFront", 48, botAddr, botAddr, userMintValue(), mintY, 500, 6, true)
	back := mkSwap("sigBack", 55, botAddr, botAddr, mintY, userMintValue(), 8, 700, true)
	user := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	v := Verdict{
		Level:    Sandwiched,
		UserSwap: user,
		FrontRun: &front,
		BackRun:  &back,
		Attacker: botAddr,
		Reasons:  []string{"same-pool front+back by bot"},
	}
	out := Format(v)

	for _, want := range []string{"SANDWICHED", botAddr, "sigFront", "sigBack"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFormat_RelatedPoolTxsSection(t *testing.T) {
	related := mkSwap("sigArb", 55, botAddr, botAddr, mintY, userMintValue(), 5, 450, true)
	user := mkSwap("sigUser", 50, userAddr, userAddr, userMintValue(), mintY, 1000, 10, true)
	v := Verdict{
		Level:          NotSandwiched,
		UserSwap:       user,
		Reasons:        []string{"no sandwich"},
		RelatedPoolTxs: []Swap{related},
	}
	out := Format(v)
	for _, want := range []string{"Swaps on user's pools", "sigArb", botAddr, "100#55", "[other]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}
