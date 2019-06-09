package deepcfr

import (
	"bytes"
	"encoding/gob"
	"sync"
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
	n           int
	rngPool     randPool
}

// NewBuffer returns an empty Buffer with the given max size.
func NewReservoirBuffer(maxSize, maxParallel int) *ReservoirBuffer {
	return &ReservoirBuffer{
		maxSize:     maxSize,
		maxParallel: maxParallel,
		samples:     make([]Sample, 0, maxSize),
		rngPool:     newRandPool(2 * maxParallel),
	}
}

// AddSample implements Buffer.
func (b *ReservoirBuffer) AddSample(sample Sample) {
	b.mx.Lock()
	if b.n < b.maxSize {
		b.samples = append(b.samples, sample)
		b.n++
		b.mx.Unlock()
		return
	}

	// Rand is slow and at steady-state most of the time we will discard the sample.
	// So we unlock now and if we need to relock to store it; this is faster on average.
	n := b.n
	b.n++
	b.mx.Unlock()

	if m := b.rngPool.Intn(n); m < b.maxSize {
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
	b.mx.Lock()
	defer b.mx.Unlock()
	return len(b.samples)
}

// GetSamples implements Buffer.
func (b *ReservoirBuffer) GetSamples() []Sample {
	b.mx.Lock()
	defer b.mx.Unlock()
	result := make([]Sample, len(b.samples))
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
