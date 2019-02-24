package cfr

import (
	"math"
)

// Params are the configuration options for CFR sampling
// and regret matching. An empty Params struct is valid and
// corresponds to "vanilla" CFR.
type Params struct {
	SampleChanceNodes     bool    // Chance Sampling
	SamplePlayerActions   bool    // Outcome Sampling
	SampleOpponentActions bool    // External Sampling
	UseRegretMatchingPlus bool    // CFR+
	LinearWeighting       bool    // Linear CFR
	DiscountAlpha         float32 // Discounted CFR
	DiscountBeta          float32 // Discounted CFR
	DiscountGamma         float32 // Discounted CFR
}

// Gets the discount factors as configured by the parameters for the
// various CFR weighting schemes: CFR+, linear CFR, etc.
func (p Params) GetDiscountFactors(iter int) (positive, negative, sum float32) {
	positive = float32(1.0)
	negative = float32(1.0)
	sum = float32(1.0)

	// See: https://arxiv.org/pdf/1809.04040.pdf
	// Linear CFR is equivalent to weighting the reach prob on each
	// iteration by (t / (t+1)), and this reduces numerical instability.
	if p.LinearWeighting {
		sum = float32(iter) / float32(iter+1)
	}

	if p.UseRegretMatchingPlus {
		negative = 0.0 // No negative regrets.
	}

	if p.DiscountAlpha != 0 {
		// t^alpha / (t^alpha + 1)
		x := float32(math.Pow(float64(iter), float64(p.DiscountAlpha)))
		positive = x / (x + 1.0)
	}

	if p.DiscountBeta != 0 {
		// t^beta / (t^beta + 1)
		x := float32(math.Pow(float64(iter), float64(p.DiscountBeta)))
		negative = x / (x + 1.0)
	}

	if p.DiscountGamma != 0 {
		// (t / (t+1)) ^ gamma
		x := float64(iter) / float64(iter+1)
		sum = float32(math.Pow(x, float64(p.DiscountGamma)))
	}

	return
}
