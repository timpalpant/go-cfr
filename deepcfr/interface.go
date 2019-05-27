package deepcfr

import (
	"encoding"
	"io"

	"github.com/timpalpant/go-cfr"
)

type Sample interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// Buffer collects samples of infoset action advantages to train a Model.
type Buffer interface {
	AddSample(Sample)
	GetSamples() []Sample
	io.Closer
}

// Model is a regression model that can be used to fit the given samples.
type Model interface {
	Train(samples []Sample) TrainedModel
}

// TrainedModel is a regression model to use in DeepCFR that predicts
// a vector of advantages for a given InfoSet.
type TrainedModel interface {
	Predict(infoSet cfr.InfoSet, nActions int) (advantages []float32)
}
