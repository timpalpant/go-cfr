package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type VRMCCFR struct {
	strategyProfile      StrategyProfile
	traversingSampler    Sampler
	notTraversingSampler Sampler

	slicePool *floatSlicePool
	mapPool   *keyIntMapPool
	rng       *rand.Rand

	traversingPlayer int
	sampledActions   map[string]int
}

func NewVRMCCFR(strategyProfile StrategyProfile, traversingSampler, notTraversingSampler Sampler) *VRMCCFR {
	return &VRMCCFR{
		strategyProfile:      strategyProfile,
		traversingSampler:    traversingSampler,
		notTraversingSampler: notTraversingSampler,
		slicePool:            &floatSlicePool{},
		mapPool:              &keyIntMapPool{},
		rng:                  rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *VRMCCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = int(iter % 2)
	c.sampledActions = c.mapPool.alloc()
	defer c.mapPool.free(c.sampledActions)
	return c.runHelper(node, node.Player(), 1.0, 1.0)
}

func (c *VRMCCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb, reachProb float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNodeType:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNodeType:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb, reachProb)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb, reachProb)
	}

	node.Close()
	return ev
}

func (c *VRMCCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb, reachProb float32) float32 {
	child, p := node.SampleChild()
	return c.runHelper(child, lastPlayer, float32(p)*sampleProb, float32(p)*reachProb)
}

func (c *VRMCCFR) handlePlayerNode(node GameTreeNode, sampleProb, reachProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb, reachProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb, reachProb)
	}
}

func (c *VRMCCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb, reachProb float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	if nChildren == 1 {
		// Optimization to skip trivial nodes with no real choice.
		child := node.GetChild(0)
		return c.runHelper(child, player, sampleProb, reachProb)
	}

	policy := c.strategyProfile.GetPolicy(node)
	baseline := policy.GetBaseline()
	qs := c.slicePool.alloc(nChildren)
	copy(qs, c.traversingSampler.Sample(node, policy))
	regrets := c.slicePool.alloc(nChildren)
	oldSampledActions := c.sampledActions
	c.sampledActions = c.mapPool.alloc()

	for i, q := range qs {
		child := node.GetChild(i)
		uHat := baseline[i]
		if q > 0 {
			u := c.runHelper(child, player, q*sampleProb, reachProb)
			uHat += (u - baseline[i]) / q
			policy.UpdateBaseline(1.0/q, i, u)
		}

		regrets[i] = uHat
	}

	cfValue := f32.DotUnitary(policy.GetStrategy(), regrets)
	f32.AddConst(-cfValue, regrets)
	policy.AddRegret(reachProb/sampleProb, qs, regrets)

	c.slicePool.free(qs)
	c.slicePool.free(regrets)
	c.mapPool.free(c.sampledActions)
	c.sampledActions = oldSampledActions
	return cfValue
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *VRMCCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb, reachProb float32) float32 {
	policy := c.strategyProfile.GetPolicy(node)
	player := node.Player()
	nChildren := node.NumChildren()
	baseline := policy.GetBaseline()
	strategy := policy.GetStrategy()

	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	if sampleProb > 0 {
		policy.AddStrategyWeight(1.0 / sampleProb)
	}

	qs := c.slicePool.alloc(nChildren)
	copy(qs, c.notTraversingSampler.Sample(node, policy))
	regrets := c.slicePool.alloc(nChildren)

	for i, q := range qs {
		p := strategy[i]
		child := node.GetChild(i)
		uHat := baseline[i]
		if q > 0 {
			u := c.runHelper(child, player, q*sampleProb, p*reachProb)
			uHat += (u - baseline[i]) / q
			policy.UpdateBaseline(1.0/q, i, u)
		}

		regrets[i] = uHat
	}

	c.slicePool.free(qs)
	c.slicePool.free(regrets)
	return f32.DotUnitary(policy.GetStrategy(), regrets)
}
