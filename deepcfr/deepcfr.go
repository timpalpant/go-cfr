package deepcfr

import (
	"io"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
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
	MarshalTo(io.Writer) error
}

// Model is a regression model that can be used to fit the given samples.
type Model interface {
	Train(samples Buffer) TrainedModel
}

// TrainedModel is a regression model to use in DeepCFR that predicts
// a vector of advantages for a given InfoSet.
type TrainedModel interface {
	Predict(infoSet cfr.InfoSet, nActions int) (advantages []float32)
}

// DeepCFR implements cfr.NodeStrategyStore, and uses function approximation
// to estimate strategies rather than accumulation of regrets for all
// infosets. This can be more tractable for large games where storing
// all of the regrets for all infosets is impractical.
//
// During CFR iterations, samples are added to the given buffer.
// When NextIter is called, the model is retrained.
type DeepCFR struct {
	model         Model
	buffers       []Buffer
	trainedModels [][]TrainedModel
	iter          int
}

// New returns a new DeepCFR policy with the given model and sample buffer.
func New(model Model, buffers []Buffer) *DeepCFR {
	return &DeepCFR{
		model:   model,
		buffers: buffers,
		trainedModels: [][]TrainedModel{
			[]TrainedModel{},
			[]TrainedModel{},
		},
		iter: 1,
	}
}

func (d *DeepCFR) currentModel(player int) TrainedModel {
	playerModels := d.trainedModels[player]
	if len(playerModels) == 0 {
		return nil
	}

	return playerModels[len(playerModels)-1]
}

// GetStrategy implements cfr.StrategyProfile.
func (d *DeepCFR) GetStrategy(node cfr.GameTreeNode) cfr.NodeStrategy {
	infoSet := node.InfoSet(node.Player())

	var strategy []float32
	model := d.currentModel(node.Player())
	if model == nil {
		strategy = uniformDist(node.NumChildren())
	} else {
		strategy = model.Predict(infoSet, node.NumChildren())
	}

	return dCFRPolicy{
		strategy:      strategy,
		buf:           d.buffers[node.Player()],
		infoSet:       infoSet,
		iter:          d.iter,
		trainedModels: d.trainedModels[node.Player()],
	}
}

// Update implements cfr.StrategyProfile.
func (d *DeepCFR) Update() {
	for player, buf := range d.buffers {
		trained := d.model.Train(buf)
		d.trainedModels[player] = append(d.trainedModels[player], trained)
	}

	d.iter++
}

type dCFRPolicy struct {
	strategy      []float32
	buf           Buffer
	infoSet       cfr.InfoSet
	iter          int
	trainedModels []TrainedModel
}

// GetActionProbability implements cfr.NodeStrategy.
func (p dCFRPolicy) GetActionProbability(i int) float32 {
	return p.strategy[i]
}

// AddRegret implements cfr.NodeStrategy.
func (p dCFRPolicy) AddRegret(reachP, counterFactualP float32, advantages []float32) {
	p.buf.AddSample(Sample{
		InfoSet:    p.infoSet,
		Advantages: append([]float32(nil), advantages...),
		Iter:       p.iter,
	})
}

// GetAverageStrategy implements cfr.NodeStrategy.
func (p dCFRPolicy) GetAverageStrategy() []float32 {
	// We calculate the average strategy as in Single Deep CFR:
	// https://arxiv.org/pdf/1901.07621.pdf.
	panic("not yet implemented")
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}
