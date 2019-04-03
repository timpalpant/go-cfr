package cfr

import (
	"github.com/timpalpant/go-cfr/internal/f32"
)

type ExternalSamplingCFR struct {
	strategyProfile       StrategyProfile
	sampledActionsFactory SampledActionsFactory
	slicePool             *threadSafeFloatSlicePool
}

func NewExternalSampling(strategyProfile StrategyProfile,
	sampledActionsFactory SampledActionsFactory) *ExternalSamplingCFR {
	return &ExternalSamplingCFR{
		strategyProfile:       strategyProfile,
		sampledActionsFactory: sampledActionsFactory,
		slicePool:             &threadSafeFloatSlicePool{},
	}
}

func (c *ExternalSamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	traversingPlayer := int(iter % 2)
	sampledActions := c.sampledActionsFactory()
	defer sampledActions.Close()
	return c.runHelper(node, node.Player(), 1.0, traversingPlayer, sampledActions)
}

func (c *ExternalSamplingCFR) runHelper(
	node GameTreeNode,
	lastPlayer int,
	sampleProb float32,
	traversingPlayer int,
	sampledActions SampledActions) float32 {

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

func (c *ExternalSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32, traversingPlayer int, sampledActions SampledActions) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb, traversingPlayer, sampledActions)
}

func (c *ExternalSamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions SampledActions) float32 {
	if traversingPlayer == node.Player() {
		return c.handleTraversingPlayerNode(node, sampleProb, traversingPlayer)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb, traversingPlayer, sampledActions)
	}
}

func (c *ExternalSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	policy := c.strategyProfile.GetPolicy(node)
	strategy := policy.GetStrategy()
	regrets := c.slicePool.alloc(nChildren)
	defer c.slicePool.free(regrets)
	sampledActions := c.sampledActionsFactory()
	defer sampledActions.Close()
	var cfValue float32
	for i := 0; i < nChildren; i++ {
		child := node.GetChild(i)
		p := strategy[i]
		regrets[i] = c.runHelper(child, player, p*sampleProb, traversingPlayer, sampledActions)
		cfValue += p * regrets[i]
	}

	if sampleProb > 0 {
		f32.AddConst(-cfValue, regrets)
		policy.AddRegret(1.0/sampleProb, regrets)
	}

	return cfValue
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *ExternalSamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int, sampledActions SampledActions) float32 {
	player := node.Player()
	policy := c.strategyProfile.GetPolicy(node)
	selected := sampledActions.Get(node, policy)

	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	if sampleProb > 0 {
		policy.AddStrategyWeight(1.0 / sampleProb)
	}

	child := node.GetChild(selected)
	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	return c.runHelper(child, player, sampleProb, traversingPlayer, sampledActions)
}
