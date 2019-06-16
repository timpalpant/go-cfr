package sampling

import (
	"fmt"
	"math/rand"

	"github.com/timpalpant/go-cfr"
)

const tol = 1e-3

// Sample one child of the given Chance node, according to its probability distribution.
func SampleChanceNode(node cfr.GameTreeNode) (cfr.GameTreeNode, float64) {
	x := rand.Float64()
	var cumProb float64
	n := node.NumChildren()
	for i := 0; i < n; i++ {
		p := node.GetChildProbability(i)
		cumProb += p
		if cumProb > x {
			return node.GetChild(i), p
		}
	}

	if cumProb < 1.0-tol { // Leave room for floating point error.
		panic(fmt.Errorf("probability distribution sums to %v != 1! node: %v, num children: %v",
			cumProb, node, n))
	}

	return node.GetChild(n - 1), node.GetChildProbability(n - 1)
}

// SampleOne returns the first element i of pv where sum(pv[:i]) > x.
func SampleOne(pv []float32, x float32) int {
	var cumProb float32
	for i, p := range pv {
		cumProb += p
		if cumProb > x {
			return i
		}
	}

	if cumProb < 1.0-tol { // Leave room for floating point error.
		panic(fmt.Errorf("probability distribution does not sum to 1! x=%v, pv=%v", x, pv))
	}

	return len(pv) - 1
}

func extend(v []float32, n int) []float32 {
	if n > len(v) {
		needed := n - len(v)
		return append(v, make([]float32, needed)...)
	}

	return v[:n]
}
