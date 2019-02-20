package cfr

import (
	"encoding/hex"
	"fmt"
	"math/rand"

	"github.com/golang/glog"
)

type Params struct {
	SampleChanceNodes bool
}

type CFR struct {
	sampleChanceNodes bool

	// Map of player -> InfoSet -> Strategy for that InfoSet.
	strategyProfile map[int]map[string]*policy
	slicePool       *floatSlicePool
}

func New(params Params) *CFR {
	return &CFR{
		sampleChanceNodes: params.SampleChanceNodes,
		strategyProfile: map[int]map[string]*policy{
			0: make(map[string]*policy),
			1: make(map[string]*policy),
		},
		slicePool: &floatSlicePool{},
	}
}

func (c *CFR) GetStrategy(player int, infoSet string) []float64 {
	policy := c.strategyProfile[player][infoSet]
	if policy == nil {
		return nil
	}

	return policy.getAverageStrategy()
}

func (c *CFR) Run(node GameTreeNode) float64 {
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
}

func (c *CFR) runHelper(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	node.BuildChildren()

	var ev float64
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

func (c *CFR) handleChanceNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	if c.sampleChanceNodes {
		x := rand.Float64()
		cumP := 0.0
		for i := 0; i < node.NumChildren(); i++ {
			child := node.GetChild(i)
			p := node.GetChildProbability(i)
			cumP += p
			if cumP > x {
				return c.runHelper(child, reachP0, reachP1, reachChance)
			}
		}
	} else {
		expectedValue := 0.0
		for i := 0; i < node.NumChildren(); i++ {
			child := node.GetChild(i)
			p := node.GetChildProbability(i)
			expectedValue += c.runHelper(child, reachP0, reachP1, reachChance*p)
		}

		return expectedValue / float64(node.NumChildren())
	}

	panic("unreachable code")
}

func (c *CFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	player := node.Player()
	policy := c.getPolicy(node)
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
			b64IS := hex.EncodeToString([]byte(is))
			panic(fmt.Errorf("policy has n_actions=%v but node has n_children=%v: %v - %v",
				policy.numActions(), node.NumChildren(), node, b64IS))
		}
		return policy
	}

	policy := newPolicy(node.NumChildren())
	c.strategyProfile[p][is] = policy
	if len(c.strategyProfile[p])%100000 == 0 {
		glog.Infof("Player %d - %d infosets", p, len(c.strategyProfile[p]))
	}
	return policy
}