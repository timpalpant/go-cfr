package cfr

// SampledActionStore records sampled player actions during a run of External Sampling CFR.
type SampledActionStore interface {
	Get(key string) (int, bool)
	Put(key string, sampledAction int)
}

type SampledActionMap map[string]int

func (m SampledActionMap) Get(key string) (int, bool) {
	i, ok := m[key]
	return i, ok
}

func (m SampledActionMap) Put(key string, i int) {
	m[key] = i
}
