package deepcfr

import (
	"encoding/gob"
	"math/rand"
)

// ReservoirBuffer is a collection of samples held in memory.
// One the buffer's max size is reached, additional
// samples are added via reservoir sampling, maintaining
// a uniform distribution over all previous values.
type ReservoirBuffer struct {
	MaxSize int
	Samples []Sample
	N       int
}

// NewBuffer returns an empty Buffer with the given max size.
func NewReservoirBuffer(maxSize int) *ReservoirBuffer {
	return &ReservoirBuffer{
		MaxSize: maxSize,
	}
}

// AddSample implements Buffer.
func (b *ReservoirBuffer) AddSample(s Sample) {
	b.N++

	if len(b.Samples) < b.MaxSize {
		b.Samples = append(b.Samples, s)
	} else {
		m := rand.Intn(b.N)
		if m < b.MaxSize {
			b.Samples[m] = s
		}
	}
}

// GetSamples implements Buffer.
func (b *ReservoirBuffer) GetSamples() []Sample {
	return b.Samples
}

func init() {
	gob.Register(&ReservoirBuffer{})
}
