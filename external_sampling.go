package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type ExternalSamplingCFR struct {
	strategyProfile StrategyProfile
	slicePool       *floatSlicePool
	mapPool         *stringIntMapPool
	rng             *rand.Rand

	traversingPlayer int
	sampledActions   map[string]int
}

func NewExternalSampling(strategyProfile StrategyProfile) *ExternalSamplingCFR {
	return &ExternalSamplingCFR{
		strategyProfile: strategyProfile,
		slicePool:       &floatSlicePool{},
		mapPool:         &stringIntMapPool{},
		rng:             rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *ExternalSamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = iter % 2
	c.sampledActions = c.mapPool.alloc()
	defer c.mapPool.free(c.sampledActions)
	return c.runHelper(node, node.Player(), 1.0)
}

func (c *ExternalSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb)
	}

	node.Close()
	return ev
}

func (c *ExternalSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb)
}

func (c *ExternalSamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb)
	}
}

func (c *ExternalSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	policy := c.strategyProfile.GetPolicy(node)
	strategy := policy.GetStrategy()
	regrets := c.slicePool.alloc(nChildren)
	oldSampledActions := c.sampledActions
	c.sampledActions = c.mapPool.alloc()

	var cfValue float32
	for i := 0; i < nChildren; i++ {
		child := node.GetChild(i)
		p := strategy[i]
		regrets[i] = c.runHelper(child, player, p*sampleProb)
		cfValue += p * regrets[i]
	}

	if sampleProb > 0 {
		f32.AddConst(-cfValue, regrets)
		policy.AddRegret(1.0/sampleProb, regrets)
	}

	c.slicePool.free(regrets)
	c.mapPool.free(c.sampledActions)
	c.sampledActions = oldSampledActions
	return cfValue
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *ExternalSamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	policy := c.strategyProfile.GetPolicy(node)

	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	if sampleProb > 0 {
		policy.AddStrategyWeight(1.0 / sampleProb)
	}

	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	child := getOrSample(c.sampledActions, node, policy, c.rng)
	return c.runHelper(child, node.Player(), sampleProb)
}

func getOrSample(sampledActions map[string]int, node GameTreeNode, policy NodePolicy, rng *rand.Rand) GameTreeNode {
	player := node.Player()
	is := node.InfoSet(player)
	key := is.Key()

	selected, ok := sampledActions[key]
	if !ok {
		x := rng.Float32()
		selected = sampleOne(policy.GetStrategy(), x)
		sampledActions[key] = selected
	}

	return node.GetChild(selected)
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
