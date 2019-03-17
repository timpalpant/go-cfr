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
	return c.runHelper(node, node.Player(), 1.0, 1.0, 1.0)
}

func (c *OutcomeSamplingCFR) runHelper(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachSigmaPrime float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer)) / reachSigmaPrime
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, reachP0, reachP1, reachSigmaPrime)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, reachP0, reachP1, reachSigmaPrime)
	}

	node.Close()
	return ev
}

func (c *OutcomeSamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachSigmaPrime float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, reachP0, reachP1, reachSigmaPrime)
}

func (c *OutcomeSamplingCFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachSigmaPrime float32) float32 {
	player := node.Player()
	strat := c.strategyProfile.GetStrategy(node)

	// Sample one action according to current strategy profile + exploration.
	// No need to save since due to perfect recall an infoset will never be revisited.
	var selectedAction int
	if rand.Float32() < c.explorationDelta {
		selectedAction = rand.Intn(node.NumChildren())
	} else {
		selectedAction = sampleOne(strat, node.NumChildren())
	}

	child := node.GetChild(selectedAction)
	p := strat.GetActionProbability(selectedAction)
	f := c.explorationDelta
	sigmaPrime := f * (1.0 / float32(node.NumChildren())) // Due to exploration.
	sigmaPrime += (1.0 - f) * p                           // Due to strategy.
	var util float32
	if player == 0 {
		util = c.runHelper(child, player, p*reachP0, reachP1, sigmaPrime*reachSigmaPrime)
	} else {
		util = c.runHelper(child, player, reachP0, p*reachP1, sigmaPrime*reachSigmaPrime)
	}

	advantages := c.slicePool.alloc(node.NumChildren())
	defer c.slicePool.free(advantages)
	advantages[selectedAction] = util
	// Transform action utilities into instantaneous advantages by
	// subtracting out the expected utility over all possible actions.
	f32.AddConst(-p*util, advantages)
	// Don't use reachChance here, we are using it to store sigma' rather than chance probs.
	reachP := reachProb(player, reachP0, reachP1, 1.0)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, 1.0)
	strat.AddRegret(reachP, counterFactualP, advantages)
	return p * util
}

func sampleOne(strat NodeStrategy, numActions int) int {
	x := rand.Float32()
	var cumProb float32
	for i := 0; i < numActions; i++ {
		p := strat.GetActionProbability(i)
		cumProb += p
		if cumProb > x {
			return i
		}
	}

	if cumProb < 1.0-eps { // Leave room for floating point error.
		panic("probability distribution does not sum to 1!")
	}

	return numActions - 1
}
