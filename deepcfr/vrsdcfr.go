package deepcfr

import (
	"bytes"
	"encoding/gob"

	"github.com/timpalpant/go-cfr"
)

// VRSingleDeepCFR implements cfr.StrategyProfile, and uses function approximation
// to estimate strategies rather than accumulation of regrets for all
// infosets. This can be more tractable for large games where storing
// all of the regrets for all infosets is impractical.
//
// During CFR iterations, samples are added to the given buffer.
// When Update is called, the model is retrained.
type VRSingleDeepCFR struct {
	model          Model
	buffers        []Buffer
	trainedModels  [][]TrainedModel
	baselineBuffer Buffer
	baselineModel  TrainedModel
	iter           int
}

// New returns a new VRSingleDeepCFR policy with the given model and sample buffer.
func NewVRSingleDeepCFR(model Model, buffers []Buffer, baselineBuffer Buffer) *VRSingleDeepCFR {
	return &VRSingleDeepCFR{
		model:          model,
		buffers:        buffers,
		baselineBuffer: baselineBuffer,
		trainedModels: [][]TrainedModel{
			[]TrainedModel{},
			[]TrainedModel{},
		},
		iter: 1,
	}
}

func (d *VRSingleDeepCFR) currentPlayer() int {
	return d.iter % 2
}

func (d *VRSingleDeepCFR) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &vrdcfrPolicy{
		node:          node,
		buf:           d.buffers[node.Player()],
		baselineBuf:   d.baselineBuffer,
		models:        d.trainedModels[node.Player()],
		baselineModel: d.baselineModel,
		iter:          d.iter,
	}
}

// Update implements cfr.StrategyProfile.
func (d *VRSingleDeepCFR) Update() {
	player := d.currentPlayer()
	buf := d.buffers[player]
	trained := d.model.Train(buf)
	model := &AdvantageModel{trained}
	d.trainedModels[player] = append(d.trainedModels[player], model)

	d.baselineModel = d.model.Train(d.baselineBuffer)

	d.iter++
}

// Iter implements cfr.StrategyProfile.
func (d *VRSingleDeepCFR) Iter() int {
	return d.iter
}

func (d *VRSingleDeepCFR) Close() error {
	for _, buf := range d.buffers {
		if err := buf.Close(); err != nil {
			return err
		}
	}

	return d.baselineBuffer.Close()
}

// MarshalBinary implements encoding.BinaryMarshaler.
// Note that to be able to use this method, the concrete types
// implementing the Model, TrainedModel, and Buffers must be registered
// with gob.
func (d *VRSingleDeepCFR) MarshalBinary() ([]byte, error) {
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

	if err := enc.Encode(&d.baselineBuffer); err != nil {
		return nil, err
	}

	if err := enc.Encode(&d.baselineModel); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.iter); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (d *VRSingleDeepCFR) UnmarshalBinary(buf []byte) error {
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

	if err := dec.Decode(&d.baselineBuffer); err != nil {
		return err
	}

	if err := dec.Decode(&d.baselineModel); err != nil {
		return err
	}

	if err := dec.Decode(&d.iter); err != nil {
		return err
	}

	return nil
}

type vrdcfrPolicy struct {
	node          cfr.GameTreeNode
	buf           Buffer
	baselineBuf   Buffer
	models        []TrainedModel
	baselineModel TrainedModel
	iter          int

	infoSet  cfr.InfoSet
	strategy []float32
	baseline []float32
}

func (d *vrdcfrPolicy) currentModel() TrainedModel {
	if len(d.models) == 0 {
		return nil
	}

	return d.models[len(d.models)-1]
}

func (d *vrdcfrPolicy) getInfoSet() cfr.InfoSet {
	if d.infoSet == nil {
		d.infoSet = d.node.InfoSet(d.node.Player())
	}

	return d.infoSet
}

func (d *vrdcfrPolicy) AddRegret(weight float32, samplingQ, instantaneousRegrets []float32) {
	weight *= float32((d.iter + 1) / 2) // Linear CFR.
	for i, r := range instantaneousRegrets {
		// We only save regret samples that were actually traversed.
		if samplingQ[i] > 0 {
			sample := NewExperienceTuple(d.node, weight, i, r)
			d.buf.AddSample(sample)
		}
	}
}

func (d *vrdcfrPolicy) GetStrategy() []float32 {
	if d.strategy == nil {
		model := d.currentModel()
		if model == nil {
			d.strategy = uniformDist(d.node.NumChildren())
		} else {
			d.strategy = model.Predict(d.getInfoSet(), d.node.NumChildren())
		}
	}

	return d.strategy
}

func (d *vrdcfrPolicy) GetBaseline() []float32 {
	if d.baseline == nil {
		if d.baselineModel == nil {
			d.baseline = make([]float32, d.node.NumChildren())
		} else {
			d.baseline = d.baselineModel.Predict(d.getInfoSet(), d.node.NumChildren())
		}
	}

	return d.baseline
}

func (d *vrdcfrPolicy) UpdateBaseline(w float32, action int, value float32) {
	w *= float32((d.iter + 1) / 2) // Linear CFR.
	sample := NewExperienceTuple(d.node, w, action, value)
	d.baselineBuf.AddSample(sample)
}

func (d *vrdcfrPolicy) AddStrategyWeight(w float32) {
	// We perform SD-CFR, so don't save strategy weight samples.
}

func (d *vrdcfrPolicy) GetAverageStrategy() []float32 {
	return getAverageStrategy(d.node, d.models)
}

func init() {
	gob.Register(&VRSingleDeepCFR{})
}
