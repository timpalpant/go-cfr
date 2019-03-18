package deepcfr

import (
	"encoding/gob"
	"io"
)

// Marshal the current state to the given io.Writer.
// Note that to be able to use this method, the concrete types
// implementing Model, TrainedModel, and cfr.InfoSet must be registered
// with gob.
func (d *DeepCFR) MarshalTo(w io.Writer) error {
	enc := gob.NewEncoder(w)

	// Need to pass pointer to interface so that Gob sees the interface rather
	// than the concrete type. See the example in encoding/gob.
	if err := enc.Encode(&d.model); err != nil {
		return err
	}

	if err := enc.Encode(d.buffers); err != nil {
		return err
	}

	if err := enc.Encode(d.trainedModels); err != nil {
		return err
	}

	if err := enc.Encode(d.iter); err != nil {
		return err
	}

	return nil
}

// Reload the current state from the given io.Reader.
func Load(r io.Reader) (*DeepCFR, error) {
	dec := gob.NewDecoder(r)

	var model Model
	if err := dec.Decode(&model); err != nil {
		return nil, err
	}

	var buffers []Buffer
	if err := dec.Decode(&buffers); err != nil {
		return nil, err
	}

	var trainedModels [][]TrainedModel
	if err := dec.Decode(&trainedModels); err != nil {
		return nil, err
	}

	var iter int
	if err := dec.Decode(&iter); err != nil {
		return nil, err
	}

	return &DeepCFR{
		model:         model,
		buffers:       buffers,
		trainedModels: trainedModels,
		iter:          iter,
	}, nil
}
