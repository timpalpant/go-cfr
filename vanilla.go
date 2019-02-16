package cfr

import (
	"fmt"

	"gonum.org/v1/gonum/floats"
)

type Vanilla struct {
	// Map of player -> InfoSet -> Strategy for that InfoSet.
	strategyProfile map[int]map[string]*policy
	slicePool       *floatSlicePool
}

var _ CFR = &Vanilla{}

func NewVanilla() *Vanilla {
	return &Vanilla{
		strategyProfile: map[int]map[string]*policy{
			0: make(map[string]*policy),
			1: make(map[string]*policy),
		},
		slicePool: &floatSlicePool{},
	}
}

func (v *Vanilla) GetStrategy(player int, infoSet string) []float64 {
	policy := v.strategyProfile[player][infoSet]
	if policy == nil {
		return nil
	}

	return policy.getAverageStrategy()
}

func (v *Vanilla) Run(node GameTreeNode) float64 {
	expectedValue := v.runHelper(node, 1.0, 1.0, 1.0)
	v.nextStrategyProfile()
	return expectedValue
}

func (v *Vanilla) nextStrategyProfile() {
	for _, policies := range v.strategyProfile {
		for _, p := range policies {
			p.nextStrategy()
		}
	}
}

func (v *Vanilla) runHelper(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	defer node.Reset()
	if IsTerminal(node) {
		return node.Utility(node.Player())
	} else if node.IsChance() {
		return v.handleChanceNode(node, reachP0, reachP1, reachChance)
	}

	return v.handlePlayerNode(node, reachP0, reachP1, reachChance)
}

func (v *Vanilla) handleChanceNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	expectedValue := 0.0
	n := node.NumChildren()
	for i := 0; i < n; i++ {
		child := node.GetChild(i)
		p := node.GetChildProbability(i)
		expectedValue += v.runHelper(child, reachP0, reachP1, reachChance*p)
	}

	return expectedValue / float64(n)
}

func (v *Vanilla) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	policy := v.getPolicy(node)
	player := node.Player()
	var reachProb float64
	if player == 0 {
		reachProb = reachP0 * reachChance
	} else {
		reachProb = reachP1 * reachChance
	}
	policy.reachProb += reachProb

	actionUtils := v.slicePool.alloc(node.NumChildren())
	defer v.slicePool.free(actionUtils)
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := policy.strategy[i]
		if player == 0 {
			actionUtils[i] = -1 * v.runHelper(child, p*reachP0, reachP1, reachChance)
		} else {
			actionUtils[i] = -1 * v.runHelper(child, reachP0, p*reachP1, reachChance)
		}
	}

	util := floats.Dot(actionUtils, policy.strategy)
	// The probability of reaching this node, assuming that the current player
	// tried to reach it.
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	for i := range actionUtils {
		regret := actionUtils[i] - util
		policy.regretSum[i] += counterFactualP * regret
	}

	return util
}

func counterFactualProb(player int, reachP0, reachP1, reachChance float64) float64 {
	if player == 0 {
		return reachP1 * reachChance
	} else {
		return reachP0 * reachChance
	}
}

func (v *Vanilla) getPolicy(node GameTreeNode) *policy {
	p := node.Player()
	is := node.InfoSet(p)
	if policy, ok := v.strategyProfile[p][is]; ok {
		if node.NumChildren() != policy.numActions() {
			panic(fmt.Errorf("policy has n_actions=%v but node has n_children=%v: %v - %v",
				policy.numActions(), node.NumChildren(), node, is))
		}
		return policy
	}

	policy := newPolicy(node.NumChildren())
	v.strategyProfile[p][is] = policy
	return policy
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
	p.strategy = p.calcStrategy()
	p.reachProb = 0.0
}

func (p *policy) calcStrategy() []float64 {
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
