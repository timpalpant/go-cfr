package deepcfr

import (
	"bytes"
	"encoding/gob"

	"github.com/timpalpant/go-cfr"
)

// OnlineOnlineDeepCFR implements cfr.OnlineStrategyProfile.
//
// During CFR iterations, samples are added to the given buffer.
// When Update is called, the model is retrained.
type OnlineDeepCFR struct {
	model            Model
	avgStrategyModel Model
	buffer           Buffer
	trainedModels    []TrainedModel
}

// New returns a new OnlineDeepCFR policy with the given model and sample buffer.
func NewOnline(regretModel, avgStrategyModel Model, buffer Buffer) *OnlineDeepCFR {
	return &OnlineDeepCFR{
		model:            regretModel,
		avgStrategyModel: avgStrategyModel,
		buffer:           buffer,
	}
}

// GetPolicy implements cfr.OnlineStrategyProfile.
func (d *OnlineDeepCFR) GetPolicy(node cfr.GameTreeNode) cfr.OnlineNodePolicy {
	var currentModel TrainedModel
	if len(d.trainedModels) > 0 {
		currentModel = d.trainedModels[len(d.trainedModels)-1]
	}

	return &onlineDCFRPolicy{
		policy: &modelBasedPolicy{
			node:  node,
			model: currentModel,
		},
		node: node,
		buf:  d.buffer,
	}
}

// Update implements cfr.OnlineStrategyProfile.
func (d *OnlineDeepCFR) Update() {
	trained := d.model.Train(d.buffer)
	d.trainedModels = append(d.trainedModels, trained)
}

// GetAverageStrategy implements cfr.OnlineStrategyProfile.
func (d *OnlineDeepCFR) GetAverageStrategy() cfr.StrategyProfile {
	trainedModel := d.avgStrategyModel.Train(d.buffer)
	return &modelStrategyProfile{
		model: trainedModel,
	}
}

// Iter implements cfr.OnlineStrategyProfile.
func (d *OnlineDeepCFR) Iter() int {
	return len(d.trainedModels)
}

// Close implements cfr.OnlineStrategyProfile.
func (d *OnlineDeepCFR) Close() error {
	return d.buffer.Close()
}

// MarshalBinary implements encoding.BinaryMarshaler.
// Note that to be able to use this method, the concrete types
// implementing the Model, TrainedModel, and Buffers must be registered
// with gob.
func (d *OnlineDeepCFR) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Need to pass pointer to interface so that Gob sees the interface rather
	// than the concrete type. See the example in encoding/gob.
	if err := enc.Encode(&d.model); err != nil {
		return nil, err
	}

	if err := enc.Encode(&d.buffer); err != nil {
		return nil, err
	}

	if err := enc.Encode(d.trainedModels); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (d *OnlineDeepCFR) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&d.model); err != nil {
		return err
	}

	if err := dec.Decode(&d.buffer); err != nil {
		return err
	}

	if err := dec.Decode(&d.trainedModels); err != nil {
		return err
	}

	return nil
}

type onlineDCFRPolicy struct {
	node   cfr.GameTreeNode
	buf    Buffer
	policy *modelBasedPolicy
}

func (d *onlineDCFRPolicy) AddExperienceTuple(weight float32, action int, value, regret float32) {
	// TODO: Linear CFR weighting?
	sample := NewExperienceTuple(d.node, weight, action, value, regret)
	d.buf.AddSample(sample)
}

func (d *onlineDCFRPolicy) GetStrategy() []float32 {
	return d.policy.GetStrategy()
}

func init() {
	gob.Register(&OnlineDeepCFR{})
}
