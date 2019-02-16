package cfr

type floatSlicePool struct {
	pool [][]float64
}

func (p *floatSlicePool) alloc(n int) []float64 {
	if p == nil {
		return make([]float64, n)
	}

	if len(p.pool) > 0 {
		m := len(p.pool)
		next := p.pool[m-1]
		p.pool = p.pool[:m-1]
		return append(next, make([]float64, n)...)
	}

	return make([]float64, n)
}

func (p *floatSlicePool) free(s []float64) {
	if p != nil && cap(s) > 0 {
		p.pool = append(p.pool, s[:0])
	}
}
