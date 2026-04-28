package ethereum

import "github.com/naiba/nb/model"

// benchMatcher returns a matcher with a pattern that never matches, so the full
// hot-path runs every iteration without early-exit noise.
func benchMatcher() *model.VanityMatcher {
	cfg := &model.VanityConfig{
		Contains:      "zzzzzz",
		Mode:          model.VanityModePrefixOrSuffix,
		CaseSensitive: false,
		UpperOrLower:  false,
	}
	return model.NewVanityMatcher(cfg)
}
