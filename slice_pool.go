package cfr

import (
	"sync"
)

type floatSlicePool struct {
	pool [][]float32
}

func (p *floatSlicePool) alloc(n int) []float32 {
	if p == nil {
		return make([]float32, n)
	}

	if len(p.pool) > 0 {
		m := len(p.pool)
		next := p.pool[m-1]
		p.pool = p.pool[:m-1]
		return append(next, make([]float32, n)...)
	}

	return make([]float32, n)
}

func (p *floatSlicePool) free(s []float32) {
	if p != nil && cap(s) > 0 {
		p.pool = append(p.pool, s[:0])
	}
}

type threadSafeFloatSlicePool struct {
	mu   sync.Mutex
	pool [][]float32
}

func (p *threadSafeFloatSlicePool) alloc(n int) []float32 {
	if len(p.pool) > 0 {
		p.mu.Lock()
		m := len(p.pool)
		next := p.pool[m-1]
		p.pool = p.pool[:m-1]
		p.mu.Unlock()

		return append(next, make([]float32, n)...)
	}

	return make([]float32, n)
}

func (p *threadSafeFloatSlicePool) free(s []float32) {
	if cap(s) > 0 {
		p.mu.Lock()
		p.pool = append(p.pool, s[:0])
		p.mu.Unlock()
	}
}
