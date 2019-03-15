package deepcfr

import (
	"encoding/gob"
	"io"
	"math/rand"
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

// MarshalTo implements Buffer.
func (b *ReservoirBuffer) MarshalTo(w io.Writer) error {
	enc := gob.NewEncoder(w)
	if err := enc.Encode(b.maxSize); err != nil {
		return err
	}

	if err := enc.Encode(b.samples); err != nil {
		return err
	}

	if err := enc.Encode(b.n); err != nil {
		return err
	}

	return nil
}

func init() {
	gob.Register(&ReservoirBuffer{})
}
