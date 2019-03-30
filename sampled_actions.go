package cfr

import "io"

// SampledActions records sampled player actions during a run of External Sampling CFR.
type SampledActions interface {
	io.Closer

	Get(key string) (int, bool)
	Put(key string, sampledAction int)
}

type SampledActionsFactory func() SampledActions

type SampledActionsMap map[string]int

func NewSampledActionsMap() SampledActions {
	return make(SampledActionsMap)
}

func (m SampledActionsMap) Get(key string) (int, bool) {
	i, ok := m[key]
	return i, ok
}

func (m SampledActionsMap) Put(key string, i int) {
	m[key] = i
}

func (m SampledActionsMap) Close() error {
	return nil
}
