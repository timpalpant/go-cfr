package deepcfr

import (
	"math/rand"
	"sync"
)

type randPool []*lockedRand

func newRandPool(n int) randPool {
	rngs := make([]*lockedRand, n)
	for i := range rngs {
		rngs[i] = &lockedRand{
			rng: rand.New(rand.NewSource(rand.Int63())),
		}
	}

	return randPool(rngs)
}

func (r randPool) Intn(n int) int {
	k := n % len(r)
	return r[k].Intn(n)
}

type lockedRand struct {
	mx  sync.Mutex
	rng *rand.Rand
}

func (lr *lockedRand) Intn(n int) int {
	lr.mx.Lock()
	result := lr.rng.Intn(n)
	lr.mx.Unlock()
	return result
}
