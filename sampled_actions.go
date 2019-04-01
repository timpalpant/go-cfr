package cfr

import (
	"io"

	"github.com/timpalpant/go-cfr/internal/sampling"
)

// SampledActions records sampled player actions during a run of External Sampling CFR.
type SampledActions interface {
	io.Closer

	Get(node GameTreeNode, policy NodePolicy) int
}

type SampledActionsFactory func() SampledActions

type SampledActionsMap map[string]int

func NewSampledActionsMap() SampledActions {
	return make(SampledActionsMap)
}

func (m SampledActionsMap) Get(node GameTreeNode, policy NodePolicy) int {
	key := node.InfoSet(node.Player()).Key()
	i, ok := m[key]
	if !ok {
		i = sampling.SampleOne(policy.GetStrategy())
		m[key] = i
	}

	return i
}

func (m SampledActionsMap) Close() error {
	return nil
}
