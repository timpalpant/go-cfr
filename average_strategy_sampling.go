package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
	"github.com/timpalpant/go-cfr/internal/policy"
)

type ASSamplingParams struct {
	Epsilon float32
	Tau     float32
	Beta    float32
}

type AverageStrategySamplingCFR struct {
	params          ASSamplingParams
	strategyProfile *PolicyTable
	slicePool       *threadSafeFloatSlicePool
}

func NewAverageStrategySampling(strategyProfile *PolicyTable, params ASSamplingParams) *AverageStrategySamplingCFR {
	return &AverageStrategySamplingCFR{
		params:          params,
		strategyProfile: strategyProfile,
		slicePool:       &threadSafeFloatSlicePool{},
	}
}

func (c *AverageStrategySamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	traversingPlayer := int(iter % 2)
	sampledActions := make(map[string]int)
	return c.runHelper(node, node.Player(), 1.0, traversingPlayer, sampledActions)
}

func (c *AverageStrategySamplingCFR) runHelper(
	node GameTreeNode,
	lastPlayer int,
	sampleProb float32,
	traversingPlayer int,
	sampledActions map[string]int) float32 {

	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer)) / sampleProb
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb, traversingPlayer, sampledActions)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb, traversingPlayer, sampledActions)
	}

	node.Close()
	return ev
}

func (c *AverageStrategySamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb, traversingPlayer, sampledActions)
}

func (c *AverageStrategySamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	if traversingPlayer == node.Player() {
		return c.handleTraversingPlayerNode(node, sampleProb, traversingPlayer, sampledActions)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb, traversingPlayer, sampledActions)
	}
}

func (c *AverageStrategySamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	regrets := c.slicePool.alloc(nChildren)
	defer c.slicePool.free(regrets)
	x := rand.Float32()
	policy := c.strategyProfile.GetPolicy(node).(*policy.Policy)
	strategy := policy.GetStrategy()
	s := policy.GetStrategySum()
	sSum := f32.Sum(s)
	var cfValue float32
	for i := 0; i < nChildren; i++ {
		child := node.GetChild(i)
		p := strategy[i]
		rho := computeRho(s[i], sSum, c.params)
		if x < rho {
			util := c.runHelper(child, player, minF32(rho, 1.0)*sampleProb, traversingPlayer, sampledActions)
			regrets[i] = util
			cfValue += p * util
		}
	}

	f32.AddConst(-cfValue, regrets)
	policy.AddRegret(regrets)
	return cfValue
}

func minF32(x, y float32) float32 {
	if x < y {
		return x
	}

	return y
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *AverageStrategySamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	policy := c.strategyProfile.GetPolicy(node)
	policy.AddStrategyWeight(1.0 / sampleProb)

	player := node.Player()
	key := node.InfoSet(player).Key()
	i, ok := sampledActions[key]
	if !ok {
		// First time hitting this infoset during this run.
		// Sample according to current strategy profile.
		strategy := c.strategyProfile.GetPolicy(node).GetStrategy()
		i = sampleOne(strategy)
		sampledActions[key] = i
	}

	child := node.GetChild(i)
	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	return c.runHelper(child, player, sampleProb, traversingPlayer, sampledActions)
}

func computeRho(s, sSum float32, params ASSamplingParams) float32 {
	rho := params.Beta + params.Tau*s
	rho /= params.Beta + sSum
	if rho < params.Epsilon {
		return params.Epsilon
	}

	return rho
}
