package sampling

type floatSlicePool struct {
	pool [][]float32
}

func (p *floatSlicePool) alloc(n int) []float32 {
	if len(p.pool) > 0 {
		m := len(p.pool)
		next := p.pool[m-1]
		p.pool = p.pool[:m-1]
		return append(next, make([]float32, n)...)
	}

	return make([]float32, n)
}

func (p *floatSlicePool) free(s []float32) {
	if cap(s) > 0 {
		p.pool = append(p.pool, s[:0])
	}
}
