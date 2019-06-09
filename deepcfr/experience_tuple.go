package deepcfr

import (
	"encoding/binary"
	"encoding/gob"

	"github.com/timpalpant/go-cfr"
)

// ExperienceTuple is a single sample of action-result collected for training.
type ExperienceTuple struct {
	Weight  float32
	InfoSet []byte
	Action  uint16
	Value   float32
}

func NewExperienceTuple(node cfr.GameTreeNode, weight float32, action int, value float32) *ExperienceTuple {
	infoSet := node.InfoSet(node.Player())
	isBuf, err := infoSet.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return &ExperienceTuple{
		Weight:  weight,
		InfoSet: isBuf,
		Action:  uint16(action),
		Value:   value,
	}
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (t *ExperienceTuple) MarshalBinary() ([]byte, error) {
	nInfoSetBytes := len(t.InfoSet) + 4
	nBytes := nInfoSetBytes + 2 + 2*4
	result := make([]byte, nBytes)

	// Copy infoset bytes, prefixed by length.
	binary.LittleEndian.PutUint32(result, uint32(len(t.InfoSet)))
	buf := result[4:]
	copy(buf, t.InfoSet)
	buf = buf[len(t.InfoSet):]

	// Encode action.
	binary.LittleEndian.PutUint16(buf, t.Action)
	buf = buf[2:]

	// Encode weight, value, regret.
	putF32(buf, t.Weight)
	buf = buf[4:]
	putF32(buf, t.Value)
	buf = buf[4:]

	return buf, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (t *ExperienceTuple) UnmarshalBinary(buf []byte) error {
	nInfoSetBytes := binary.LittleEndian.Uint32(buf)
	buf = buf[4:]

	// UnmarshalBinary must copy the data it wishes to keep.
	t.InfoSet = make([]byte, nInfoSetBytes)
	copy(t.InfoSet, buf)
	buf = buf[nInfoSetBytes:]

	// Decode the action.
	t.Action = binary.LittleEndian.Uint16(buf)
	buf = buf[2:]

	// Decode weight, value, regret.
	t.Weight = decodeF32(buf)
	buf = buf[4:]
	t.Value = decodeF32(buf)
	buf = buf[4:]

	return nil
}

func init() {
	gob.Register(&ExperienceTuple{})
}
