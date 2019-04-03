package cfr

import (
	"io"

	"github.com/timpalpant/go-cfr/internal/sampling"
)

// SampledActions records sampled player actions during a run of External Sampling CFR.
type SampledActions interface {
	io.Closer

	// Get returns the sampled action for the player at the given node.
	// If this node has not yet been sampled, then an action is sampled from
	// the current strategy of the given policy. Future calls to Get with
	// the same node will return the same sampled action.
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
