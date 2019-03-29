package policy

import (
	"bytes"
	"encoding/gob"

	"github.com/timpalpant/go-cfr/internal/f32"
)

// Policy implements cfr.Policy by keeping a table of
// accumulated regrets and strategies.
type Policy struct {
	currentStrategy       []float32
	currentStrategyWeight float32

	regretSum   []float32
	strategySum []float32
}

// NewPolicy returns a new NodePolicy for a game node with the given number of actions.
func New(nActions int) *Policy {
	return &Policy{
		currentStrategy:       uniformDist(nActions),
		currentStrategyWeight: 0.0,
		regretSum:             make([]float32, nActions),
		strategySum:           make([]float32, nActions),
	}
}

func (p *Policy) GetStrategy() []float32 {
	return p.currentStrategy
}

func (p *Policy) NextStrategy(discountPositiveRegret, discountNegativeRegret, discountstrategySum float32) {
	if discountstrategySum != 1.0 {
		f32.ScalUnitary(discountstrategySum, p.strategySum)
	}

	f32.AxpyUnitary(p.currentStrategyWeight, p.currentStrategy, p.strategySum)

	if discountPositiveRegret != 1.0 {
		for i, x := range p.regretSum {
			if x > 0 {
				p.regretSum[i] *= discountPositiveRegret
			}
		}
	}

	if discountNegativeRegret != 1.0 {
		for i, x := range p.regretSum {
			if x < 0 {
				p.regretSum[i] *= discountNegativeRegret
			}
		}
	}

	p.regretMatching()
	p.currentStrategyWeight = 0.0
}

func (p *Policy) AddRegret(instantaneousRegrets []float32) {
	f32.Add(p.regretSum, instantaneousRegrets)
}

func (p *Policy) AddStrategyWeight(w float32) {
	p.currentStrategyWeight += w
}

func (p *Policy) GetAverageStrategy() []float32 {
	avgStrat := make([]float32, len(p.strategySum))

	total := f32.Sum(p.strategySum)
	if total > 0 {
		f32.ScalUnitaryTo(avgStrat, 1.0/total, p.strategySum)
	} else {
		for i := range avgStrat {
			avgStrat[i] = 1.0 / float32(len(avgStrat))
		}
	}

	return avgStrat
}

func (p *Policy) GetStrategySum() []float32 {
	return p.strategySum
}

func (p *Policy) NumActions() int {
	return len(p.regretSum)
}

func (p *Policy) regretMatching() {
	copy(p.currentStrategy, p.regretSum)
	makePositive(p.currentStrategy)
	total := f32.Sum(p.currentStrategy)
	if total > 0 {
		f32.ScalUnitary(1.0/total, p.currentStrategy)
	} else {
		for i := range p.currentStrategy {
			p.currentStrategy[i] = 1.0 / float32(len(p.currentStrategy))
		}
	}
}

func (p *Policy) GobDecode(buf []byte) error {
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

	p.regretSum = regretSum
	p.strategySum = strategySum
	p.currentStrategy = make([]float32, nActions)
	p.regretMatching()
	return nil
}

func (p *Policy) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(p.NumActions()); err != nil {
		return nil, err
	}

	if err := enc.Encode(p.regretSum); err != nil {
		return nil, err
	}

	if err := enc.Encode(p.strategySum); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}
