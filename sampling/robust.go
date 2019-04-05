package sampling

import (
	"math/rand"

	"github.com/timpalpant/go-cfr"
)

// RobustSampler implements cfr.Sampler by sampling a fixed number of actions
// uniformly randomly.
type RobustSampler struct {
	p   []float32
	k   int
	rng *rand.Rand
}

func NewRobustSampler(k int) *RobustSampler {
	return &RobustSampler{
		k:   k,
		rng: rand.New(rand.NewSource(rand.Int63())),
	}
}

func (rs *RobustSampler) Sample(node cfr.GameTreeNode, policy cfr.NodePolicy) []float32 {
	nChildren := node.NumChildren()
	rs.p = extend(rs.p, nChildren)

	if nChildren <= rs.k {
		for i := range rs.p[:rs.k] {
			rs.p[i] = 1.0
		}

		return rs.p[:rs.k]
	}

	for i := 0; i < rs.k; i++ {
		rs.p[i] = float32(rs.k) / float32(nChildren)
	}

	for i := rs.k; i < nChildren; i++ {
		rs.p[i] = 0
	}

	rs.rng.Shuffle(nChildren, func(i, j int) {
		rs.p[i], rs.p[j] = rs.p[j], rs.p[i]
	})

	return rs.p
}
