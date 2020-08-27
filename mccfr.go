package cfr

import (
	"fmt"
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

// Sampler selects a subset of child nodes to traverse.
type Sampler interface {
	// Sample returns a vector of sampling probabilities for a
	// subset of the N children of this NodePolicy. Children with
	// p > 0 will be traversed. The returned slice may be reused
	// between calls to sample; a caller must therefore copy the
	// values before the next call to Sample.
	Sample(GameTreeNode, NodePolicy) []float32
}

type MCCFR struct {
	strategyProfile StrategyProfile
	sampler         Sampler

	slicePool *floatSlicePool
	mapPool   *keyIntMapPool
	rng       *rand.Rand

	traversingPlayer int
	sampledActions   map[string]int
}

func NewMCCFR(strategyProfile StrategyProfile, sampler Sampler) *MCCFR {
	return &MCCFR{
		strategyProfile: strategyProfile,
		sampler:         sampler,
		slicePool:       &floatSlicePool{},
		mapPool:         &keyIntMapPool{},
		rng:             rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *MCCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = int(iter % 2)
	c.sampledActions = c.mapPool.alloc()
	defer c.mapPool.free(c.sampledActions)
	return c.runHelper(node, node.Player(), 1.0)
}

func (c *MCCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
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

func (c *MCCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb)
}

func (c *MCCFR) handlePlayerNode(node GameTreeNode, sampleProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb)
	}
}

func (c *MCCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32) float32 {
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
func (c *MCCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32) float32 {
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

func getOrSample(sampledActions map[string]int, node GameTreeNode, policy NodePolicy, rng *rand.Rand) int {
	key := node.InfoSetKey(node.Player())
	selected, ok := sampledActions[string(key)]
	if !ok {
		x := rng.Float32()
		selected = sampleOne(policy.GetStrategy(), x)
		sampledActions[string(key)] = selected
	}

	if selected >= node.NumChildren() {
		panic(fmt.Errorf("sampled action: %d but node has %d children! node: %v, policy: %v",
			selected, node.NumChildren(), node, policy))
	}

	return selected
}

func sampleOne(pv []float32, x float32) int {
	var cumProb float32
	for i, p := range pv {
		cumProb += p
		if cumProb > x {
			return i
		}
	}

	if cumProb < 1.0-eps { // Leave room for floating point error.
		panic("probability distribution does not sum to 1!")
	}

	return len(pv) - 1
}
