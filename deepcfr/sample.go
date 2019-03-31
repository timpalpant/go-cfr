package deepcfr

import (
	"encoding/binary"
	"math"

	"github.com/timpalpant/go-cfr"
)

// Sample is a single sample of instantaneous advantages
// collected for training.
type Sample struct {
	InfoSet    []byte
	Advantages []float32
	Weight     float32
}

func NewSample(infoSet cfr.InfoSet, advantages []float32, weight float32) Sample {
	isBuf, err := infoSet.MarshalBinary()
	if err != nil {
		panic(err)
	}

	advantagesCopy := make([]float32, len(advantages))
	copy(advantagesCopy, advantages)

	return Sample{
		InfoSet:    isBuf,
		Advantages: advantagesCopy,
		Weight:     weight,
	}
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s Sample) MarshalBinary() ([]byte, error) {
	nInfoSetBytes := len(s.InfoSet) + 4
	nAdvantagesBytes := 4 * len(s.Advantages)
	nSampleWeightBytes := 4
	nBytes := nInfoSetBytes + nAdvantagesBytes + nSampleWeightBytes
	result := make([]byte, nBytes)

	// Copy infoset bytes, prefixed by length.
	binary.LittleEndian.PutUint32(result, uint32(len(s.InfoSet)))
	buf := result[4:]
	copy(buf, s.InfoSet)
	buf = buf[len(s.InfoSet):]

	// Encode sample Weight.
	bits := math.Float32bits(s.Weight)
	binary.LittleEndian.PutUint32(buf, bits)
	buf = buf[4:]

	// Encode sample advantages.
	for _, x := range s.Advantages {
		bits := math.Float32bits(x)
		binary.LittleEndian.PutUint32(buf, bits)
		buf = buf[4:]
	}

	return result, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *Sample) UnmarshalBinary(buf []byte) error {
	nInfoSetBytes := binary.LittleEndian.Uint32(buf)
	buf = buf[4:]

	// UnmarshalBinary must copy the data it wishes to keep.
	s.InfoSet = make([]byte, nInfoSetBytes)
	copy(s.InfoSet, buf)
	buf = buf[nInfoSetBytes:]

	// Decode the weight.
	weightBits := binary.LittleEndian.Uint32(buf)
	s.Weight = math.Float32frombits(weightBits)
	buf = buf[4:]

	// Decode the vector of advantages.
	s.Advantages = decodeF32s(buf)

	return nil
}

func decodeF32s(buf []byte) []float32 {
	n := len(buf) / 4
	result := make([]float32, n)
	for i := range result {
		bits := binary.LittleEndian.Uint32(buf)
		result[i] = math.Float32frombits(bits)
		buf = buf[4:]
	}

	return result
}
