package deepcfr

import (
	"encoding/binary"
	"math"

	"github.com/timpalpant/go-cfr"
)

// Sample is a single sample of instantaneous advantages
// collected for training.
type Sample struct {
	InfoSet    cfr.InfoSet
	Advantages []float32
	Weight     float32
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s Sample) MarshalBinary() ([]byte, error) {
	buf, err := s.InfoSet.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// 4 bytes per advantage float + 4 for Weight and 4 for number of advantages.
	nBytes := 4 * (len(s.Advantages) + 2)
	buf = append(buf, make([]byte, nBytes)...)
	ourBuf := buf[len(buf)-nBytes:]

	// Encode sample advantages.
	for _, x := range s.Advantages {
		bits := math.Float32bits(x)
		binary.LittleEndian.PutUint32(ourBuf, bits)
		ourBuf = ourBuf[4:]
	}

	// Encode sample Weight.
	bits := math.Float32bits(s.Weight)
	binary.LittleEndian.PutUint32(ourBuf, bits)
	ourBuf = ourBuf[4:]

	// Place number of Advantages at the end, so we can separate them out
	// when decoding.
	binary.LittleEndian.PutUint32(ourBuf, uint32(len(s.Advantages)))

	return buf, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *Sample) UnmarshalBinary(buf []byte) error {
	// Look at last 4 bytes to figure out how many advantages we have.
	nBuf := buf[len(buf)-4:]
	nAdvantages := binary.LittleEndian.Uint32(nBuf)

	// Separate out the last N bytes that we need to decode from the InfoSet bytes.
	nBytes := int(4 * (nAdvantages + 2))
	ourBuf := buf[len(buf)-nBytes : len(buf)-4]

	// Decode the weight.
	sampleWeightBuf := ourBuf[len(ourBuf)-4:]
	weightBits := binary.LittleEndian.Uint32(sampleWeightBuf)
	s.Weight = math.Float32frombits(weightBits)

	// Decode the vector of advantages.
	s.Advantages = make([]float32, nAdvantages)
	for i := range s.Advantages {
		aBuf := ourBuf[4*i : 4*(i+1)]
		bits := binary.LittleEndian.Uint32(aBuf)
		x := math.Float32frombits(bits)
		s.Advantages[i] = x
	}

	// Decode the infoset.
	isBuf := buf[:len(buf)-nBytes]
	return s.InfoSet.UnmarshalBinary(isBuf)
}
