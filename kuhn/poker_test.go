package kuhn

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/deepcfr"
	"github.com/timpalpant/go-cfr/sampling"
	"github.com/timpalpant/go-cfr/tree"
)

func TestPoker_GameTree(t *testing.T) {
	root := NewGame()

	nNodes := tree.CountNodes(root)
	if nNodes != 58 {
		t.Errorf("expected %d nodes, got %d", 58, nNodes)
	}

	nTerminal := tree.CountTerminalNodes(root)
	if nTerminal != 30 {
		t.Errorf("expected %d terminal nodes, got %d", 30, nTerminal)
	}
}

func TestPoker_InfoSets(t *testing.T) {
	root := NewGame()
	nInfoSets := tree.CountInfoSets(root)
	if nInfoSets != 12 {
		t.Errorf("expected %d nodes, got %d", 12, nInfoSets)
	}
}

func TestPoker_VanillaCFR(t *testing.T) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	opt := cfr.New(policy)
	testCFR(t, opt, policy, 10000)
}

func TestPoker_ChanceSamplingCFR(t *testing.T) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	opt := cfr.NewChanceSampling(policy)
	testCFR(t, opt, policy, 200000)
}

func TestPoker_ExternalSamplingCFR(t *testing.T) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	es := sampling.NewExternalSampler()
	opt := cfr.NewGeneralizedSampling(policy, es)
	testCFR(t, opt, policy, 200000)
}

func TestPoker_OutcomeSamplingCFR(t *testing.T) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	os := sampling.NewOutcomeSampler(0.05)
	opt := cfr.NewGeneralizedSampling(policy, os)
	testCFR(t, opt, policy, 200000)
}

func TestPoker_AverageStrategySamplingCFR(t *testing.T) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	params := sampling.AverageStrategyParams{
		Epsilon: 0.05,
		Beta:    1000000,
		Tau:     1000,
	}
	as := sampling.NewAverageStrategySampler(params)
	opt := cfr.NewGeneralizedSampling(policy, as)
	testCFR(t, opt, policy, 200000)
}

func TestPoker_RobustSamplingCFR(t *testing.T) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	rs := sampling.NewRobustSampler(1)
	opt := cfr.NewGeneralizedSampling(policy, rs)
	testCFR(t, opt, policy, 200000)
}

func TestPoker_CFRPlus(t *testing.T) {
	plus := cfr.DiscountParams{UseRegretMatchingPlus: true}
	policy := cfr.NewPolicyTable(plus)
	es := sampling.NewExternalSampler()
	opt := cfr.NewGeneralizedSampling(policy, es)
	testCFR(t, opt, policy, 200000)
}

func TestPoker_LinearCFR(t *testing.T) {
	linear := cfr.DiscountParams{LinearWeighting: true}
	policy := cfr.NewPolicyTable(linear)
	es := sampling.NewExternalSampler()
	opt := cfr.NewGeneralizedSampling(policy, es)
	testCFR(t, opt, policy, 200000)
}

func TestPoker_DiscountedCFR(t *testing.T) {
	abg := cfr.DiscountParams{
		// From https://arxiv.org/pdf/1809.04040.pdf
		//   we found that setting α=3/2, β=0, and γ=2
		//   led to performance that was consistently stronger than CFR+
		DiscountAlpha: 1.5,
		DiscountBeta:  0.0,
		DiscountGamma: 2.0,
	}

	policy := cfr.NewPolicyTable(abg)
	opt := cfr.New(policy)
	testCFR(t, opt, policy, 10000)
}

func BenchmarkPoker_VanillaCFR(b *testing.B) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	opt := cfr.New(policy)
	b.ResetTimer()
	runCFR(b, opt, policy, b.N)
}

func BenchmarkPoker_ChanceSamplingCFR(b *testing.B) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	opt := cfr.NewChanceSampling(policy)
	b.ResetTimer()
	runCFR(b, opt, policy, b.N)
}

func BenchmarkPoker_ExternalSamplingCFR(b *testing.B) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	es := sampling.NewExternalSampler()
	opt := cfr.NewGeneralizedSampling(policy, es)
	b.ResetTimer()
	runCFR(b, opt, policy, b.N)
}

func BenchmarkPoker_AverageStrategySamplingCFR(b *testing.B) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	params := sampling.AverageStrategyParams{
		Epsilon: 0.05,
		Beta:    1000000,
		Tau:     1000,
	}
	as := sampling.NewAverageStrategySampler(params)
	opt := cfr.NewGeneralizedSampling(policy, as)
	b.ResetTimer()
	runCFR(b, opt, policy, b.N)
}

