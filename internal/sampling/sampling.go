package sampling

import (
	"math/rand"
)

const eps = 1e-3

func SampleOne(pv []float32) int {
	x := rand.Float32()
	var cumProb float32
	for i, p := range pv {
		cumProb += p
		if cumProb > x {
			return i
		}
	}

	if cumProb < 1.0-eps { // Leave room for floating point error.
		panic("probability distribution does not sum to 1!")
	}

	return len(pv) - 1
}
