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
	return &dcfrPolicy{
		node:         node,
		buf:          d.buffers[node.Player()],
		currentModel: d.currentModel(node.Player()),
		iter:         d.iter,
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

func (d *DeepCFR) SampleModel() TrajectorySampledSDCFR {
	models := sampleModels(d.trainedModels)
	return TrajectorySampledSDCFR(models)
}

func (d *DeepCFR) Close() error {
	for _, buf := range d.buffers {
		if err := buf.Close(); err != nil {
			return err
		}
	}

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
// Note that to be able to use this method, the concrete types
// implementing the Model, TrainedModel, and Buffers must be registered
// with gob.
func (d *DeepCFR) MarshalBinary() ([]byte, error) {
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

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (d *DeepCFR) UnmarshalBinary(buf []byte) error {
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
	node            cfr.GameTreeNode
	buf             Buffer
	currentModel    TrainedModel
	currentStrategy []float32
	iter            int
}

func (d *dcfrPolicy) AddRegret(w float32, instantaneousRegrets []float32) {
	w *= float32(d.iter) // Linear CFR.
	d.buf.AddSample(d.node, instantaneousRegrets, w)
}

func (d *dcfrPolicy) GetStrategy() []float32 {
	if d.currentStrategy == nil {
		if d.currentModel == nil {
			d.currentStrategy = uniformDist(d.node.NumChildren())
		} else {
			infoSet := d.node.InfoSet(d.node.Player())
			d.currentStrategy = d.currentModel.Predict(infoSet, d.node.NumChildren())
		}
	}

	return d.currentStrategy
}

func (d *dcfrPolicy) AddStrategyWeight(w float32) {
}

func (d *dcfrPolicy) GetAverageStrategy() []float32 {
	panic("not implemented: use SampleModel to perform trajectory sampling SD-CFR")
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func init() {
	gob.Register(&DeepCFR{})
}
