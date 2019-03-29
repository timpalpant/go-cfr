package deepcfr

import (
	"bytes"
	"encoding/gob"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
)

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

func (d *DeepCFR) currentPlayer() int {
	return d.iter % 2
}

func (d *DeepCFR) currentModel(player int) TrainedModel {
	playerModels := d.trainedModels[player]
	if len(playerModels) == 0 {
		return nil
	}

	return playerModels[len(playerModels)-1]
}

func (d *DeepCFR) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	infoSet := node.InfoSet(node.Player())

	var strategy []float32
	currentModel := d.currentModel(node.Player())
	if currentModel == nil {
		strategy = uniformDist(node.NumChildren())
	} else {
		strategy = currentModel.Predict(infoSet, node.NumChildren())
	}

	return dcfrPolicy{
		buf:             d.buffers[node.Player()],
		infoSet:         infoSet,
		currentStrategy: strategy,
		iter:            d.iter,
	}
}

// Update implements cfr.StrategyProfile.
func (d *DeepCFR) Update() {
	player := d.currentPlayer()
	buf := d.buffers[player]
	trained := d.model.Train(buf)
	d.trainedModels[player] = append(d.trainedModels[player], trained)

	d.iter++
}

// Iter implements cfr.StrategyProfile.
func (d *DeepCFR) Iter() int {
	return d.iter
}

// Note that to be able to use this method, the concrete types
// implementing Model, TrainedModel, and cfr.InfoSet must be registered
// with gob.
func (d *DeepCFR) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Need to pass pointer to interface so that Gob sees the interface rather
	// than the concrete type. See the example in encoding/gob.
	if err := enc.Encode(&d.model); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.buffers); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.trainedModels); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.iter); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Reload the current state from the given io.Reader.
func (d *DeepCFR) GobDecode(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&d.model); err != nil {
		return err
	}

	if err := dec.Decode(&d.buffers); err != nil {
		return err
	}

	if err := dec.Decode(&d.trainedModels); err != nil {
		return err
	}

	if err := dec.Decode(&d.iter); err != nil {
		return err
	}

	return nil
}

type dcfrPolicy struct {
	infoSet         cfr.InfoSet
	buf             Buffer
	currentStrategy []float32
	iter            int
}

func (d dcfrPolicy) AddRegret(instantaneousRegrets []float32) {
	advantages := make([]float32, len(instantaneousRegrets))
	copy(advantages, instantaneousRegrets)
	d.buf.AddSample(Sample{
		InfoSet:    d.infoSet,
		Advantages: advantages,
		Weight:     float32(d.iter),
	})
}

func (d dcfrPolicy) GetStrategy() []float32 {
	return d.currentStrategy
}

func (d dcfrPolicy) AddStrategyWeight(w float32) {
}

func (d dcfrPolicy) GetAverageStrategy() []float32 {
	return nil
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}
