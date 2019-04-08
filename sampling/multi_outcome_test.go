package sampling

import (
	"testing"
)

func TestChooseK(t *testing.T) {
	p0 := []float32{0.01, 0.1, 0.1, 0.79}
	eps := float32(0.05)

	for k := 1; k <= 4; k++ {
		s := NewMultiOutcomeSampler(k, eps)
		pk := s.chooseK(p0)
		t.Logf("k=%d: %v", k, pk)
	}
}
