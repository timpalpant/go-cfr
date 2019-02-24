package deepcfr

import (
	"github.com/timpalpant/go-cfr"
)

// Sample is a single sample of instantaneous advantages
// collected for training.
type Sample struct {
	InfoSet    cfr.InfoSet
	Advantages []float32
	Iter       int
}

// Buffer collects samples of infoset action advantages to train a Model.
type Buffer interface {
	AddSample(s Sample)
	GetSamples() []Sample
}

// Model is a regression model to use in DeepCFR that predicts
// a vector of advantages for a given InfoSet.
type Model interface {
	Train(samples Buffer)
	Predict(infoSet cfr.InfoSet, nActions int) (advantages []float32)
}

// DeepCFR implements cfr.PolicyStore, and uses function approximation
// to estimate strategies rather than accumulation of regrets for all
// infosets. This can be more tractable for large games where storing
// all of the regrets for all infosets is impractical.
//
// During CFR iterations, samples are added to the given buffer.
// When NextIter is called, the model is retrained.
type DeepCFR struct {
	model Model
	buf   Buffer
	iter  int
}

func New(model Model, buffer Buffer) *DeepCFR {
	return &DeepCFR{
		model: model,
		buf:   buffer,
		iter:  1,
	}
}

// GetStrategy implements cfr.StrategyProfile.
func (d *DeepCFR) GetStrategy(node cfr.GameTreeNode) cfr.NodeStrategy {
	infoSet := node.InfoSet(node.Player())
	strategy := d.model.Predict(infoSet, node.NumChildren())
	return dCFRPolicy{
		strategy: strategy,
		buf:      d.buf,
		infoSet:  infoSet,
		iter:     d.iter,
	}
}

func (d *DeepCFR) Update() {
	d.model.Train(d.buf)
	d.iter++
}

type dCFRPolicy struct {
	strategy []float32
	buf      Buffer
	infoSet  cfr.InfoSet
	iter     int
}

// GetActionProbability implements cfr.Policy.
func (p dCFRPolicy) GetActionProbability(i int) float32 {
	return p.strategy[i]
}

// AddRegret implements cfr.Policy.
func (p dCFRPolicy) AddRegret(reachP, counterFactualP float32, advantages []float32) {
	p.buf.AddSample(Sample{
		InfoSet:    p.infoSet,
		Advantages: append([]float32(nil), advantages...),
		Iter:       p.iter,
	})
}

// NextStrategy implements cfr.Policy.
func (p dCFRPolicy) NextStrategy(discountPos, discountNeg, discountSum float32) {
	// DeepCFR training must be performed out-of-band by calling NextIter().
}

// GetAverageStrategy implements cfr.Policy.
func (p dCFRPolicy) GetAverageStrategy() []float32 {
	// TODO: Should average over all trained models like in Single Deep CFR:
	// https://arxiv.org/pdf/1901.07621.pdf.
	panic("not yet implemented")
}