func BenchmarkPoker_OutcomeSamplingCFR(b *testing.B) {
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	os := sampling.NewOutcomeSampler(0.05)
	opt := cfr.NewGeneralizedSampling(policy, os)
	b.ResetTimer()
	runCFR(b, opt, policy, b.N)
}

type cfrImpl interface {
	Run(cfr.GameTreeNode) float32
}

func testCFR(t *testing.T, opt cfrImpl, policy cfr.StrategyProfile, nIter int) {
	root := runCFR(t, opt, policy, nIter)
	seen := make(map[string]struct{})
	tree.Visit(root, func(node cfr.GameTreeNode) {
		if node.Type() != cfr.PlayerNode {
			return
		}

		key := node.InfoSet(node.Player()).Key()
		if _, ok := seen[key]; ok {
			return
		}

		actionProbs := policy.GetPolicy(node).GetAverageStrategy()
		if actionProbs != nil {
			t.Logf("%6s: check=%.2f bet=%.2f", node, actionProbs[0], actionProbs[1])
		}

		seen[key] = struct{}{}
	})
}

type logger interface {
	Logf(string, ...interface{})
}

func runCFR(log logger, opt cfrImpl, policy cfr.StrategyProfile, nIter int) cfr.GameTreeNode {
	root := NewGame()
	var expectedValue float32
	for i := 1; i <= nIter; i++ {
		expectedValue += opt.Run(root)
		if nIter/10 > 0 && i%(nIter/10) == 0 {
			log.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}

		policy.Update()
	}

	return root
}

type randomGuessModel struct{}

func (m randomGuessModel) Train(samples deepcfr.Buffer) deepcfr.TrainedModel {
	return m
}

func (m randomGuessModel) Predict(infoSet cfr.InfoSet, nActions int) []float32 {
	result := make([]float32, nActions)
	for i := range result {
		result[i] = 1.0 / float32(nActions)
	}

	return result
}

func TestPoker_DeepCFR(t *testing.T) {
	model := &randomGuessModel{}
	gob.Register(model)
	buf0 := deepcfr.NewReservoirBuffer(10, 1)
	buf1 := deepcfr.NewReservoirBuffer(10, 1)
	deepCFR := deepcfr.New(model, []deepcfr.Buffer{buf0, buf1}, true)
	root := NewGame()
	es := sampling.NewExternalSampler()
	opt := cfr.NewGeneralizedSampling(deepCFR, es)
	for i := 1; i <= 1000; i++ {
		opt.Run(root)
	}

	deepCFR.Update()

	for i := 1; i <= 1000; i++ {
		opt.Run(root)
	}

	t.Logf("Buffer 0:")
	for i, sample := range buf0.GetSamples() {
		t.Logf("Sample %d: %v", i, sample)
	}

	t.Logf("Buffer 1:")
	for i, sample := range buf1.GetSamples() {
		t.Logf("Sample %d: %v", i, sample)
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(deepCFR); err != nil {
		t.Error(err)
	}

	dec := gob.NewDecoder(&buf)
	var reloaded deepcfr.DeepCFR
	if err := dec.Decode(&reloaded); err != nil {
		t.Error(err)
	}
}

func TestMarshalStrategy(t *testing.T) {
	root := NewGame()
	policy := cfr.NewPolicyTable(cfr.DiscountParams{})
	opt := cfr.New(policy)
	opt.Run(root)
	policy.Update()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(policy); err != nil {
		t.Error(err)
	}

	dec := gob.NewDecoder(&buf)
	var reloaded cfr.PolicyTable
	if err := dec.Decode(&reloaded); err != nil {
		t.Error(err)
	}

	// Verify that current strategy and average strategy are unchanged
	// after marshalling round trip.
	tree.Visit(root, func(node cfr.GameTreeNode) {
		if node.Type() != cfr.PlayerNode {
			return
		}

		p1 := policy.GetPolicy(node).GetStrategy()
		p2 := reloaded.GetPolicy(node).GetStrategy()
		if len(p1) != len(p2) {
			t.Errorf("expected %v, got %v", p1, p2)
		} else {
			for i := 0; i < len(p1); i++ {
				if p1[i] != p2[i] {
					t.Errorf("expected %v, got %v", p1, p2)
					break
				}
			}
		}

		avgStrat1 := policy.GetPolicy(node).GetAverageStrategy()
		avgStrat2 := reloaded.GetPolicy(node).GetAverageStrategy()
		if !reflect.DeepEqual(avgStrat1, avgStrat2) {
			t.Errorf("expected %v, got %v", avgStrat1, avgStrat2)
		}
	})
}
