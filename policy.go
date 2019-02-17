package cfr

import (
	"gonum.org/v1/gonum/floats"
)

func reachProb(player int, reachP0, reachP1, reachChance float64) float64 {
	if player == 0 {
		return reachP0 * reachChance
	} else {
		return reachP1 * reachChance
	}
}

// The probability of reaching this node, assuming that the current player
// tried to reach it.
func counterFactualProb(player int, reachP0, reachP1, reachChance float64) float64 {
	if player == 0 {
		return reachP1 * reachChance
	} else {
		return reachP0 * reachChance
	}
}

type policy struct {
	reachProb   float64
	regretSum   []float64
	strategy    []float64
	strategySum []float64
}

func newPolicy(nActions int) *policy {
	return &policy{
		reachProb:   0.0,
		regretSum:   make([]float64, nActions),
		strategy:    uniformDist(nActions),
		strategySum: make([]float64, nActions),
	}
}

func (p *policy) numActions() int {
	return len(p.strategy)
}

func (p *policy) nextStrategy() {
	floats.AddScaled(p.strategySum, p.reachProb, p.strategy)
	p.strategy = p.calcStrategyUnsafe()
	p.reachProb = 0.0
}

func (p *policy) calcStrategyUnsafe() []float64 {
	strat := make([]float64, len(p.regretSum))
	copy(strat, p.regretSum)
	makePositive(strat)
	total := floats.Sum(strat)
	if total > 0 {
		vecDiv(strat, total)
		return strat
	}

	return uniformDist(len(strat))
}

func (p *policy) getAverageStrategy() []float64 {
	total := floats.Sum(p.strategySum)
	if total > 0 {
		avgStrat := make([]float64, len(p.strategySum))
		copy(avgStrat, p.strategySum)
		vecDiv(p.strategySum, total)
		purify(avgStrat, 0.001)
		total := floats.Sum(avgStrat)
		vecDiv(avgStrat, total) // Re-normalize.
		return avgStrat
	}

	return uniformDist(len(p.strategy))
}

func (p *policy) update(actionUtils []float64, reachProb, counterFactualProb float64) float64 {
	p.reachProb += reachProb
	util := floats.Dot(actionUtils, p.strategy)
	for i := range actionUtils {
		regret := actionUtils[i] - util
		p.regretSum[i] += counterFactualProb * regret
	}

	return util
}

func uniformDist(n int) []float64 {
	result := make([]float64, n)
	floats.AddConst(1.0/float64(n), result)
	return result
}

func makePositive(v []float64) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}

func vecDiv(v []float64, c float64) {
	for i := range v {
		v[i] /= c
	}
}

func purify(v []float64, tol float64) {
	for i := range v {
		if v[i] < tol {
			v[i] = 0.0
		}
	}
}
