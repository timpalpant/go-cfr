package cfr

import (
	"testing"
)

// BenchmarkFloatSlicePoolAllocFree-24      	200000000	         9.63 ns/op
func BenchmarkFloatSlicePoolAllocFree(b *testing.B) {
	pool := &floatSlicePool{}
	for i := 0; i < b.N; i++ {
		v := pool.alloc(10)
		pool.free(v)
	}
}

// BenchmarkKeyIntMapPoolAllocFree-24    	200000000	         7.99 ns/op
func BenchmarkKeyIntMapPoolAllocFree(b *testing.B) {
	pool := &keyIntMapPool{}
	for i := 0; i < b.N; i++ {
		v := pool.alloc()
		pool.free(v)
	}
}
