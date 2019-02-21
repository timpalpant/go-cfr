package cfr

import (
	"github.com/timpalpant/go-cfr/internal/f32"
)

type policy struct {
	reachProb   float32
	regretSum   []float32
	strategy    []float32
	strategySum []float32
}

func newPolicy(nActions int) *policy {
	return &policy{
		reachProb:   0.0,
		regretSum:   make([]float32, nActions),
		strategy:    uniformDist(nActions),
		strategySum: make([]float32, nActions),
	}
}

func (p *policy) numActions() int {
	return len(p.strategy)
}

func (p *policy) nextStrategy() {
	f32.AxpyUnitary(p.reachProb, p.strategy, p.strategySum)
	p.calcStrategy()
	p.reachProb = 0.0
}

func (p *policy) calcStrategy() {
	copy(p.strategy, p.regretSum)
	makePositive(p.strategy)
	total := f32.Sum(p.strategy)
	if total > 0 {
		f32.ScalUnitary(1.0/total, p.strategy)
		return
	}

	for i := range p.strategy {
		p.strategy[i] = 1.0 / float32(len(p.strategy))
	}
}

func (p *policy) getAverageStrategy() []float32 {
	total := f32.Sum(p.strategySum)
	if total > 0 {
		avgStrat := make([]float32, len(p.strategySum))
		f32.ScalUnitaryTo(avgStrat, 1.0/total, p.strategySum)
		return avgStrat
	}

	return uniformDist(len(p.strategy))
}

func (p *policy) update(actionUtils []float32, reachProb, counterFactualProb float32) float32 {
	p.reachProb += reachProb
	util := f32.DotUnitary(actionUtils, p.strategy)
	for i := range actionUtils {
		regret := actionUtils[i] - util
		p.regretSum[i] += counterFactualProb * regret
	}

	return util
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}

func purify(v []float32, tol float32) {
	for i := range v {
		if v[i] < tol {
			v[i] = 0.0
		}
	}
}
