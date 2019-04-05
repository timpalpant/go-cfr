package deepcfr

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/timpalpant/go-cfr"
)

type randPool []*rand.Rand

func newRandPool(n int) randPool {
	rngs := make([]*rand.Rand, n)
	for i := range rngs {
		rngs[i] = rand.New(rand.NewSource(rand.Int63()))
	}

	return randPool(rngs)
}

func (r randPool) Intn(n int) int {
	k := n % len(r)
	return r[k].Intn(n)
}

// ReservoirBuffer is a collection of samples held in memory.
// One the buffer's max size is reached, additional
// samples are added via reservoir sampling, maintaining
// a uniform distribution over all previous values.
//
// It is safe to call AddSample concurrently from multiple goroutines.
// GetSamples does not copy the underlying slice of samples, and therefore
// is not safe to call concurrently with AddSample.
type ReservoirBuffer struct {
	mx      sync.Mutex
	maxSize int
	// "Better to eat the extra cost of a few bytes per Sample,
	// than to starve on the GC of a million pointers."
	//    - Go Proverb
	samples []Sample
	n       int64
	rngPool randPool
}

// NewBuffer returns an empty Buffer with the given max size.
func NewReservoirBuffer(maxSize, maxParallel int) *ReservoirBuffer {
	return &ReservoirBuffer{
		maxSize: maxSize,
		samples: make([]Sample, 0, maxSize),
		// RNG pool needs to be 2x because we otherwise we might collide
		// as the pool entry wraps around.
		rngPool: newRandPool(2 * maxParallel),
	}
}

// AddSample implements Buffer.
func (b *ReservoirBuffer) AddSample(node cfr.GameTreeNode, advantages []float32, weight float32) {
	// We a are a little bit sloppy here for improved performance:
	// Because we do not hold a lock for the duration of the call, it is possible
	// for an earlier call to AddSample to collide and overwrite a later call
	// if both are simultaneously assigned to the same random bucket.
	n := int(atomic.AddInt64(&b.n, 1))

	if n <= b.maxSize {
		sample := NewSample(node, advantages, weight)
		b.mx.Lock()
		b.samples = append(b.samples, sample)
		b.mx.Unlock()
	} else {
		m := b.rngPool.Intn(n)
		if m < b.maxSize {
			sample := NewSample(node, advantages, weight)
			b.mx.Lock()
			b.samples[m] = sample
			b.mx.Unlock()
		}
	}
}

// GetSamples implements Buffer.
func (b *ReservoirBuffer) GetSamples() []Sample {
	n := int(atomic.LoadInt64(&b.n))
	nSamples := min(n, b.maxSize)
	result := make([]Sample, nSamples)

	b.mx.Lock()
	defer b.mx.Unlock()
	copy(result, b.samples)

	return result
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func (b *ReservoirBuffer) Close() error {
	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (b *ReservoirBuffer) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(b.maxSize); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.n); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.samples); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (b *ReservoirBuffer) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&b.maxSize); err != nil {
		return err
	}

	if err := dec.Decode(&b.n); err != nil {
		return err
	}

	if err := dec.Decode(&b.samples); err != nil {
		return err
	}

	return nil
}

func init() {
	gob.Register(&ReservoirBuffer{})
}
