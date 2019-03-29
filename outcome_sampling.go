package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

const eps = 1e-3

// OutcomeSamplingCFR performs CFR iterations by sampling all player and chance actions
// such that each run corresponds to a single terminal history through the game tree.
type OutcomeSamplingCFR struct {
	strategyProfile  StrategyProfile
	explorationDelta float32
	slicePool        *threadSafeFloatSlicePool
}

// NewOutcomeSampling creates a new OutcomeSamplingCFR with the given strategy profile.
// explorationDelta is the fraction of the time in (0.0, 1.0) to explore off-policy random
// actions.
func NewOutcomeSampling(strategyProfile StrategyProfile, explorationDelta float32) *OutcomeSamplingCFR {
	return &OutcomeSamplingCFR{
		strategyProfile:  strategyProfile,
		explorationDelta: explorationDelta,
		slicePool:        &threadSafeFloatSlicePool{},
	}
}

// Run performs a single iteration of outcome sampling CFR.
// It is safe to call concurrently from multiple goroutines if the underlying strategy profile is thread-safe.
func (c *OutcomeSamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	traversingPlayer := int(iter % 2)
	return c.runHelper(node, node.Player(), 1.0, traversingPlayer)
}

func (c *OutcomeSamplingCFR) runHelper(
	node GameTreeNode,
	lastPlayer int,
	sampleProb float32,
	traversingPlayer int) float32 {

	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer)) / sampleProb
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb, traversingPlayer)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb, traversingPlayer)
	}

	node.Close()
	return ev
}

func (c *OutcomeSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32, traversingPlayer int) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb, traversingPlayer)
}

func (c *OutcomeSamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int) float32 {
	if traversingPlayer == node.Player() {
		return c.handleTraversingPlayerNode(node, sampleProb, traversingPlayer)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb, traversingPlayer)
	}
}

func (c *OutcomeSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	regrets := c.slicePool.alloc(nChildren)
	defer c.slicePool.free(regrets)
	policy := c.strategyProfile.GetPolicy(node)
	strategy := policy.GetStrategy()

	// Sample one action according to current strategy profile + exploration.
	// No need to save since due to perfect recall an infoset will never be revisited.
	var selected int
	if rand.Float32() < c.explorationDelta {
		selected = rand.Intn(nChildren)
	} else {
		selected = sampleOne(strategy)
	}

	child := node.GetChild(selected)
	p := strategy[selected]

	f := c.explorationDelta
	sp := f * (1.0 / float32(nChildren)) // Due to exploration.
	sp += (1.0 - f) * p                  // Due to strategy.

	cfValue := c.runHelper(child, player, sp*sampleProb, traversingPlayer)
	regrets[selected] = cfValue
	f32.AddConst(-p*cfValue, regrets)
	policy.AddRegret(1.0, regrets)
	return p * cfValue
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *OutcomeSamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32, traversingPlayer int) float32 {
	policy := c.strategyProfile.GetPolicy(node)
	selected := sampleOne(policy.GetStrategy())

	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	policy.AddStrategyWeight(1.0 / sampleProb)

	child := node.GetChild(selected)
	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	return c.runHelper(child, node.Player(), sampleProb, traversingPlayer)
}

func sampleOne(pv []float32) int {
	x := rand.Float32()
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
