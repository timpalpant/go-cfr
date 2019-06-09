package deepcfr

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"sync"

	"github.com/golang/glog"
	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
)

// SingleDeepCFR implements cfr.StrategyProfile, and uses function approximation
// to estimate strategies rather than accumulation of regrets for all
// infosets. This can be more tractable for large games where storing
// all of the regrets for all infosets is impractical.
//
// During CFR iterations, samples are added to the given buffer.
// When Update is called, the model is retrained.
type SingleDeepCFR struct {
	model         Model
	buffers       []Buffer
	trainedModels [][]TrainedModel
	iter          int
}

// New returns a new SingleDeepCFR policy with the given model and sample buffer.
func NewSingleDeepCFR(model Model, buffers []Buffer) *SingleDeepCFR {
	return &SingleDeepCFR{
		model:   model,
		buffers: buffers,
		trainedModels: [][]TrainedModel{
			[]TrainedModel{},
			[]TrainedModel{},
		},
		iter: 1,
	}
}

func (d *SingleDeepCFR) currentPlayer() int {
	return d.iter % 2
}

func (d *SingleDeepCFR) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &dcfrPolicy{
		node:   node,
		buf:    d.buffers[node.Player()],
		models: d.trainedModels[node.Player()],
		iter:   d.iter,
	}
}

type AdvantageModel struct {
	Model TrainedModel
}

func (m AdvantageModel) Predict(infoSet cfr.InfoSet, nActions int) []float32 {
	return regretMatching(m.Model.Predict(infoSet, nActions))
}

// Update implements cfr.StrategyProfile.
func (d *SingleDeepCFR) Update() {
	player := d.currentPlayer()
	buf := d.buffers[player]
	trained := d.model.Train(buf)
	model := &AdvantageModel{trained}
	d.trainedModels[player] = append(d.trainedModels[player], model)

	d.iter++
}

// Iter implements cfr.StrategyProfile.
func (d *SingleDeepCFR) Iter() int {
	return d.iter
}

func (d *SingleDeepCFR) Close() error {
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
func (d *SingleDeepCFR) MarshalBinary() ([]byte, error) {
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
func (d *SingleDeepCFR) UnmarshalBinary(buf []byte) error {
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
	node     cfr.GameTreeNode
	buf      Buffer
	models   []TrainedModel
	strategy []float32
	iter     int
}

func (d *dcfrPolicy) currentModel() TrainedModel {
	if len(d.models) == 0 {
		return nil
	}

	return d.models[len(d.models)-1]
}

func (d *dcfrPolicy) AddRegret(weight float32, samplingQ, instantaneousRegrets []float32) {
	weight *= float32((d.iter + 1) / 2) // Linear CFR.
	sample := NewRegretSample(d.node, instantaneousRegrets, weight)
	d.buf.AddSample(sample)
}

func (d *dcfrPolicy) GetStrategy() []float32 {
	if d.strategy == nil {
		model := d.currentModel()
		if model == nil {
			d.strategy = uniformDist(d.node.NumChildren())
		} else {
			infoSet := d.node.InfoSet(d.node.Player())
			d.strategy = model.Predict(infoSet, d.node.NumChildren())
		}
	}

	return d.strategy
}

func (d *dcfrPolicy) GetBaseline() []float32 {
	return make([]float32, d.node.NumChildren())
}

func (d *dcfrPolicy) UpdateBaseline(w float32, action int, value float32) {}

func (d *dcfrPolicy) AddStrategyWeight(w float32) {
	// We perform SD-CFR, so don't save strategy weight samples.
}

func (d *dcfrPolicy) GetAverageStrategy() []float32 {
	return getAverageStrategy(d.node, d.models)
}

func getAverageStrategy(node cfr.GameTreeNode, models []TrainedModel) []float32 {
	nChildren := node.NumChildren()
	if nChildren == 1 {
		return []float32{1.0}
	}

	var modelPredictions [][]float32
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		modelPredictions = getModelPredictions(node, models)
		wg.Done()
	}()

	var modelWeights []float32
	wg.Add(1)
	go func() {
		modelWeights = getModelWeights(node, models)
		wg.Done()
	}()

	wg.Wait()

	result := make([]float32, nChildren)
	for t, w := range modelWeights {
		glog.V(3).Infof("[t=%d] Weight: %v, Strategy: %v", t, w, modelPredictions[t])
		f32.AxpyUnitary(w, modelPredictions[t], result)
	}

	return result
}

func getModelPredictions(node cfr.GameTreeNode, models []TrainedModel) [][]float32 {
	var mu sync.Mutex
	var wg sync.WaitGroup
	modelPredictions := make([][]float32, len(models))
	infoSet := node.InfoSet(node.Player())
	nChildren := node.NumChildren()
	for i, model := range models {
		wg.Add(1)
		go func(i int, model TrainedModel) {
			result := model.Predict(infoSet, nChildren)
			mu.Lock()
			modelPredictions[i] = result
			mu.Unlock()
			wg.Done()
		}(i, model)
	}

	wg.Wait()
	return modelPredictions
}

func getModelWeights(node cfr.GameTreeNode, models []TrainedModel) []float32 {
	var mu sync.Mutex
	var wg sync.WaitGroup
	modelWeights := make([]float32, len(models))
	for i := range modelWeights {
		modelWeights[i] = float32(i + 1)
	}

	lastChild := node
	for ancestor := node.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		if ancestor.Type() == cfr.PlayerNodeType && ancestor.Player() == node.Player() {
			nChildren := ancestor.NumChildren()
			childIdx := childIndex(ancestor, lastChild)
			infoSet := ancestor.InfoSet(ancestor.Player())
			for i, model := range models {
				wg.Add(1)
				go func(i int, model TrainedModel) {
					strategy := model.Predict(infoSet, nChildren)
					mu.Lock()
					modelWeights[i] *= strategy[childIdx]
					mu.Unlock()
					wg.Done()
				}(i, model)
			}
		}

		lastChild = ancestor
	}

	wg.Wait()

	normalization := f32.Sum(modelWeights)
	f32.ScalUnitary(1.0/normalization, modelWeights)
	return modelWeights
}

func childIndex(parent, child cfr.GameTreeNode) int {
	childIdx := -1
	nChildren := parent.NumChildren()
	for i := 0; i < nChildren; i++ {
		if parent.GetChild(i) == child {
			childIdx = i
			break
		}
	}

	if childIdx == -1 {
		var children string
		for i := 0; i < nChildren; i++ {
			child := parent.GetChild(i)
			children += fmt.Sprintf("\t%d: (%p) %v\n", i, child, child)
		}
		panic(fmt.Errorf(
			"failed to identify action leading to history!\n\n"+
				"node(%p): %v\nlastChild(%p): %v\nnode children:\n%v",
			parent, parent, child, child, children))
	}

	return childIdx
}

func regretMatching(advantages []float32) []float32 {
	makePositive(advantages)
	if total := f32.Sum(advantages); total > 0 {
		f32.ScalUnitary(1.0/total, advantages)
	} else { // Choose action with highest advantage.
		largest := float32(math.Inf(-1))
		selectedAction := -1
		for i, x := range advantages {
			if x > largest {
				selectedAction = i
			}

			advantages[i] = 0
		}

		advantages[selectedAction] = 1.0
	}

	return advantages
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}

func init() {
	gob.Register(&SingleDeepCFR{})
	gob.Register(&AdvantageModel{})
}
