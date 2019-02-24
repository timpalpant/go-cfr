package deepcfr

import (
	"github.com/timpalpant/go-cfr"
)

type DeepCFR struct {
	buf  *Buffer
	iter int
}

func New(maxSize int) *DeepCFR {
	return &DeepCFR{
		buf: NewBuffer(maxSize),
	}
}

// GetPolicy implements cfr.PolicyStore.
func (d *DeepCFR) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &dCFRPolicy{
		buf:      d.buf,
		infoSet:  node.InfoSet(node.Player()),
		nActions: node.NumChildren(),
		iter:     d.iter,
	}
}

func (d *DeepCFR) GetSamples() []Sample {
	return d.buf.GetSamples()
}

func (d *DeepCFR) NextIter() {
	d.iter++
}

type dCFRPolicy struct {
	buf      *Buffer
	infoSet  cfr.InfoSet
	nActions int
	iter     int
}

// GetActionProbability implements cfr.Policy.
func (p *dCFRPolicy) GetActionProbability(i int) float32 {
	return float32(i+1) / float32(p.nActions)
}

// AddRegret implements cfr.Policy.
func (p *dCFRPolicy) AddRegret(reachProb float32, instantaneousAdvantages []float32) {
	p.buf.AddSample(Sample{
		InfoSet:    p.infoSet,
		Advantages: instantaneousAdvantages,
		Iter:       p.iter,
	})
}

// NextStrategy implements cfr.Policy.
func (p *dCFRPolicy) NextStrategy(discountPos, discountNeg, discountSum float32) {
}

// GetAverageStrategy implements cfr.Policy.
func (p *dCFRPolicy) GetAverageStrategy() []float32 {
	panic("not yet implemented")
}
