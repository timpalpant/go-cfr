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
	nBytes := len(s.InfoSet) + 4*(len(s.Advantages)+2)

	result := make([]byte, nBytes)
	binary.LittleEndian.PutUint32(result, uint32(len(s.Advantages)))
	buf := result[4:]

	// Encode sample advantages.
	for _, x := range s.Advantages {
		bits := math.Float32bits(x)
		binary.LittleEndian.PutUint32(buf, bits)
		buf = buf[4:]
	}

	// Encode sample Weight.
	bits := math.Float32bits(s.Weight)
	binary.LittleEndian.PutUint32(buf, bits)
	buf = buf[4:]

	// Copy InfoSet bytes.
	copy(buf, s.InfoSet)
	return result, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *Sample) UnmarshalBinary(buf []byte) error {
	// Look at last 4 bytes to figure out how many advantages we have.
	nBuf := buf[:4]
	nAdvantages := binary.LittleEndian.Uint32(nBuf)

	// Separate out the last bytes that we need to decode from the InfoSet bytes.
	nBytes := int(4 * (nAdvantages + 2))
	s.InfoSet = buf[nBytes:]
	buf = buf[4:nBytes]

	// Decode the weight.
	weightBits := binary.LittleEndian.Uint32(buf[:4])
	s.Weight = math.Float32frombits(weightBits)
	buf = buf[4:]

	// Decode the vector of advantages.
	s.Advantages = make([]float32, nAdvantages)
	for i := range s.Advantages {
		aBuf := buf[4*i : 4*(i+1)]
		bits := binary.LittleEndian.Uint32(aBuf)
		x := math.Float32frombits(bits)
		s.Advantages[i] = x
	}

	return nil
}
