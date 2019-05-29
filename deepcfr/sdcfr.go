package deepcfr

import (
	"math/rand"

	"github.com/golang/glog"

	"github.com/timpalpant/go-cfr"
)

type TrajectorySampledSDCFR []TrainedModel

func (d TrajectorySampledSDCFR) Close() error {
	return nil
}

func (d TrajectorySampledSDCFR) Iter() int {
	return -1
}

func (d TrajectorySampledSDCFR) Update() {}

func (d TrajectorySampledSDCFR) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &modelBasedPolicy{
		node:  node,
		model: d[node.Player()],
	}
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
	m := len(candidates) * (len(candidates) + 1) / 2
	x := rand.Intn(m) + 1
	n := 0
	for i := range candidates {
		n += i + 1
		if n >= x {
			glog.V(3).Infof("Sampled model %d (out of %d)", i, len(candidates))
			return candidates[i]
		}
	}

	return nil
}
