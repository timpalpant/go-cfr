package sampling

import (
	"math/rand"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
)

// MultiOutcomeSampler implements cfr.Sampler by sampling at most k player actions
// with probability according to the current strategy.
type MultiOutcomeSampler struct {
	k    int
	eps  float32
	rng  *rand.Rand
	p    []float32
	pool *floatSlicePool
}

func NewMultiOutcomeSampler(k int, explorationEps float32) *MultiOutcomeSampler {
	return &MultiOutcomeSampler{
		k:    k,
		eps:  explorationEps,
		rng:  rand.New(rand.NewSource(rand.Int63())),
		p:    make([]float32, k),
		pool: &floatSlicePool{},
	}
}

func (os *MultiOutcomeSampler) Sample(node cfr.GameTreeNode, policy cfr.NodePolicy) []float32 {
	nChildren := node.NumChildren()
	os.p = extend(os.p, nChildren)
	if nChildren <= os.k {
		for i := range os.p[:nChildren] {
			os.p[i] = 1.0
		}

		return os.p[:nChildren]
	}

	// p is the result to return.
	// q is the p-vector to sample from next.
	// r is a p-vector of the k-sampling probabilities.
	// We copy r[i] into p[i] for the k sampled actions.
	for i := range os.p {
		os.p[i] = 0 // memclr
	}

	q := os.pool.alloc(nChildren)
	copy(q, policy.GetStrategy())
	f32.AddConst(os.eps, q)
	f32.ScalUnitary(1.0/(1.0+float32(nChildren)*eps), q) // Renormalize.

	// Compute probability of choosing i if we draw k times.
	qEff := os.chooseK(q)

	for i := 0; i < os.k; i++ {
		sampled := SampleOne(q, os.rng.Float32())
		os.p[sampled] = qEff[sampled]

		// Remove sampled action from being re-sampled.
		qSample := q[sampled]
		q[sampled] = 0
		f32.ScalUnitary(1.0/(1.0-qSample), q)
	}

	os.pool.free(qEff)
	os.pool.free(q)
	return os.p
}

func (os *MultiOutcomeSampler) chooseK(p []float32) []float32 {
	result := os.pool.alloc(len(p))

	for j := range p {
		result[j] = os.chooseKHelper(p, j, os.k)
	}

	return result
}

func (os *MultiOutcomeSampler) chooseKHelper(p []float32, j, k int) float32 {
	if k == 1 {
		return p[j]
	}

	var descendant float32
	for i := range p {
		if i != j && p[i] > 0 {
			choseI := os.pool.alloc(len(p))
			copy(choseI, p)
			choseI[i] = 0
			f32.ScalUnitary(1.0/(1-p[i]), choseI)
			descendant += p[i] * os.chooseKHelper(choseI, j, k-1)
			os.pool.free(choseI)
		}
	}

	return p[j] + descendant
}
