package cfr

import (
	"github.com/timpalpant/go-cfr/internal/f32"
	"math/rand"
)

type ASSamplingParams struct {
	Epsilon float32
	Tau     float32
	Beta    float32
}

type AverageStrategySamplingCFR struct {
	params          ASSamplingParams
	strategyProfile StrategyProfile
	slicePool       *threadSafeFloatSlicePool
}

func NewAverageStrategySampling(strategyProfile StrategyProfile, params ASSamplingParams) *AverageStrategySamplingCFR {
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
	return c.runHelper(node, node.Player(), 1.0, 1.0, traversingPlayer, sampledActions)
}

func (c *AverageStrategySamplingCFR) runHelper(
	node GameTreeNode,
	lastPlayer int,
	reachP0 float32,
	reachP1 float32,
	traversingPlayer int,
	sampledActions map[string]int) float32 {

	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, reachP0, reachP1, traversingPlayer, sampledActions)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, reachP0, reachP1, traversingPlayer, sampledActions)
	}

	node.Close()
	return ev
}

func (c *AverageStrategySamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1 float32, traversingPlayer int, sampledActions map[string]int) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, reachP0, reachP1, traversingPlayer, sampledActions)
}

func (c *AverageStrategySamplingCFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1 float32, traversingPlayer int, sampledActions map[string]int) float32 {
	if traversingPlayer == node.Player() {
		return c.handleTraversingPlayerNode(node, reachP0, reachP1, traversingPlayer, sampledActions)
	} else {
		return c.handleSampledPlayerNode(node, reachP0, reachP1, traversingPlayer, sampledActions)
	}
}

func (c *AverageStrategySamplingCFR) handleTraversingPlayerNode(node GameTreeNode, reachP0, reachP1 float32, traversingPlayer int, sampledActions map[string]int) float32 {
	player := node.Player()
	strat := c.strategyProfile.GetStrategy(node).(updateableNodeStrategy)
	advantages := c.slicePool.alloc(node.NumChildren())
	defer c.slicePool.free(advantages)
	x := rand.Float32()
	sSum := computeSum(strat)
	var expectedUtil float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		s := strat.getStrategySum(i)
		rho := computeRho(s, sSum, c.params)
		if x < rho {
			p := strat.GetActionProbability(i)
			var util float32
			if player == 0 {
				util = c.runHelper(child, player, p*reachP0, reachP1, traversingPlayer, sampledActions)
			} else {
				util = c.runHelper(child, player, reachP0, p*reachP1, traversingPlayer, sampledActions)
			}

			advantages[i] = util
			expectedUtil += p * util
		}
	}

	f32.AddConst(-expectedUtil, advantages)
	reachP := reachProb(player, reachP0, reachP1, 1.0)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, 1.0)
	strat.AddRegret(reachP, counterFactualP, advantages)
	return expectedUtil
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *AverageStrategySamplingCFR) handleSampledPlayerNode(node GameTreeNode, reachP0, reachP1 float32, traversingPlayer int, sampledActions map[string]int) float32 {
	player := node.Player()
	strat := c.strategyProfile.GetStrategy(node)
	key := node.InfoSet(player).Key()

	i, ok := sampledActions[key]
	if !ok {
		// First time hitting this infoset during this run.
		// Sample according to current strategy profile.
		i = sampleOne(strat, node.NumChildren())
		sampledActions[key] = i
	}

	child := node.GetChild(i)
	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	return c.runHelper(child, player, reachP0, reachP1, traversingPlayer, sampledActions)
}

func computeRho(s, sSum float32, params ASSamplingParams) float32 {
	rho := params.Beta + params.Tau*s
	rho /= params.Beta + sSum
	if rho < params.Epsilon {
		return params.Epsilon
	}

	return rho
}

func computeSum(s updateableNodeStrategy) float32 {
	var result float32
	for i := 0; i < s.numActions(); i++ {
		result += s.getStrategySum(i)
	}
	return result
}
