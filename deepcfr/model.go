package deepcfr

import (
	"github.com/timpalpant/go-cfr"
)

type modelStrategyProfile struct {
	model TrainedModel
}

func (m *modelStrategyProfile) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &modelBasedPolicy{
		node:  node,
		model: m.model,
	}
}

func (m *modelStrategyProfile) Update() {
	panic("cannot update a model-based policy")
}

func (m *modelStrategyProfile) Iter() int { return 0 }

func (m *modelStrategyProfile) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (m *modelStrategyProfile) UnmarshalBinary(buf []byte) error {
	return nil
}

func (m *modelStrategyProfile) Close() error {
	return nil
}

type modelBasedPolicy struct {
	node     cfr.GameTreeNode
	model    TrainedModel
	strategy []float32
}

func (d *modelBasedPolicy) AddRegret(w float32, instantaneousRegrets []float32) {
	panic("cannot add regret to sampled SD-CFR model")
}

func (d *modelBasedPolicy) GetStrategy() []float32 {
	if d.strategy == nil {
		if d.model == nil {
			d.strategy = uniformDist(d.node.NumChildren())
		} else {
			infoSet := d.node.InfoSet(d.node.Player())
			d.strategy = d.model.Predict(infoSet, d.node.NumChildren())
		}
	}

	return d.strategy
}

func (d *modelBasedPolicy) GetBaseline() []float32 {
	return make([]float32, d.node.NumChildren())
}

func (d *modelBasedPolicy) SetBaseline(v []float32) {
	panic("cannot update baseline of sampled SD-CFR model")
}

func (d *modelBasedPolicy) AddStrategyWeight(w float32) {
	panic("cannot add strategy weight to sampled SD-CFR model")
}

func (d *modelBasedPolicy) GetAverageStrategy() []float32 {
	return d.GetStrategy()
}
