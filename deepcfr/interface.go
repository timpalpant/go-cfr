package deepcfr

import (
	"io"

	"github.com/timpalpant/go-cfr"
)

// Samples must be binary marshalable, but embedding that interface breaks gob decoding.
// Ref: https://stackoverflow.com/questions/43324919/gob-panics-decoding-an-interface
type Sample interface{}

// Buffer collects samples of infoset action advantages to train a Model.
type Buffer interface {
	AddSample(Sample)
	GetSample(idx int) Sample
	GetSamples() []Sample
	Len() int
	io.Closer
}

// Model is a regression model that can be used to fit the given samples.
type Model interface {
	Train(buffer Buffer) TrainedModel
}

// TrainedModel is a regression model to use in DeepCFR that predicts
// a vector of advantages for a given InfoSet.
type TrainedModel interface {
	Predict(infoSet cfr.InfoSet, nActions int) (advantages []float32)
}
