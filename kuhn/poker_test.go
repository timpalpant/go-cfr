package kuhn

import (
	"testing"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/deepcfr"
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
	testCFR(t, cfr.SamplingParams{}, cfr.DiscountParams{}, 10000)
}

func TestPoker_ChanceSamplingCFR(t *testing.T) {
	testCFR(t, cfr.SamplingParams{SampleChanceNodes: true}, cfr.DiscountParams{}, 100000)
}

func TestPoker_ExternalSamplingCFR(t *testing.T) {
	es := cfr.SamplingParams{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
	}

	testCFR(t, es, cfr.DiscountParams{}, 100000)
}

func TestPoker_CFRPlus(t *testing.T) {
	testCFR(t, cfr.SamplingParams{}, cfr.DiscountParams{UseRegretMatchingPlus: true}, 10000)
}

func TestPoker_LinearCFR(t *testing.T) {
	es := cfr.SamplingParams{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
	}
	linear := cfr.DiscountParams{LinearWeighting: true}
	testCFR(t, es, linear, 10000)
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

	testCFR(t, cfr.SamplingParams{}, abg, 10000)
}

func testCFR(t *testing.T, params cfr.SamplingParams, discounts cfr.DiscountParams, nIter int) {
	root := NewGame()
	policy := cfr.NewStrategyTable(discounts)
	opt := cfr.New(params, policy)
	var expectedValue float32
	for i := 1; i <= nIter; i++ {
		expectedValue += opt.Run(root)
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}

		policy.Update()
	}

	seen := make(map[cfr.NodeStrategy]struct{})
	tree.Visit(root, func(node cfr.GameTreeNode) {
		if node.Type() != cfr.PlayerNode {
			return
		}

		node.BuildChildren()
		defer node.FreeChildren()
		strat := policy.GetStrategy(node)
		if _, ok := seen[strat]; ok {
			return
		}

		actionProbs := strat.GetAverageStrategy()
		if actionProbs != nil {
			t.Logf("%6s: check=%.2f bet=%.2f", node, actionProbs[0], actionProbs[1])
		}

		seen[strat] = struct{}{}
	})
}

type randomGuessModel struct{}

func (m randomGuessModel) Train(samples deepcfr.Buffer) {}

func (m randomGuessModel) Predict(infoSet cfr.InfoSet, nActions int) []float32 {
	result := make([]float32, nActions)
	for i := range result {
		result[i] = 1.0 / float32(nActions)
	}

	return result
}

func TestPoker_DeepCFR(t *testing.T) {
	buf := deepcfr.NewReservoirBuffer(10)
	deepCFR := deepcfr.New(&randomGuessModel{}, buf)
	params := cfr.SamplingParams{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
	}

	root := NewGame()
	opt := cfr.New(params, deepCFR)
	for i := 1; i <= 1000; i++ {
		opt.Run(root)
	}

	deepCFR.Update()

	for i := 1; i <= 1000; i++ {
		opt.Run(root)
	}

	for i, sample := range buf.GetSamples() {
		t.Logf("Sample %d: %v", i, sample)
	}
}
