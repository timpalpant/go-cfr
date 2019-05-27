package deepcfr

import (
	"encoding/binary"
	"encoding/gob"
	"math"

	"github.com/timpalpant/go-cfr"
)

// RegretSample is a single sample of instantaneous advantages
// collected for training.
type RegretSample struct {
	Weight     float32
	InfoSet    []byte
	Advantages []float32
}

func NewRegretSample(node cfr.GameTreeNode, advantages []float32, weight float32) *RegretSample {
	infoSet := node.InfoSet(node.Player())
	isBuf, err := infoSet.MarshalBinary()
	if err != nil {
		panic(err)
	}

	advantagesCopy := make([]float32, len(advantages))
	copy(advantagesCopy, advantages)

	return &RegretSample{
		Weight:     weight,
		InfoSet:    isBuf,
		Advantages: advantagesCopy,
	}
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s *RegretSample) MarshalBinary() ([]byte, error) {
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
		putF32(buf, x)
		buf = buf[4:]
	}

	return result, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *RegretSample) UnmarshalBinary(buf []byte) error {
	nInfoSetBytes := binary.LittleEndian.Uint32(buf)
	buf = buf[4:]

	// UnmarshalBinary must copy the data it wishes to keep.
	s.InfoSet = make([]byte, nInfoSetBytes)
	copy(s.InfoSet, buf)
	buf = buf[nInfoSetBytes:]

	// Decode the weight.
	s.Weight = decodeF32(buf)
	buf = buf[4:]

	// Decode the vector of advantages.
	s.Advantages = decodeF32s(buf)

	return nil
}

func decodeF32s(buf []byte) []float32 {
	n := len(buf) / 4
	result := make([]float32, n)
	for i := range result {
		result[i] = decodeF32(buf)
		buf = buf[4:]
	}

	return result
}

func putF32(buf []byte, x float32) {
	bits := math.Float32bits(x)
	binary.LittleEndian.PutUint32(buf, bits)
}

func decodeF32(buf []byte) float32 {
	bits := binary.LittleEndian.Uint32(buf)
	return math.Float32frombits(bits)
}

func init() {
	gob.Register(&RegretSample{})
}
