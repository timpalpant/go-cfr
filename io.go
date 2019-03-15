package cfr

import (
	"bytes"
	"encoding/gob"
	"io"
)

func LoadStrategyTable(r io.Reader) (*StrategyTable, error) {
	dec := gob.NewDecoder(r)
	var params DiscountParams
	if err := dec.Decode(&params); err != nil {
		return nil, err
	}

	var iter int64
	if err := dec.Decode(&iter); err != nil {
		return nil, err
	}

	var nStrategies int64
	if err := dec.Decode(&nStrategies); err != nil {
		return nil, err
	}

	strategies := make(map[string]*strategy, nStrategies)
	for i := int64(0); i < nStrategies; i++ {
		var keyLen int64
		if err := dec.Decode(&keyLen); err != nil {
			return nil, err
		}

		var key string
		if err := dec.Decode(&key); err != nil {
			return nil, err
		}

		var s strategy
		if err := dec.Decode(&s); err != nil {
			return nil, err
		}

		strategies[string(key)] = &s
	}

	return &StrategyTable{
		params:     params,
		iter:       int(iter),
		strategies: strategies,
	}, nil
}

func (st *StrategyTable) MarshalTo(w io.Writer) error {
	enc := gob.NewEncoder(w)
	if err := enc.Encode(st.params); err != nil {
		return err
	}

	if err := enc.Encode(st.iter); err != nil {
		return err
	}

	if err := enc.Encode(len(st.strategies)); err != nil {
		return err
	}

	for key, s := range st.strategies {
		if err := enc.Encode(len(key)); err != nil {
			return err
		}

		if err := enc.Encode(key); err != nil {
			return err
		}

		if err := enc.Encode(s); err != nil {
			return err
		}
	}

	return nil
}

func (s *strategy) GobDecode(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	var nActions int
	if err := dec.Decode(&nActions); err != nil {
		return err
	}

	regretSum := make([]float32, 0, nActions)
	if err := dec.Decode(&regretSum); err != nil {
		return err
	}

	strategySum := make([]float32, 0, nActions)
	if err := dec.Decode(&strategySum); err != nil {
		return err
	}

	s.regretSum = regretSum
	s.strategySum = strategySum
	s.current = make([]float32, nActions)
	s.calcStrategy()
	return nil
}

func (s *strategy) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(s.numActions()); err != nil {
		return nil, err
	}

	if err := enc.Encode(s.regretSum); err != nil {
		return nil, err
	}

	if err := enc.Encode(s.strategySum); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
