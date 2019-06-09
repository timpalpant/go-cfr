package deepcfr

import (
	"bytes"
	"encoding/gob"
	"sync"
)

type CircularBuffer struct {
	mx      sync.Mutex
	maxSize int
	samples []Sample
	idx     int
}

func NewCircularBuffer(maxSize int) *CircularBuffer {
	return &CircularBuffer{
		maxSize: maxSize,
		samples: make([]Sample, 0, maxSize),
	}
}

func (b *CircularBuffer) AddSample(sample Sample) {
	b.mx.Lock()
	defer b.mx.Unlock()

	if len(b.samples) < b.maxSize {
		b.samples = append(b.samples, sample)
	} else {
		b.samples[b.idx] = sample
		b.idx++
	}
}

func (b *CircularBuffer) GetSample(idx int) Sample {
	b.mx.Lock()
	defer b.mx.Unlock()
	return b.samples[idx]
}

func (b *CircularBuffer) Len() int {
	b.mx.Lock()
	defer b.mx.Unlock()
	return len(b.samples)
}

func (b *CircularBuffer) GetSamples() []Sample {
	result := make([]Sample, len(b.samples))
	b.mx.Lock()
	defer b.mx.Unlock()
	copy(result, b.samples)
	return result
}

func (b *CircularBuffer) Close() error {
	return nil
}

func (b *CircularBuffer) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(b.maxSize); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.samples); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (b *CircularBuffer) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&b.maxSize); err != nil {
		return err
	}

	if err := dec.Decode(&b.samples); err != nil {
		return err
	}

	return nil
}

func init() {
	gob.Register(&CircularBuffer{})
}
