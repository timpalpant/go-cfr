package cfr

import (
	"gonum.org/v1/gonum/floats"
)

type Strategy struct {
	reachProb   float64
	regretSum   []float64
	strategy    []float64
	strategySum []float64
}

func newStrategy(nActions int) *Strategy {
	return &Strategy{
		reachProb:   0.0,
		regretSum:   make([]float64, nActions),
		strategy:    uniformDist(nActions),
		strategySum: make([]float64, nActions),
	}
}

func (s *Strategy) nextStrategy() {
	floats.AddScaled(s.strategySum, s.reachProb, s.strategy)
	s.strategy = s.calcStrategy()
	s.reachProb = 0.0
}

func (s *Strategy) calcStrategy() []float64 {
	strat := make([]float64, len(s.regretSum))
	copy(strat, s.regretSum)
	makePositive(strat)
	total := floats.Sum(strat)
	if total > 0 {
		vecDiv(strat, total)
		return strat
	}

	return uniformDist(len(strat))
}

func (s *Strategy) getAverageStrategy() []float64 {
	total := floats.Sum(s.strategySum)
	if total > 0 {
		avgStrat := make([]float64, len(s.strategySum))
		copy(avgStrat, s.strategySum)
		vecDiv(s.strategySum, total)
		purify(avgStrat, 0.001)
		total := floats.Sum(avgStrat)
		vecDiv(avgStrat, total) // Re-normalize.
		return avgStrat
	}

	return uniformDist(len(s.strategy))
}

type Vanilla struct {
	strategyProfile map[int]map[InfoSet]*Strategy
}

func NewVanilla(game Game) *Vanilla {
	strategyProfile := make(map[int]map[InfoSet]*Strategy, game.NumPlayers())
	for i := 0; i < game.NumPlayers(); i++ {
		strategyProfile[i] = make(map[InfoSet]*Strategy)
	}

	return &Vanilla{strategyProfile}
}

func (v *Vanilla) Run(node GameTreeNode) float64 {
	expectedValue := v.runHelper(node, 1.0, 1.0, 1.0)
	v.nextStrategyProfile()
	return expectedValue
}

func (v *Vanilla) nextStrategyProfile() {
	for _, strategies := range v.strategyProfile {
		for _, strat := range strategies {
			strat.nextStrategy()
		}
	}
}

func (v *Vanilla) runHelper(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	if IsTerminal(node) {
		return node.Utility()
	} else if node.IsChance() {
		return v.handleChanceNode(node, reachP0, reachP1, reachChance)
	}

	return v.handlePlayerNode(node, reachP0, reachP1, reachChance)
}

func (v *Vanilla) handleChanceNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	expectedValue := 0.0
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := node.GetChildProbability(i)
		expectedValue += v.runHelper(child, reachP0, reachP1, reachChance*p)
	}

	return expectedValue / float64(node.NumChildren())
}

func (v *Vanilla) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	strat := v.getStrategy(node)
	if node.Player() == 0 {
		strat.reachProb += reachP0
	} else {
		strat.reachProb += reachP1
	}

	actionUtils := make([]float64, node.NumChildren())
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		if node.Player() == 0 {
			actionUtils[i] = -1 * v.runHelper(child, reachP0*strat.strategy[i], reachP1, reachChance)
		} else {
			actionUtils[i] = -1 * v.runHelper(child, reachP0, reachP1*strat.strategy[i], reachChance)
		}
	}

	util := floats.Dot(actionUtils, strat.strategy)
	if node.Player() == 0 {
		for i := range actionUtils {
			regret := actionUtils[i] - util
			strat.regretSum[i] += reachP1 * reachChance * regret
		}
	} else {
		for i := range actionUtils {
			regret := actionUtils[i] - util
			strat.regretSum[i] += reachP0 * reachChance * regret
		}
	}

	return util
}

func (v *Vanilla) getStrategy(node GameTreeNode) *Strategy {
	p := node.Player()
	is := node.InfoSet()
	if strat, ok := v.strategyProfile[p][is]; ok {
		return strat
	}

	strat := newStrategy(node.NumChildren())
	v.strategyProfile[p][is] = strat
	return strat
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
