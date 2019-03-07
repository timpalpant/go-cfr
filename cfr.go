package cfr

import (
	"math/rand"

	"github.com/golang/glog"

	"github.com/timpalpant/go-cfr/internal/f32"
)

const eps = 1e-3

type CFR struct {
	params          SamplingParams
	strategyProfile StrategyProfile
	iter            int
	nVisited        int
	slicePool       *floatSlicePool
}

func New(params SamplingParams, strategyProfile StrategyProfile) *CFR {
	return &CFR{
		params:          params,
		strategyProfile: strategyProfile,
		slicePool:       &floatSlicePool{},
	}
}

func (c *CFR) Run(node GameTreeNode) float32 {
	c.iter++
	c.nVisited = 0
	return c.runHelper(node, node.Player(), 1.0, 1.0, 1.0)
}

func (c *CFR) runHelper(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachChance float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = node.Utility(lastPlayer)
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, reachP0, reachP1, reachChance)
	default:
		sgn := getSign(lastPlayer, node)
		ev = sgn * c.handlePlayerNode(node, reachP0, reachP1, reachChance)
	}

	node.Close()
	c.nVisited++
	if c.nVisited%10000000 == 0 {
		glog.V(2).Infof("Visited %d million nodes", c.nVisited/1000000)
	}

	return ev
}

func (c *CFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachChance float32) float32 {
	if c.params.SampleChanceNodes {
		child := node.SampleChild()
		return c.runHelper(child, lastPlayer, reachP0, reachP1, reachChance)
	}

	var expectedValue float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := node.GetChildProbability(i)
		expectedValue += c.runHelper(child, lastPlayer, reachP0, reachP1, reachChance*p)
	}

	return expectedValue / float32(node.NumChildren())
}

func (c *CFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float32) float32 {
	player := node.Player()

	if node.NumChildren() == 1 { // Fast path for trivial nodes with no real choice.
		child := node.GetChild(0)
		return c.runHelper(child, player, reachP0, reachP1, reachChance)
	}

	strat := c.strategyProfile.GetStrategy(node)
	if c.params.SampleOpponentActions && c.traversingPlayer() != player {
		// Sample according to current strategy profile.
		i, p := sampleOne(strat, node.NumChildren())
		child := node.GetChild(i)
		if player == 0 {
			return c.runHelper(child, player, p*reachP0, reachP1, reachChance)
		} else {
			return c.runHelper(child, player, reachP0, p*reachP1, reachChance)
		}
	}

	advantages := c.slicePool.alloc(node.NumChildren())
	var expectedUtil float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := strat.GetActionProbability(i)
		var util float32
		if player == 0 {
			util = c.runHelper(child, player, p*reachP0, reachP1, reachChance)
		} else {
			util = c.runHelper(child, player, reachP0, p*reachP1, reachChance)
		}

		advantages[i] = util
		expectedUtil += p * util
	}

	// Transform action utilities into instantaneous advantages by
	// subtracting out the expected utility over all possible actions.
	f32.AddConst(-expectedUtil, advantages)
	reachP := reachProb(player, reachP0, reachP1, reachChance)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	strat.AddRegret(reachP, counterFactualP, advantages)
	c.slicePool.free(advantages) // Not using defer because it is slow.
	return expectedUtil
}

func getSign(lastPlayer int, child GameTreeNode) float32 {
	if child.Type() == PlayerNode && child.Player() != lastPlayer {
		return -1.0
	}

	return 1.0
}

func sampleOne(strat NodeStrategy, numActions int) (int, float32) {
	x := rand.Float32()
	var cumProb float32
	for i := 0; i < numActions; i++ {
		p := strat.GetActionProbability(i)
		cumProb += p
		if cumProb > x {
			return i, p
		}
	}

	if cumProb < 1.0-eps { // Leave room for floating point error.
		panic("probability distribution does not sum to 1!")
	}

	return numActions - 1, strat.GetActionProbability(numActions - 1)
}

func (c *CFR) traversingPlayer() int {
	return c.iter % 2
}

func reachProb(player int, reachP0, reachP1, reachChance float32) float32 {
	if player == 0 {
		return reachP0 * reachChance
	} else {
		return reachP1 * reachChance
	}
}

// The probability of reaching this node, assuming that the current player
// tried to reach it.
func counterFactualProb(player int, reachP0, reachP1, reachChance float32) float32 {
	if player == 0 {
		return reachP1 * reachChance
	} else {
		return reachP0 * reachChance
	}
}
