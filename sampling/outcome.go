package sampling

import (
	"math/rand"

	"github.com/timpalpant/go-cfr"
)

// OutcomeSampler implements cfr.Sampler by sampling one player action
// according to the current strategy.
type OutcomeSampler struct {
	eps float32
	rng *rand.Rand
	p   []float32
}

func NewOutcomeSampler(explorationEps float32) *OutcomeSampler {
	return &OutcomeSampler{
		eps: explorationEps,
		rng: rand.New(rand.NewSource(rand.Int63())),
	}
}

func (os *OutcomeSampler) Sample(node cfr.GameTreeNode, policy cfr.NodePolicy) []float32 {
	nChildren := node.NumChildren()

	var selected int
	p := policy.GetStrategy()
	if os.rng.Float32() < os.eps {
		selected = os.rng.Intn(nChildren)
	} else {
		selected = SampleOne(p, os.rng.Float32())
	}

	os.p = extend(os.p, nChildren)
	for i := range os.p {
		os.p[i] = 0 // memclr
	}

	q := os.eps * (1.0 / float32(nChildren)) // Sampled due to exploration.
	q += (1.0 - os.eps) * p[selected]        // Sampled due to strategy.

	os.p[selected] = q
	return os.p
}
