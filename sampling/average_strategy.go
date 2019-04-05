package sampling

import (
	"math/rand"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
	"github.com/timpalpant/go-cfr/internal/policy"
)

type AverageStrategyParams struct {
	Epsilon float32
	Tau     float32
	Beta    float32
}

// AverageStrategySampler implements cfr.Sampler by sampling some player actions
// according to the current average strategy strategy.
type AverageStrategySampler struct {
	params AverageStrategyParams
	rng    *rand.Rand
	p      []float32
}

func NewAverageStrategySampler(params AverageStrategyParams) *AverageStrategySampler {
	return &AverageStrategySampler{
		params: params,
		rng:    rand.New(rand.NewSource(rand.Int63())),
	}
}

func (as *AverageStrategySampler) Sample(node cfr.GameTreeNode, pol cfr.NodePolicy) []float32 {
	nChildren := node.NumChildren()
	as.p = extend(as.p, nChildren)

	x := as.rng.Float32()
	s := pol.(*policy.Policy).GetStrategySum()
	sSum := f32.Sum(s)
	for i := range as.p {
		rho := computeRho(s[i], sSum, as.params)
		if x < rho {
			as.p[i] = minF32(rho, 1.0)
		} else {
			as.p[i] = 0
		}
	}

	return as.p
}

func minF32(x, y float32) float32 {
	if x < y {
		return x
	}

	return y
}

func computeRho(s, sSum float32, params AverageStrategyParams) float32 {
	rho := params.Beta + params.Tau*s
	rho /= params.Beta + sSum
	if rho < params.Epsilon {
		return params.Epsilon
	}

	return rho
}
