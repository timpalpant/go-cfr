package cfr

import (
	"sync"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type ExternalSamplingCFR struct {
	strategyProfile StrategyProfile
	mu              sync.Mutex
	iter            int
	sampledActions  map[string]int
	slicePool       *threadSafeFloatSlicePool
}

func NewExternalSampling(strategyProfile StrategyProfile) *ExternalSamplingCFR {
	return &ExternalSamplingCFR{
		strategyProfile: strategyProfile,
		sampledActions:  make(map[string]int),
		slicePool:       &threadSafeFloatSlicePool{},
	}
}

func (c *ExternalSamplingCFR) Run(node GameTreeNode) float32 {
	c.iter++
	for k := range c.sampledActions {
		delete(c.sampledActions, k)
	}

	return c.runHelper(node, node.Player(), 1.0, 1.0)
}

func (c *ExternalSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, reachP0, reachP1 float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, reachP0, reachP1)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, reachP0, reachP1)
	}

	node.Close()
	return ev
}

func (c *ExternalSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1 float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, reachP0, reachP1)
}

func (c *ExternalSamplingCFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1 float32) float32 {
	if c.traversingPlayer() == node.Player() {
		return c.handleTraversingPlayerNode(node, reachP0, reachP1)
	} else {
		return c.handleSampledPlayerNode(node, reachP0, reachP1)
	}
}

func (c *ExternalSamplingCFR) handleTraversingPlayerNode(node GameTreeNode, reachP0, reachP1 float32) float32 {
	player := node.Player()
	strat := c.strategyProfile.GetStrategy(node)
	advantages := c.slicePool.alloc(node.NumChildren())
	defer c.slicePool.free(advantages)
	var expectedUtil float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := strat.GetActionProbability(i)
		var util float32
		if player == 0 {
			util = c.runHelper(child, player, p*reachP0, reachP1)
		} else {
			util = c.runHelper(child, player, reachP0, p*reachP1)
		}

		advantages[i] = util
		expectedUtil += p * util
	}

	// Transform action utilities into instantaneous advantages by
	// subtracting out the expected utility over all possible actions.
	f32.AddConst(-expectedUtil, advantages)
	reachP := reachProb(player, reachP0, reachP1, 1.0)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, 1.0)
	strat.AddRegret(reachP, counterFactualP, advantages)
	return expectedUtil
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *ExternalSamplingCFR) handleSampledPlayerNode(node GameTreeNode, reachP0, reachP1 float32) float32 {
	player := node.Player()
	strat := c.strategyProfile.GetStrategy(node)
	key := node.InfoSet(player).Key()

	c.mu.Lock()
	i, ok := c.sampledActions[key]
	if !ok {
		// First time hitting this infoset during this run.
		// Sample according to current strategy profile.
		i = sampleOne(strat, node.NumChildren())
		c.sampledActions[key] = i
	}
	c.mu.Unlock()

	child := node.GetChild(i)
	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	return c.runHelper(child, player, reachP0, reachP1)
}

func (c *ExternalSamplingCFR) traversingPlayer() int {
	return c.iter % 2
}
