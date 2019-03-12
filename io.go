package cfr

import (
	"encoding/binary"
	"io"
)

var byteOrder = binary.LittleEndian

func LoadStrategyTable(r io.Reader) (*StrategyTable, error) {
	var params DiscountParams
	if err := binary.Read(r, byteOrder, &params); err != nil {
		return nil, err
	}

	var iter int64
	if err := binary.Read(r, byteOrder, &iter); err != nil {
		return nil, err
	}

	var nStrategies int64
	if err := binary.Read(r, byteOrder, &nStrategies); err != nil {
		return nil, err
	}

	strategies := make(map[string]*strategy, nStrategies)
	for i := int64(0); i < nStrategies; i++ {
		var keyLen int64
		if err := binary.Read(r, byteOrder, &keyLen); err != nil {
			return nil, err
		}

		key := make([]byte, int(keyLen))
		_, err := io.ReadFull(r, key)
		if err != nil {
			return nil, err
		}

		s, err := readStrategy(r)
		if err != nil {
			return nil, err
		}

		strategies[string(key)] = s
	}

	return &StrategyTable{
		params:     params,
		iter:       int(iter),
		strategies: strategies,
	}, nil
}

func (st *StrategyTable) MarshalTo(w io.Writer) error {
	if err := binary.Write(w, byteOrder, st.params); err != nil {
		return err
	}

	if err := binary.Write(w, byteOrder, int64(st.iter)); err != nil {
		return err
	}

	if err := binary.Write(w, byteOrder, int64(len(st.strategies))); err != nil {
		return err
	}

	for key, s := range st.strategies {
		if err := binary.Write(w, byteOrder, int64(len(key))); err != nil {
			return err
		}

		if _, err := io.WriteString(w, key); err != nil {
			return err
		}

		if err := s.marshalTo(w); err != nil {
			return err
		}
	}

	return nil
}

func readStrategy(r io.Reader) (*strategy, error) {
	var nActions int64
	if err := binary.Read(r, byteOrder, &nActions); err != nil {
		return nil, err
	}

	s := newStrategy(int(nActions))
	if err := binary.Read(r, byteOrder, &s.regretSum); err != nil {
		return nil, err
	}

	if err := binary.Read(r, byteOrder, &s.strategySum); err != nil {
		return nil, err
	}

	s.calcStrategy()
	return s, nil
}

func (s *strategy) marshalTo(w io.Writer) error {
	if err := binary.Write(w, byteOrder, int64(s.numActions())); err != nil {
		return err
	}

	if err := binary.Write(w, byteOrder, s.regretSum); err != nil {
		return err
	}

	return binary.Write(w, byteOrder, s.strategySum)
}
