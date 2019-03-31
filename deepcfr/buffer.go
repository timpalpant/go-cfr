package deepcfr

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"sync"

	"github.com/timpalpant/go-cfr"
)

// ReservoirBuffer is a collection of samples held in memory.
// One the buffer's max size is reached, additional
// samples are added via reservoir sampling, maintaining
// a uniform distribution over all previous values.
type ReservoirBuffer struct {
	maxSize int
	samples []Sample
	n       int
}

// NewBuffer returns an empty Buffer with the given max size.
func NewReservoirBuffer(maxSize int) *ReservoirBuffer {
	return &ReservoirBuffer{
		maxSize: maxSize,
	}
}

// AddSample implements Buffer.
func (b *ReservoirBuffer) AddSample(infoSet cfr.InfoSet, advantages []float32, weight float32) {
	b.n++

	if len(b.samples) < b.maxSize {
		sample := NewSample(infoSet, advantages, weight)
		b.samples = append(b.samples, sample)
	} else {
		m := rand.Intn(b.n)
		if m < b.maxSize {
			b.samples[m] = NewSample(infoSet, advantages, weight)
		}
	}
}

// GetSamples implements Buffer.
func (b *ReservoirBuffer) GetSamples() []Sample {
	return b.samples
}

func (b *ReservoirBuffer) Close() error {
	return nil
}

// Cap returns the max number of samples that will be kept in the buffer.
func (b *ReservoirBuffer) Cap() int {
	return b.maxSize
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

// ThreadSafeReservoirBuffer wraps ReservoirBuffer to be safe for use
// from multiple goroutines.
type ThreadSafeReservoirBuffer struct {
	mu  sync.Mutex
	buf ReservoirBuffer
}

// AddSample implements Buffer.
func (b *ThreadSafeReservoirBuffer) AddSample(infoSet cfr.InfoSet, advantages []float32, weight float32) {
	b.mu.Lock()
	b.buf.AddSample(infoSet, advantages, weight)
	b.mu.Unlock()
}

// GetSamples implements Buffer.
func (b *ThreadSafeReservoirBuffer) GetSamples() []Sample {
	b.mu.Lock()
	samples := b.buf.GetSamples()
	b.mu.Unlock()
	return samples
}

func (b *ThreadSafeReservoirBuffer) Close() error {
	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (b *ThreadSafeReservoirBuffer) MarshalBinary() ([]byte, error) {
	return b.buf.MarshalBinary()
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (b *ThreadSafeReservoirBuffer) UnmarshalBinary(buf []byte) error {
	return b.buf.UnmarshalBinary(buf)
}

// NewThreadSafeReservoirBuffer creates a new reservoir buffer with the
// given max capacity that is safe for use from multiple goroutines.
func NewThreadSafeReservoirBuffer(maxSize int) *ThreadSafeReservoirBuffer {
	return &ThreadSafeReservoirBuffer{
		buf: ReservoirBuffer{maxSize: maxSize},
	}
}

func init() {
	gob.Register(&ReservoirBuffer{})
	gob.Register(&ThreadSafeReservoirBuffer{})
}
