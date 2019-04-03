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
//
// It is safe to call AddSample concurrently from multiple goroutines.
// GetSamples does not copy the underlying slice of samples, and therefore
// is not safe to call concurrently with AddSample.
type ReservoirBuffer struct {
	mx      sync.Mutex
	maxSize int
	samples []Sample
	n       int
	rng     *rand.Rand
}

// NewBuffer returns an empty Buffer with the given max size.
func NewReservoirBuffer(maxSize int, seed int64) *ReservoirBuffer {
	return &ReservoirBuffer{
		maxSize: maxSize,
		rng:     rand.New(rand.NewSource(seed)),
	}
}

// AddSample implements Buffer.
func (b *ReservoirBuffer) AddSample(node cfr.GameTreeNode, advantages []float32, weight float32) {
	// Either: We construct the Sample here (which is slow).
	// Unfortunately it might not be needed, and we'll do work unnecessarily.
	// Alternatively, we wait to construct the Sample until we know it's being
	// kept, however, then we need to do the slow step under the lock.
	sample := NewSample(node, advantages, weight)

	b.mx.Lock()
	defer b.mx.Unlock()
	b.n++

	if len(b.samples) < b.maxSize {
		b.samples = append(b.samples, sample)
	} else {
		m := b.rng.Intn(b.n)
		if m < b.maxSize {
			b.samples[m] = sample
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

func init() {
	gob.Register(&ReservoirBuffer{})
}
