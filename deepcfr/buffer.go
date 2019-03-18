package deepcfr

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"sync"
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
func (b *ReservoirBuffer) AddSample(s Sample) {
	b.n++

	if len(b.samples) < b.maxSize {
		b.samples = append(b.samples, s)
	} else {
		m := rand.Intn(b.n)
		if m < b.maxSize {
			b.samples[m] = s
		}
	}
}

// GetSamples implements Buffer.
func (b *ReservoirBuffer) GetSamples() []Sample {
	return b.samples
}

// Cap returns the max number of samples that will be kept in the buffer.
func (b *ReservoirBuffer) Cap() int {
	return b.maxSize
}

// GobEncode implements gob.GobEncoder.
func (b *ReservoirBuffer) GobEncode() ([]byte, error) {
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

// GobEncode implements gob.GobDecoder.
func (b *ReservoirBuffer) GobDecode(buf []byte) error {
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
	mu sync.Mutex
	// FIXME: This should not be a pointer.
	buf *ReservoirBuffer
}

// AddSample implements Buffer.
func (b *ThreadSafeReservoirBuffer) AddSample(s Sample) {
	b.mu.Lock()
	b.buf.AddSample(s)
	b.mu.Unlock()
}

// GetSamples implements Buffer.
func (b *ThreadSafeReservoirBuffer) GetSamples() []Sample {
	b.mu.Lock()
	samples := b.buf.GetSamples()
	b.mu.Unlock()
	return samples
}

// GobEncode implements gob.GobEncoder.
func (b *ThreadSafeReservoirBuffer) GobEncode() ([]byte, error) {
	return b.buf.GobEncode()
}

// GobEncode implements gob.GobDecoder.
func (b *ThreadSafeReservoirBuffer) GobDecode(buf []byte) error {
	// Hack to instantiate empty reservoir buffer to perform decoding.
	b.buf = NewReservoirBuffer(1)
	return b.buf.GobDecode(buf)
}

// NewThreadSafeReservoirBuffer creates a new reservoir buffer with the
// given max capacity that is safe for use from multiple goroutines.
func NewThreadSafeReservoirBuffer(maxSize int) *ThreadSafeReservoirBuffer {
	return &ThreadSafeReservoirBuffer{buf: NewReservoirBuffer(maxSize)}
}

func init() {
	gob.Register(&ReservoirBuffer{})
	gob.Register(&ThreadSafeReservoirBuffer{})
}
