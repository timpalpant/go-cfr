package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type OnlineOutcomeSamplingCFR struct {
	strategyProfile StrategyProfile
	sampler         Sampler

	slicePool *floatSlicePool
	mapPool   *keyIntMapPool
	rng       *rand.Rand

	traversingPlayer int
	sampledActions   map[string]int
}

func NewOnlineOutcomeSamplingCFR(strategyProfile StrategyProfile, sampler Sampler) *OnlineOutcomeSamplingCFR {
	return &OnlineOutcomeSamplingCFR{
		strategyProfile: strategyProfile,
		sampler:         sampler,
		slicePool:       &floatSlicePool{},
		mapPool:         &keyIntMapPool{},
		rng:             rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *OnlineOutcomeSamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = int(iter % 2)
	c.sampledActions = c.mapPool.alloc()
	defer c.mapPool.free(c.sampledActions)
	return c.runHelper(node, node.Player(), 1.0)
}

func (c *OnlineOutcomeSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNodeType:
		ev = float32(node.Utility(lastPlayer)) / sampleProb
	case ChanceNodeType:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb)
	}

	node.Close()
	return ev
}

func (c *OnlineOutcomeSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb)
}

func (c *OnlineOutcomeSamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb)
	}
}

func (c *OnlineOutcomeSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	if nChildren == 1 {
		// Optimization to skip trivial nodes with no real choice.
		child := node.GetChild(0)
		return c.runHelper(child, player, sampleProb)
	}

	policy := c.strategyProfile.GetPolicy(node)
	isNew := policy.IsEmpty()
	qs := c.slicePool.alloc(nChildren)
	copy(qs, c.sampler.Sample(node, policy))
	regrets := c.slicePool.alloc(nChildren)
	oldSampledActions := c.sampledActions
	c.sampledActions = c.mapPool.alloc()
	strategy := policy.GetStrategy()
	for i, q := range qs {
		child := node.GetChild(i)
		var util float32
		if q > 0 {
			if isNew {
				util = c.randomRollout(child, player, q*sampleProb)
			} else {
				util = c.runHelper(child, player, q*sampleProb)
			}
		}

		regrets[i] = util
	}

	cfValue := f32.DotUnitary(strategy, regrets)
	f32.AddConst(-cfValue, regrets)
	policy.AddRegret(1.0/sampleProb, qs, regrets)

	c.slicePool.free(qs)
	c.slicePool.free(regrets)
	c.mapPool.free(c.sampledActions)
	c.sampledActions = oldSampledActions
	return cfValue
}

func (c *OnlineOutcomeSamplingCFR) randomRollout(node GameTreeNode, player int, sampleProb float32) float32 {
	x := float64(1.0)
	for node.Type() != TerminalNodeType {
		nChildren := node.NumChildren()
		if node.Type() == PlayerNodeType && node.Player() == player {
			x /= float64(nChildren)
		}

		selected := c.rng.Intn(nChildren)
		node = node.GetChild(selected)
		defer node.Close()
	}

	return float32(x * node.Utility(player))
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *OnlineOutcomeSamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32) float32 {
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
