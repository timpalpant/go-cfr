package deepcfr

import (
	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
)

// Sample is a single sample of instantaneous advantages
// collected for training.
type Sample struct {
	InfoSet    cfr.InfoSet
	Advantages []float32
	Weight     float32
}

// Buffer collects samples of infoset action advantages to train a Model.
type Buffer interface {
	AddSample(s Sample)
	GetSamples() []Sample
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

// DeepCFR implements cfr.StrategyProfile, and uses function approximation
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

func (d *DeepCFR) AddRegret(node cfr.GameTreeNode, instantaneousRegrets []float32) {
	buf := d.buffers[node.Player()]
	buf.AddSample(Sample{
		InfoSet:    node.InfoSet(node.Player()),
		Advantages: append([]float32(nil), instantaneousRegrets...),
		Weight:     float32(d.iter),
	})
}

func (d *DeepCFR) GetPolicy(node cfr.GameTreeNode) []float32 {
	var strategy []float32
	currentModel := d.currentModel(node.Player())
	if currentModel == nil {
		strategy = uniformDist(node.NumChildren())
	} else {
		infoSet := node.InfoSet(node.Player())
		strategy = currentModel.Predict(infoSet, node.NumChildren())
	}

	return strategy
}

func (d *DeepCFR) AddStrategyWeight(node cfr.GameTreeNode, w float32) {
}

func (d *DeepCFR) GetAverageStrategy(node cfr.GameTreeNode) []float32 {
	return nil
}

// Update implements cfr.StrategyProfile.
func (d *DeepCFR) Update() {
	player := d.iter % 2
	buf := d.buffers[player]
	trained := d.model.Train(buf)
	d.trainedModels[player] = append(d.trainedModels[player], trained)

	d.iter++
}

// Iter implements cfr.StrategyProfile.
func (d *DeepCFR) Iter() int {
	return d.iter
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}
