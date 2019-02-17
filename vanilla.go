package cfr

import (
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/golang/glog"
)

type Vanilla struct {
	mu sync.Mutex
	// Map of player -> InfoSet -> Strategy for that InfoSet.
	strategyProfile map[int]map[string]*policy
	slicePool       sync.Pool
}

var _ CFR = &Vanilla{}

func NewVanilla() *Vanilla {
	return &Vanilla{
		strategyProfile: map[int]map[string]*policy{
			0: make(map[string]*policy),
			1: make(map[string]*policy),
		},
		slicePool: sync.Pool{
			New: func() interface{} {
				return []float64(nil)
			},
		},
	}
}

func (v *Vanilla) GetStrategy(player int, infoSet string) []float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	policy := v.strategyProfile[player][infoSet]
	if policy == nil {
		return nil
	}

	return policy.getAverageStrategy()
}

type work struct {
	node GameTreeNode
	p    float64
}

func (v *Vanilla) Run(node GameTreeNode) float64 {
	expectedValue := v.runHelper(node, 1.0, 1.0, 1.0, 1)
	v.nextStrategyProfile()
	return expectedValue
}

func (v *Vanilla) nextStrategyProfile() {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, policies := range v.strategyProfile {
		for _, p := range policies {
			p.nextStrategy()
		}
	}
}

func (v *Vanilla) runHelper(node GameTreeNode, reachP0, reachP1, reachChance float64, depth int) float64 {
	if depth < 10 {
		glog.Infof("[depth=%d] %v", depth, node)
	}

	switch node.Type() {
	case TerminalNode:
		return node.Utility(node.Player())
	case ChanceNode:
		return v.handleChanceNode(node, reachP0, reachP1, reachChance, depth)
	default:
		return v.handlePlayerNode(node, reachP0, reachP1, reachChance, depth)
	}
}

func (v *Vanilla) handleChanceNode(node GameTreeNode, reachP0, reachP1, reachChance float64, depth int) float64 {
	expectedValue := 0.0
	n := 0
	node.VisitChildren(func(child GameTreeNode, p float64) {
		expectedValue += v.runHelper(child, reachP0, reachP1, reachChance*p, depth+1)
		n++
	})

	return expectedValue / float64(n)
}

func (v *Vanilla) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float64, depth int) float64 {
	player := node.Player()
	actionUtils := v.slicePool.Get().([]float64)
	defer v.slicePool.Put(actionUtils[:0])
	node.VisitChildren(func(child GameTreeNode, p float64) {
		var u float64
		if player == 0 {
			u = -1 * v.runHelper(child, p*reachP0, reachP1, reachChance, depth+1)
		} else {
			u = -1 * v.runHelper(child, reachP0, p*reachP1, reachChance, depth+1)
		}

		actionUtils = append(actionUtils, u)
	})

	policy := v.getPolicy(node, len(actionUtils))
	reachP := reachProb(player, reachP0, reachP1, reachChance)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	return policy.update(actionUtils, reachP, counterFactualP)
}

func (v *Vanilla) getPolicy(node GameTreeNode, nActions int) *policy {
	p := node.Player()
	is := node.InfoSet(p)

	v.mu.Lock()
	defer v.mu.Unlock()
	if policy, ok := v.strategyProfile[p][is]; ok {
		if policy.numActions() != nActions {
			b64IS := base64.StdEncoding.EncodeToString([]byte(is))
			panic(fmt.Errorf("policy has n_actions=%v but node has n_children=%v: %v - %v",
				policy.numActions(), nActions, node, b64IS))
		}
		return policy
	}

	policy := newPolicy(nActions)
	v.strategyProfile[p][is] = policy
	if len(v.strategyProfile[p])%100000 == 0 {
		glog.Infof("Player %d - %d infosets", p, len(v.strategyProfile[p]))
	}
	return policy
}
