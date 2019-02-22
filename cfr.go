package cfr

import (
	"fmt"
	"math/rand"

	"github.com/golang/glog"
)

type Params struct {
	SampleChanceNodes     bool // Chance Sampling
	SamplePlayerActions   bool // Outcome Sampling
	SampleOpponentActions bool // External Sampling
	UseRegretMatchingPlus bool // CFR+
	// Strategy probabilities below this value will be set to zero.
	PurificationThreshold float32
}

type CFR struct {
	params Params

	iter int

	// Map of player -> InfoSet -> Strategy for that InfoSet.
	strategyProfile map[int]map[string]*policy
	slicePool       *floatSlicePool
}

func New(params Params) *CFR {
	return &CFR{
		params: params,
		strategyProfile: map[int]map[string]*policy{
			0: make(map[string]*policy),
			1: make(map[string]*policy),
		},
		slicePool: &floatSlicePool{},
	}
}

func (c *CFR) GetStrategy(player int, infoSet string) []float32 {
	policy := c.strategyProfile[player][infoSet]
	if policy == nil {
		return nil
	}

	return policy.getAverageStrategy()
}

func (c *CFR) Run(node GameTreeNode) float32 {
	expectedValue := c.runHelper(node, 1.0, 1.0, 1.0)
	c.nextStrategyProfile()
	return expectedValue
}

func (c *CFR) nextStrategyProfile() {
	for _, policies := range c.strategyProfile {
		for _, p := range policies {
			p.nextStrategy()
		}
	}

	c.iter++
}

func (c *CFR) runHelper(node GameTreeNode, reachP0, reachP1, reachChance float32) float32 {
	node.BuildChildren()

	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = node.Utility(node.Player())
	case ChanceNode:
		ev = c.handleChanceNode(node, reachP0, reachP1, reachChance)
	default:
		ev = c.handlePlayerNode(node, reachP0, reachP1, reachChance)
	}

	node.FreeChildren()
	return ev
}

func (c *CFR) handleChanceNode(node GameTreeNode, reachP0, reachP1, reachChance float32) float32 {
	if c.params.SampleChanceNodes {
		child := node.SampleChild()
		return c.runHelper(child, reachP0, reachP1, reachChance)
	}

	var expectedValue float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := node.GetChildProbability(i)
		expectedValue += c.runHelper(child, reachP0, reachP1, reachChance*p)
	}

	return expectedValue / float32(node.NumChildren())
}

func (c *CFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float32) float32 {
	if node.NumChildren() == 1 { // Fast path for trivial nodes with no real choice.
		child := node.GetChild(0)
		return -1 * c.runHelper(child, reachP0, reachP1, reachChance)
	}

	player := node.Player()
	policy := c.getPolicy(node)
	if c.params.SampleOpponentActions && c.iter%2 != player {
		// Sample according to current strategy profile.
		x := rand.Float32()
		var cumProb float32
		for i, p := range policy.strategy {
			cumProb += p
			if cumProb > x {
				child := node.GetChild(i)
				if player == 0 {
					return -1 * c.runHelper(child, p*reachP0, reachP1, reachChance)
				} else {
					return -1 * c.runHelper(child, reachP0, p*reachP1, reachChance)
				}
			}
		}

		panic("unreachable code")
	}

	actionUtils := c.slicePool.alloc(node.NumChildren())
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := policy.strategy[i]
		if player == 0 {
			actionUtils[i] = -1 * c.runHelper(child, p*reachP0, reachP1, reachChance)
		} else {
			actionUtils[i] = -1 * c.runHelper(child, reachP0, p*reachP1, reachChance)
		}
	}

	reachP := reachProb(player, reachP0, reachP1, reachChance)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	cfUtility := policy.update(actionUtils, reachP, counterFactualP)
	c.slicePool.free(actionUtils)
	return cfUtility
}

func (c *CFR) getPolicy(node GameTreeNode) *policy {
	p := node.Player()
	is := node.InfoSet(p)

	if policy, ok := c.strategyProfile[p][is]; ok {
		if policy.numActions() != node.NumChildren() {
			panic(fmt.Errorf("policy has n_actions=%v but node has n_children=%v: %v",
				policy.numActions(), node.NumChildren(), node))
		}
		return policy
	}

	policy := newPolicy(node.NumChildren())
	c.strategyProfile[p][is] = policy
	if len(c.strategyProfile[p])%100000 == 0 {
		glog.V(2).Infof("Player %d - %d infosets", p, len(c.strategyProfile[p]))
	}
	return policy
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
