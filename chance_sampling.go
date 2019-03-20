package cfr

import (
	"fmt"
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type ChanceSamplingCFR struct {
	strategyProfile StrategyProfile
	slicePool       *threadSafeFloatSlicePool
}

func NewChanceSampling(strategyProfile StrategyProfile) *ChanceSamplingCFR {
	return &ChanceSamplingCFR{
		strategyProfile: strategyProfile,
		slicePool:       &threadSafeFloatSlicePool{},
	}
}

func (c *ChanceSamplingCFR) Run(node GameTreeNode) float32 {
	return c.runHelper(node, node.Player(), 1.0, 1.0)
}

func (c *ChanceSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, reachP0, reachP1 float32) float32 {
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

func (c *ChanceSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1 float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, reachP0, reachP1)
}

func (c *ChanceSamplingCFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1 float32) float32 {
	player := node.Player()
	strat := c.strategyProfile.GetStrategy(node)
	policy := strat.GetPolicy()

	advantages := c.slicePool.alloc(node.NumChildren())
	defer c.slicePool.free(advantages)
	var expectedUtil float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := policy[i]
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

// Sample one child of the given Chance node, according to its probability distribution.
func SampleChanceNode(node GameTreeNode) (GameTreeNode, float64) {
	x := rand.Float64()
	var cumProb float64
	n := node.NumChildren()
	for i := 0; i < n; i++ {
		p := node.GetChildProbability(i)
		cumProb += p
		if cumProb > x {
			return node.GetChild(i), p
		}
	}

	if cumProb < 1.0-eps { // Leave room for floating point error.
		panic(fmt.Errorf("probability distribution sums to %v != 1! node: %v, num children: %v",
			cumProb, node, n))
	}

	return node.GetChild(n - 1), node.GetChildProbability(n - 1)
}
