package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type RobustSamplingCFR struct {
	strategyProfile StrategyProfile
	k               int

	slicePool *floatSlicePool
	mapPool   *stringIntMapPool
	rng       *rand.Rand
	aranges   [][]int

	traversingPlayer int
	sampledActions   map[string]int
}

func NewRobustSampling(strategyProfile StrategyProfile, k int) *RobustSamplingCFR {
	return &RobustSamplingCFR{
		strategyProfile: strategyProfile,
		k:               k,
		slicePool:       &floatSlicePool{},
		mapPool:         &stringIntMapPool{},
		rng:             rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *RobustSamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = int(iter % 2)
	c.sampledActions = c.mapPool.alloc()
	defer c.mapPool.free(c.sampledActions)
	return c.runHelper(node, node.Player(), 1.0)
}

func (c *RobustSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer)) / sampleProb
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb)
	}

	node.Close()
	return ev
}

func (c *RobustSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb)
}

func (c *RobustSamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb)
	}
}

func (c *RobustSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	policy := c.strategyProfile.GetPolicy(node)
	strategy := policy.GetStrategy()

	// Sample min(k, |A|) actions with uniform probability.
	selected := c.arange(nChildren)
	if c.k < len(selected) {
		c.rng.Shuffle(len(selected), func(i, j int) {
			selected[i], selected[j] = selected[j], selected[i]
		})

		selected = selected[:c.k]
	}

	q := float32(len(selected)) / float32(nChildren)
	regrets := c.slicePool.alloc(nChildren)
	oldSampledActions := c.sampledActions
	c.sampledActions = c.mapPool.alloc()

	var cfValue float32
	for _, i := range selected {
		child := node.GetChild(i)
		p := strategy[i]
		util := c.runHelper(child, player, q*sampleProb)
		regrets[i] = util
		cfValue += p * util
	}

	f32.AddConst(-cfValue, regrets)
	policy.AddRegret(1.0/q, regrets)

	c.slicePool.free(regrets)
	c.mapPool.free(c.sampledActions)
	c.sampledActions = oldSampledActions
	return cfValue
}

func (c *RobustSamplingCFR) arange(n int) []int {
	if n >= len(c.aranges) {
		for len(c.aranges) <= n {
			r := arange(len(c.aranges))
			c.aranges = append(c.aranges, r)
		}
	}

	return c.aranges[n]
}

func min(i, j int) int {
	if i < j {
		return i
	}

	return j
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *RobustSamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32) float32 {
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

func arange(n int) []int {
	result := make([]int, n)
	for i := range result {
		result[i] = i
	}
	return result
}
