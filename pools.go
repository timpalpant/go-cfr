package cfr

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

type keyIntMapPool struct {
	pool []map[string]int
}

func (p *keyIntMapPool) alloc() map[string]int {
	if len(p.pool) > 0 {
		m := len(p.pool)
		next := p.pool[m-1]
		p.pool = p.pool[:m-1]
		return next
	}

	return make(map[string]int)
}

func (p *keyIntMapPool) free(m map[string]int) {
	for k := range m {
		delete(m, k)
	}

	p.pool = append(p.pool, m)
}
