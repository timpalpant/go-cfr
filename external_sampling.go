package cfr

import (
	"github.com/timpalpant/go-cfr/internal/f32"
)

type ExternalSamplingCFR struct {
	strategyProfile StrategyProfile
	slicePool       *threadSafeFloatSlicePool
}

func NewExternalSampling(strategyProfile StrategyProfile) *ExternalSamplingCFR {
	return &ExternalSamplingCFR{
		strategyProfile: strategyProfile,
		slicePool:       &threadSafeFloatSlicePool{},
	}
}

func (c *ExternalSamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	traversingPlayer := int(iter % 2)
	sampledActions := make(map[string]int)
	return c.runHelper(node, node.Player(), 1.0, traversingPlayer, sampledActions)
}

func (c *ExternalSamplingCFR) runHelper(
	node GameTreeNode,
	lastPlayer int,
	sampleProb float32,
	traversingPlayer int,
	sampledActions map[string]int) float32 {

	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb, traversingPlayer, sampledActions)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb, traversingPlayer, sampledActions)
	}

	node.Close()
	return ev
}

func (c *ExternalSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb, traversingPlayer, sampledActions)
}

func (c *ExternalSamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	if traversingPlayer == node.Player() {
		return c.handleTraversingPlayerNode(node, sampleProb, traversingPlayer, sampledActions)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb, traversingPlayer, sampledActions)
	}
}

func (c *ExternalSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	strat := c.strategyProfile.GetStrategy(node)
	policy := c.slicePool.alloc(nChildren)
	defer c.slicePool.free(policy)
	policy = strat.GetPolicy(policy)
	advantages := c.slicePool.alloc(nChildren)
	defer c.slicePool.free(advantages)
	var expectedUtil float32
	for i := 0; i < nChildren; i++ {
		child := node.GetChild(i)
		p := policy[i]
		util := c.runHelper(child, player, p*sampleProb, traversingPlayer, sampledActions)
		advantages[i] = util
		expectedUtil += p * util
	}

	f32.AddConst(-expectedUtil, advantages)
	strat.AddRegret(sampleProb, 1.0, advantages)
	return expectedUtil
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *ExternalSamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions map[string]int) float32 {
	player := node.Player()
	strat := c.strategyProfile.GetStrategy(node)
	key := node.InfoSet(player).Key()

	i, ok := sampledActions[key]
	if !ok {
		// First time hitting this infoset during this run.
		// Sample according to current strategy profile.
		policy := c.slicePool.alloc(node.NumChildren())
		policy = strat.GetPolicy(policy)
		i = sampleOne(policy)
		c.slicePool.free(policy)
		sampledActions[key] = i
	}

	child := node.GetChild(i)
	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	return c.runHelper(child, player, sampleProb, traversingPlayer, sampledActions)
}
