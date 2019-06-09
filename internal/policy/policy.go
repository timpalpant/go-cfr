package policy

import (
	"encoding/binary"
	"math"

	"github.com/timpalpant/go-cfr/internal/f32"
)

// Policy implements cfr.NodePolicy by keeping a table of
// accumulated regrets and strategies.
type Policy struct {
	currentStrategy       []float32
	currentStrategyWeight float32

	baseline []float32

	regretSum   []float32
	strategySum []float32
}

// NewPolicy returns a new Policy for a game node with the given number of actions.
func New(nActions int) *Policy {
	return &Policy{
		currentStrategy:       uniformDist(nActions),
		currentStrategyWeight: 0.0,
		baseline:              make([]float32, nActions),
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

func (p *Policy) AddRegret(w float32, samplingQ, instantaneousRegrets []float32) {
	f32.AxpyUnitary(w, instantaneousRegrets, p.regretSum)
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

func (p *Policy) GetBaseline() []float32 {
	return p.baseline
}

func (p *Policy) UpdateBaseline(w float32, action int, value float32) {
	p.baseline[action] *= (1 - w)
	p.baseline[action] += w * value
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

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (p *Policy) UnmarshalBinary(buf []byte) error {
	nFloats := len(buf) / 4
	nActions := (nFloats - 1) / 4

	p.currentStrategyWeight = decodeF32(buf[:4])
	buf = buf[4:]

	p.currentStrategy = decodeF32s(buf[:4*nActions])
	buf = buf[4*nActions:]

	p.regretSum = decodeF32s(buf[:4*nActions])
	buf = buf[4*nActions:]

	p.strategySum = decodeF32s(buf[:4*nActions])
	buf = buf[4*nActions:]

	p.baseline = decodeF32s(buf[:4*nActions])

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (p *Policy) MarshalBinary() ([]byte, error) {
	nActions := len(p.regretSum)
	nBytes := 4 * (4*nActions + 1)
	result := make([]byte, nBytes)

	putF32(result, p.currentStrategyWeight)
	buf := result[4:]

	putF32s(buf, p.currentStrategy)
	buf = buf[4*nActions:]

	putF32s(buf, p.regretSum)
	buf = buf[4*nActions:]

	putF32s(buf, p.strategySum)
	buf = buf[4*nActions:]

	putF32s(buf, p.baseline)

	return result, nil
}

func putF32(buf []byte, x float32) {
	bits := math.Float32bits(x)
	binary.LittleEndian.PutUint32(buf, bits)
}

func decodeF32(buf []byte) float32 {
	bits := binary.LittleEndian.Uint32(buf[:4])
	return math.Float32frombits(bits)
}

func putF32s(buf []byte, v []float32) {
	for i, x := range v {
		xBuf := buf[4*i : 4*(i+1)]
		putF32(xBuf, x)
	}
}

func decodeF32s(buf []byte) []float32 {
	n := len(buf) / 4
	v := make([]float32, n)
	for i := range v {
		bits := binary.LittleEndian.Uint32(buf[:4])
		v[i] = math.Float32frombits(bits)
		buf = buf[4:]
	}

	return v
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
