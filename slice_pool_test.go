package cfr

import (
	"testing"
)

// BenchmarkAllocFree-24              	200000000	         7.79 ns/op
func BenchmarkAllocFree(b *testing.B) {
	pool := &floatSlicePool{}
	for i := 0; i < b.N; i++ {
		v := pool.alloc(10)
		pool.free(v)
	}
}

// BenchmarkThreadSafeAllocFree-24    	30000000	        42.9 ns/op
func BenchmarkThreadSafeAllocFree(b *testing.B) {
	pool := &threadSafeFloatSlicePool{}
	for i := 0; i < b.N; i++ {
		v := pool.alloc(10)
		pool.free(v)
	}
}

// BenchmarkThreadSafeAllocFree_Parallel-24    	 5000000	       327 ns/op
func BenchmarkThreadSafeAllocFree_Parallel(b *testing.B) {
	pool := &threadSafeFloatSlicePool{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v := pool.alloc(10)
			pool.free(v)
		}
	})
}
