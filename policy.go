package cfr

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type policyStore struct {
	strategyProfile map[int]map[string]*policy
}

func newPolicyStore() *policyStore {
	return &policyStore{
		strategyProfile: map[int]map[string]*policy{
			0: make(map[string]*policy),
			1: make(map[string]*policy),
		},
	}
}

func (ps *policyStore) GetPolicy(node GameTreeNode) NodePolicy {
	p := node.Player()
	is := node.InfoSet(p)
	key := is.Key()

	if policy, ok := ps.strategyProfile[p][key]; ok {
		if policy.numActions() != node.NumChildren() {
			panic(fmt.Errorf("policy has n_actions=%v but node has n_children=%v: %v",
				policy.numActions(), node.NumChildren(), node))
		}
		return policy
	}

	policy := newPolicy(node.NumChildren())
	ps.strategyProfile[p][key] = policy
	if len(ps.strategyProfile[p])%100000 == 0 {
		glog.V(2).Infof("Player %d - %d infosets", p, len(ps.strategyProfile[p]))
	}

	return policy
}

type policy struct {
	reachProb   float32
	regretSum   []float32
	strategy    []float32
	strategySum []float32
}

func newPolicy(nActions int) *policy {
	return &policy{
		reachProb:   0.0,
		regretSum:   make([]float32, nActions),
		strategy:    uniformDist(nActions),
		strategySum: make([]float32, nActions),
	}
}

func (p *policy) GetActionProbability(i int) float32 {
	return p.strategy[i]
}

func (p *policy) NextStrategy(discountPositiveRegret, discountNegativeRegret, discountStrategySum float32) {
	if discountStrategySum != 1.0 {
		f32.ScalUnitary(discountStrategySum, p.strategySum)
	}

	f32.AxpyUnitary(p.reachProb, p.strategy, p.strategySum)

	if discountPositiveRegret != 1.0 {
		for i, x := range p.regretSum {
			if x > 0 {
				p.regretSum[i] *= discountPositiveRegret
			}
		}
	}

	if discountNegativeRegret != 1.0 {
		for i, x := range p.regretSum {
			if x < 0 {
				p.regretSum[i] *= discountNegativeRegret
			}
		}
	}

	p.calcStrategy()
	p.reachProb = 0.0
}

func (p *policy) numActions() int {
	return len(p.strategy)
}

func (p *policy) calcStrategy() {
	copy(p.strategy, p.regretSum)
	makePositive(p.strategy)
	total := f32.Sum(p.strategy)
	if total > 0 {
		f32.ScalUnitary(1.0/total, p.strategy)
		return
	}

	for i := range p.strategy {
		p.strategy[i] = 1.0 / float32(len(p.strategy))
	}
}

func (p *policy) GetAverageStrategy() []float32 {
	total := f32.Sum(p.strategySum)
	if total > 0 {
		avgStrat := make([]float32, len(p.strategySum))
		f32.ScalUnitaryTo(avgStrat, 1.0/total, p.strategySum)
		return avgStrat
	}

	return uniformDist(len(p.strategy))
}

func (p *policy) AddRegret(reachProb float32, instantaneousRegrets []float32) {
	p.reachProb += reachProb
	f32.Add(p.regretSum, instantaneousRegrets)
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}
