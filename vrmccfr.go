package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type VRMCCFR struct {
	strategyProfile StrategyProfile
	sampler         Sampler
	decayAlpha      float32

	slicePool *floatSlicePool
	rng       *rand.Rand

	traversingPlayer int
}

func NewVRMCCFR(strategyProfile StrategyProfile, sampler Sampler, decayAlpha float32) *VRMCCFR {
	return &VRMCCFR{
		strategyProfile: strategyProfile,
		sampler:         sampler,
		decayAlpha:      decayAlpha,
		slicePool:       &floatSlicePool{},
		rng:             rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *VRMCCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = int(iter % 2)
	return c.runHelper(node, node.Player(), 1.0)
}

func (c *VRMCCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
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

func (c *VRMCCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb)
}

func (c *VRMCCFR) handlePlayerNode(node GameTreeNode, sampleProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb)
	}
}

func (c *VRMCCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	if nChildren == 1 {
		// Optimization to skip trivial nodes with no real choice.
		child := node.GetChild(0)
		return c.runHelper(child, player, sampleProb)
	}

	policy := c.strategyProfile.GetPolicy(node)
	baseline := policy.GetBaseline()
	qs := c.slicePool.alloc(nChildren)
	copy(qs, c.sampler.Sample(node, policy))
	regrets := c.slicePool.alloc(nChildren)

	for i, q := range qs {
		child := node.GetChild(i)
		u := baseline[i]
		if q > 0 {
			u += (c.runHelper(child, player, q*sampleProb) - baseline[i]) / q
			c.updateBaseline(baseline, i, u)
		}

		regrets[i] = u
	}

	policy.SetBaseline(baseline)
	cfValue := f32.DotUnitary(policy.GetStrategy(), regrets)
	f32.AddConst(-cfValue, regrets)
	policy.AddRegret(1.0/sampleProb, regrets)

	c.slicePool.free(qs)
	c.slicePool.free(regrets)
	return cfValue
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *VRMCCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	policy := c.strategyProfile.GetPolicy(node)

	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	if sampleProb > 0 {
		policy.AddStrategyWeight(1.0 / sampleProb)
	}

	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	selected := sampleOne(policy.GetStrategy(), c.rng.Float32())
	child := node.GetChild(selected)
	result := c.runHelper(child, node.Player(), sampleProb)

	baseline := policy.GetBaseline()
	c.updateBaseline(baseline, selected, result)
	policy.SetBaseline(baseline)

	return result
}

func (c *VRMCCFR) updateBaseline(baseline []float32, i int, value float32) {
	baseline[i] *= (1 - c.decayAlpha)
	baseline[i] += c.decayAlpha * value
}
