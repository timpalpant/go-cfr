package deepcfr

import (
	"math/rand"
	"sync/atomic"
	"testing"
)

// BenchmarkRandPool		30000000	        42.5 ns/op
// BenchmarkRandPool-4   	30000000	        43.0 ns/op
// BenchmarkRandPool-24    	20000000	        71.6 ns/op
// BenchmarkRandPool-256    20000000	        89.8 ns/op
func BenchmarkRandPool(b *testing.B) {
	pool := newRandPool(128)
	var n int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			k := int(atomic.AddInt64(&n, 1))
			pool.Intn(k)
		}
	})
}

// BenchmarkRand     	50000000	       38.8 ns/op
// BenchmarkRand-4      10000000	       169 ns/op
// BenchmarkRand-24      5000000	       273 ns/op
// BenchmarkRand-256    10000000	       224 ns/op
func BenchmarkRand(b *testing.B) {
	var n int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			k := int(atomic.AddInt64(&n, 1))
			rand.Intn(k)
		}
	})
}
