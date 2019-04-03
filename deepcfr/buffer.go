package deepcfr

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"sync/atomic"

	"github.com/timpalpant/go-cfr"
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
	samples []atomic.Value
	n       int64
}

// NewBuffer returns an empty Buffer with the given max size.
func NewReservoirBuffer(maxSize int) *ReservoirBuffer {
	return &ReservoirBuffer{
		samples: make([]atomic.Value, maxSize),
	}
}

// AddSample implements Buffer.
func (b *ReservoirBuffer) AddSample(node cfr.GameTreeNode, advantages []float32, weight float32) {
	// We a are a little bit sloppy here for improved performance:
	// Because we do not hold a lock for the duration of the call, it is possible
	// for an earlier call to AddSample to collide and overwrite a later call
	// if both are simultaneously assigned to the same random bucket.
	n := int(atomic.AddInt64(&b.n, 1))

	if n < len(b.samples) {
		sample := NewSample(node, advantages, weight)
		b.samples[n-1].Store(sample)
	} else {
		m := rand.Intn(n)
		if m < len(b.samples) {
			sample := NewSample(node, advantages, weight)
			b.samples[m].Store(sample)
		}
	}
}

// GetSamples implements Buffer.
func (b *ReservoirBuffer) GetSamples() []Sample {
	n := int(atomic.LoadInt64(&b.n))
	nSamples := min(n, len(b.samples))
	result := make([]Sample, nSamples)
	for i := 0; i < nSamples; i++ {
		result[i] = b.samples[i].Load().(Sample)
	}

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

	if err := enc.Encode(len(b.samples)); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.n); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.GetSamples()); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (b *ReservoirBuffer) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	maxSize := len(b.samples)
	if err := dec.Decode(maxSize); err != nil {
		return err
	}

	if err := dec.Decode(&b.n); err != nil {
		return err
	}

	var samples []Sample
	if err := dec.Decode(&samples); err != nil {
		return err
	}

	b.samples = make([]atomic.Value, maxSize)
	for i, s := range samples {
		b.samples[i].Store(s)
	}

	return nil
}

func init() {
	gob.Register(&ReservoirBuffer{})
}
