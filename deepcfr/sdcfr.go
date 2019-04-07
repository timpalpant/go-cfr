package deepcfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr"
)

type TrajectorySampledSDCFR []TrainedModel

func (d TrajectorySampledSDCFR) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &sampledDCFRPolicy{
		node:  node,
		model: d[node.Player()],
	}
}

type sampledDCFRPolicy struct {
	node     cfr.GameTreeNode
	model    TrainedModel
	strategy []float32
}

func (d *sampledDCFRPolicy) AddRegret(w float32, instantaneousRegrets []float32) {
	panic("cannot add regret to sampled SD-CFR model")
}

func (d *sampledDCFRPolicy) GetStrategy() []float32 {
	if d.strategy == nil {
		infoSet := d.node.InfoSet(d.node.Player())
		d.strategy = d.model.Predict(infoSet, d.node.NumChildren())
	}

	return d.strategy
}

func (d *sampledDCFRPolicy) AddStrategyWeight(w float32) {
	panic("cannot add strategy weight to sampled SD-CFR model")
}

func (d *sampledDCFRPolicy) GetAverageStrategy() []float32 {
	return d.GetStrategy()
}

func sampleModels(models [][]TrainedModel) []TrainedModel {
	result := make([]TrainedModel, len(models))
	for i, candidates := range models {
		selected := sampleModel(candidates)
		result[i] = selected
	}

	return result
}

func sampleModel(candidates []TrainedModel) TrainedModel {
	m := len(candidates) * (len(candidates) - 1) / 2
	x := rand.Intn(m) + 1
	n := 0
	for i := range candidates {
		n += i + 1
		if n >= x {
			return candidates[i]
		}
	}

	return nil
}
