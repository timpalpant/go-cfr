package deepcfr

import (
	"bytes"
	"encoding/gob"
	"sync"
	"sync/atomic"
)

// ReservoirBuffer is a collection of samples held in memory.
// One the buffer's max size is reached, additional
// samples are added via reservoir sampling, maintaining
// a uniform distribution over all previous values.
//
// It is safe to call AddSample concurrently from multiple goroutines.
// GetSamples does not copy the underlying slice of samples, and therefore
// is not safe to call concurrently with AddSample.
type ReservoirBuffer struct {
	mx          sync.Mutex
	maxSize     int
	maxParallel int
	samples     []Sample
	n           int64
	rngPool     randPool
}

// NewBuffer returns an empty Buffer with the given max size.
func NewReservoirBuffer(maxSize, maxParallel int) *ReservoirBuffer {
	return &ReservoirBuffer{
		maxSize:     maxSize,
		maxParallel: maxParallel,
		samples:     make([]Sample, maxSize),
		rngPool:     newRandPool(2 * maxParallel),
	}
}

// AddSample implements Buffer.
func (b *ReservoirBuffer) AddSample(sample Sample) {
	// We a are a little bit sloppy here for improved performance:
	// Because we do not hold a lock for the duration of the call, it is possible
	// for an earlier call to AddSample to collide and overwrite a later call
	// if both are simultaneously assigned to the same random bucket.
	n := int(atomic.AddInt64(&b.n, 1))

	if n <= b.maxSize {
		b.mx.Lock()
		b.samples[n-1] = sample
		b.mx.Unlock()
	} else if m := b.rngPool.Intn(n); m < b.maxSize {
		b.mx.Lock()
		b.samples[m] = sample
		b.mx.Unlock()
	}
}

// GetSample implements Buffer.
func (b *ReservoirBuffer) GetSample(idx int) Sample {
	b.mx.Lock()
	defer b.mx.Unlock()
	return b.samples[idx]
}

func (b *ReservoirBuffer) Len() int {
	return int(atomic.LoadInt64(&b.n))
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

	if err := enc.Encode(b.maxParallel); err != nil {
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

	if err := dec.Decode(&b.maxParallel); err != nil {
		return err
	}

	if err := dec.Decode(&b.n); err != nil {
		return err
	}

	if err := dec.Decode(&b.samples); err != nil {
		return err
	}

	b.rngPool = newRandPool(b.maxParallel)
	return nil
}

func init() {
	gob.Register(&ReservoirBuffer{})
}
