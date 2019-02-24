package deepcfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr"
)

// Sample is a single sample of instantaneous advantages
// collected for training.
type Sample struct {
	InfoSet    cfr.InfoSet
	Advantages []float32
	Iter       int
}

// Buffer is a collection of samples.
// One the Buffer's max size is reached, additional
// samples are added via reservoir sampling, maintaining
// a uniform distribution over all previous values.
type Buffer struct {
	maxSize int
	samples []Sample
	n       int
}

// NewBuffer returns an empty Buffer with the given max size.
func NewBuffer(maxSize int) *Buffer {
	return &Buffer{
		maxSize: maxSize,
	}
}

// AddSample adds the given Sample to the Buffer, using
// reservoir sampling once it has filled to maintain a uniform probability
// over all added samples.
func (b *Buffer) AddSample(s Sample) {
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

// GetSamples returns all collected samples.
func (b *Buffer) GetSamples() []Sample {
	return b.samples
}

// Reset empties the Buffer, but retains allocated capacity for reuse.
func (b *Buffer) Reset() {
	b.samples = b.samples[:0]
	b.n = 0
}
