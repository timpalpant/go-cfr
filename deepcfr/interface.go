package deepcfr

import (
	"io"

	"github.com/timpalpant/go-cfr"
)

// Buffer collects samples of infoset action advantages to train a Model.
type Buffer interface {
	AddSample(s Sample)
	GetSamples() []Sample
	io.Closer
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
