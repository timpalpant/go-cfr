package sampling

import (
	"math/rand"

	"github.com/timpalpant/go-cfr"
)

// OutcomeSampler implements cfr.Sampler by sampling one player action
// according to the current strategy.
type OutcomeSampler struct {
	rng *rand.Rand
	p   []float32
}

func NewOutcomeSampler() *OutcomeSampler {
	return &OutcomeSampler{
		rng: rand.New(rand.NewSource(rand.Int63())),
	}
}

func (os *OutcomeSampler) Sample(node cfr.GameTreeNode, policy cfr.NodePolicy) []float32 {
	p := policy.GetStrategy()
	x := os.rng.Float32()
	selected := SampleOne(p, x)

	os.p = extend(os.p, len(p))
	for i := range os.p {
		os.p[i] = 0 // memclr
	}

	os.p[selected] = p[selected]
	return os.p
}
