package cfr

import (
	"github.com/timpalpant/go-cfr/internal/f32"
)

type CFR struct {
	strategyProfile StrategyProfile
	slicePool       *floatSlicePool
}

func New(strategyProfile StrategyProfile) *CFR {
	return &CFR{
		strategyProfile: strategyProfile,
		slicePool:       &floatSlicePool{},
	}
}

func (c *CFR) Run(node GameTreeNode) float32 {
	return c.runHelper(node, node.Player(), 1.0, 1.0, 1.0)
}

func (c *CFR) runHelper(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachChance float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer))
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, reachP0, reachP1, reachChance)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, reachP0, reachP1, reachChance)
	}

	node.Close()
	return ev
}

func (c *CFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachChance float32) float32 {
	var expectedValue float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := float32(node.GetChildProbability(i))
		expectedValue += p * c.runHelper(child, lastPlayer, reachP0, reachP1, reachChance*p)
	}

	return expectedValue
}

func (c *CFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	policy := c.strategyProfile.GetPolicy(node)
	strategy := policy.GetStrategy()
	regrets := c.slicePool.alloc(nChildren)
	defer c.slicePool.free(regrets)
	var cfValue float32
	for i := 0; i < nChildren; i++ {
		child := node.GetChild(i)
		p := strategy[i]
		var util float32
		if player == 0 {
			util = c.runHelper(child, player, p*reachP0, reachP1, reachChance)
		} else {
			util = c.runHelper(child, player, reachP0, p*reachP1, reachChance)
		}

		regrets[i] = util
		cfValue += p * util
	}

	// Transform action utilities into instantaneous regrets by
	// subtracting out the expected utility over all possible actions.
	f32.AddConst(-cfValue, regrets)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	policy.AddRegret(counterFactualP, regrets)
	reachP := reachProb(player, reachP0, reachP1, reachChance)
	policy.AddStrategyWeight(reachP)
	return cfValue
}

func getSign(player1, player2 int) float32 {
	if player1 == player2 {
		return 1.0
	}

	return -1.0
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
