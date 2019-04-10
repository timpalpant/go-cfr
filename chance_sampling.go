package cfr

import (
	"github.com/timpalpant/go-cfr/internal/f32"
)

const eps = 1e-3

type ChanceSamplingCFR struct {
	strategyProfile StrategyProfile
	slicePool       *floatSlicePool
}

func NewChanceSampling(strategyProfile StrategyProfile) *ChanceSamplingCFR {
	return &ChanceSamplingCFR{
		strategyProfile: strategyProfile,
		slicePool:       &floatSlicePool{},
	}
}

func (c *ChanceSamplingCFR) Run(node GameTreeNode) float32 {
	return c.runHelper(node, node.Player(), 1.0, 1.0)
}

func (c *ChanceSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, reachP0, reachP1 float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, reachP0, reachP1)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, reachP0, reachP1)
	}

	node.Close()
	return ev
}

func (c *ChanceSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1 float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, reachP0, reachP1)
}

func (c *ChanceSamplingCFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1 float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	if nChildren == 1 {
		// Optimization to skip trivial nodes with no real choice.
		child := node.GetChild(0)
		return c.runHelper(child, player, reachP0, reachP1)
	}

	policy := c.strategyProfile.GetPolicy(node)
	strategy := policy.GetStrategy()

	regrets := c.slicePool.alloc(nChildren)
	defer c.slicePool.free(regrets)
	var cfValue float32
	for i := 0; i < nChildren; i++ {
		child := node.GetChild(i)
		p := strategy[i]
		var util float32
		if player == 0 {
			util = c.runHelper(child, player, p*reachP0, reachP1)
		} else {
			util = c.runHelper(child, player, reachP0, p*reachP1)
		}

		regrets[i] = util
		cfValue += p * util
	}

	// Transform action utilities into instantaneous regrets by
	// subtracting out the expected utility over all possible actions.
	f32.AddConst(-cfValue, regrets)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, 1.0)
	policy.AddRegret(counterFactualP, regrets)
	reachP := reachProb(player, reachP0, reachP1, 1.0)
	policy.AddStrategyWeight(reachP)
	return cfValue
}
