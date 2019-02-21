package cfr

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
	addScaled(p.strategySum, p.reachProb, p.strategy)
	p.strategy = p.calcStrategy()
	p.reachProb = 0.0
}

func (p *policy) calcStrategy() []float32 {
	strat := make([]float32, len(p.regretSum))
	copy(strat, p.regretSum)
	makePositive(strat)
	total := sum(strat)
	if total > 0 {
		vecDiv(strat, total)
		return strat
	}

	return uniformDist(len(strat))
}

func (p *policy) getAverageStrategy() []float32 {
	total := sum(p.strategySum)
	if total > 0 {
		avgStrat := make([]float32, len(p.strategySum))
		copy(avgStrat, p.strategySum)
		vecDiv(avgStrat, total)
		return avgStrat
	}

	return uniformDist(len(p.strategy))
}

func (p *policy) update(actionUtils []float32, reachProb, counterFactualProb float32) float32 {
	p.reachProb += reachProb
	util := dot(actionUtils, p.strategy)
	for i := range actionUtils {
		regret := actionUtils[i] - util
		p.regretSum[i] += counterFactualProb * regret
	}

	return util
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	for i := range result {
		result[i] = 1.0 / float32(n)
	}
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}

// We can replace this with something from gonum.org/v1/gonum/internal/asm/f32
// if it is performance critical.
func addScaled(dst []float32, c float32, x []float32) {
	for i := range x {
		dst[i] += c * x[i]
	}
}

func sum(x []float32) float32 {
	var result float32
	for _, xi := range x {
		result += xi
	}
	return result
}

// SIMD dot function is here if needed: https://godoc.org/gonum.org/v1/gonum/internal/asm/f32#DotUnitary
func dot(x, y []float32) float32 {
	var result float32
	for i := range x {
		result += x[i] * y[i]
	}
	return result
}

func vecDiv(v []float32, c float32) {
	for i := range v {
		v[i] /= c
	}
}

func purify(v []float32, tol float32) {
	for i := range v {
		if v[i] < tol {
			v[i] = 0.0
		}
	}
}
