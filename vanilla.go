package cfr

import (
	"encoding/base64"
	"fmt"

	"github.com/golang/glog"
)

type Vanilla struct {
	// Map of player -> InfoSet -> Strategy for that InfoSet.
	strategyProfile map[int]map[string]*policy
	slicePool       *floatSlicePool
}

var _ CFR = &Vanilla{}

func NewVanilla() *Vanilla {
	return &Vanilla{
		strategyProfile: map[int]map[string]*policy{
			0: make(map[string]*policy),
			1: make(map[string]*policy),
		},
		slicePool: &floatSlicePool{},
	}
}

func (v *Vanilla) GetStrategy(player int, infoSet string) []float64 {
	policy := v.strategyProfile[player][infoSet]
	if policy == nil {
		return nil
	}

	return policy.getAverageStrategy()
}

func (v *Vanilla) Run(node GameTreeNode) float64 {
	expectedValue := v.runHelper(node, 1.0, 1.0, 1.0)
	v.nextStrategyProfile()
	return expectedValue
}

func (v *Vanilla) nextStrategyProfile() {
	for _, policies := range v.strategyProfile {
		for _, p := range policies {
			p.nextStrategy()
		}
	}
}

func (v *Vanilla) runHelper(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	node.BuildChildren()
	defer node.FreeChildren()

	switch node.Type() {
	case TerminalNode:
		return node.Utility(node.Player())
	case ChanceNode:
		return v.handleChanceNode(node, reachP0, reachP1, reachChance)
	default:
		return v.handlePlayerNode(node, reachP0, reachP1, reachChance)
	}
}

func (v *Vanilla) handleChanceNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	expectedValue := 0.0
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := node.GetChildProbability(i)
		expectedValue += v.runHelper(child, reachP0, reachP1, reachChance*p)
	}

	return expectedValue / float64(node.NumChildren())
}

func (v *Vanilla) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float64) float64 {
	player := node.Player()
	policy := v.getPolicy(node)
	actionUtils := v.slicePool.alloc(node.NumChildren())
	defer v.slicePool.free(actionUtils)
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := policy.strategy[i]
		if player == 0 {
			actionUtils[i] = -1 * v.runHelper(child, p*reachP0, reachP1, reachChance)
		} else {
			actionUtils[i] = -1 * v.runHelper(child, reachP0, p*reachP1, reachChance)
		}
	}

	reachP := reachProb(player, reachP0, reachP1, reachChance)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	return policy.update(actionUtils, reachP, counterFactualP)
}

func (v *Vanilla) getPolicy(node GameTreeNode) *policy {
	p := node.Player()
	is := node.InfoSet(p)

	if policy, ok := v.strategyProfile[p][is]; ok {
		if policy.numActions() != node.NumChildren() {
			b64IS := base64.StdEncoding.EncodeToString([]byte(is))
			panic(fmt.Errorf("policy has n_actions=%v but node has n_children=%v: %v - %v",
				policy.numActions(), node.NumChildren(), node, b64IS))
		}
		return policy
	}

	policy := newPolicy(node.NumChildren())
	v.strategyProfile[p][is] = policy
	if len(v.strategyProfile[p])%100000 == 0 {
		glog.Infof("Player %d - %d infosets", p, len(v.strategyProfile[p]))
	}
	return policy
}
