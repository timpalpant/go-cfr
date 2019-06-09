package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type GeneralizedSamplingCFR struct {
	strategyProfile StrategyProfile
	sampler         Sampler

	slicePool *floatSlicePool
	mapPool   *keyIntMapPool
	rng       *rand.Rand

	traversingPlayer int
	sampledActions   map[string]int
}

func NewGeneralizedSampling(strategyProfile StrategyProfile, sampler Sampler) *GeneralizedSamplingCFR {
	return &GeneralizedSamplingCFR{
		strategyProfile: strategyProfile,
		sampler:         sampler,
		slicePool:       &floatSlicePool{},
		mapPool:         &keyIntMapPool{},
		rng:             rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *GeneralizedSamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = int(iter % 2)
	c.sampledActions = c.mapPool.alloc()
	defer c.mapPool.free(c.sampledActions)
	return c.runHelper(node, node.Player(), 1.0)
}

func (c *GeneralizedSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNodeType:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNodeType:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb)
	}

	node.Close()
	return ev
}

func (c *GeneralizedSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb)
}

func (c *GeneralizedSamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb)
	}
}

func (c *GeneralizedSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	if nChildren == 1 {
		// Optimization to skip trivial nodes with no real choice.
		child := node.GetChild(0)
		return c.runHelper(child, player, sampleProb)
	}

	policy := c.strategyProfile.GetPolicy(node)
	qs := c.slicePool.alloc(nChildren)
	copy(qs, c.sampler.Sample(node, policy))
	regrets := c.slicePool.alloc(nChildren)
	oldSampledActions := c.sampledActions
	c.sampledActions = c.mapPool.alloc()

	for i, q := range qs {
		child := node.GetChild(i)
		var util float32
		if q > 0 {
			util = c.runHelper(child, player, q*sampleProb)
		} else {
			util = c.probe(child, player)
		}

		regrets[i] = util
	}

	cfValue := f32.DotUnitary(policy.GetStrategy(), regrets)
	f32.AddConst(-cfValue, regrets)
	policy.AddRegret(1.0/sampleProb, qs, regrets)

	c.slicePool.free(qs)
	c.slicePool.free(regrets)
	c.mapPool.free(c.sampledActions)
	c.sampledActions = oldSampledActions
	return cfValue
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *GeneralizedSamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	policy := c.strategyProfile.GetPolicy(node)

	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	if sampleProb > 0 {
		policy.AddStrategyWeight(1.0 / sampleProb)
	}

	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	child := node.GetChild(getOrSample(c.sampledActions, node, policy, c.rng))
	return c.runHelper(child, node.Player(), sampleProb)
}

func (c *GeneralizedSamplingCFR) probe(node GameTreeNode, player int) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNodeType:
		ev = float32(node.Utility(player))
	case ChanceNodeType:
		child, _ := node.SampleChild()
		ev = c.probe(child, player)
	default:
		policy := c.strategyProfile.GetPolicy(node)
		strategy := policy.GetStrategy()
		x := c.rng.Float32()
		selected := sampleOne(strategy, x)
		child := node.GetChild(selected)
		ev = c.probe(child, player)
	}

	node.Close()
	return ev
}
