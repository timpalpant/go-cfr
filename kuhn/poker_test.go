package kuhn

import (
	"bytes"
	"testing"

	"github.com/timpalpant/go-cfr"
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
	testCFR(t, cfr.Params{}, 10000)
}

func TestPoker_ChanceSamplingCFR(t *testing.T) {
	testCFR(t, cfr.Params{
		SampleChanceNodes: true,
	}, 100000)
}

func TestPoker_ExternalSamplingCFR(t *testing.T) {
	testCFR(t, cfr.Params{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
	}, 100000)
}

func TestPoker_CFRPlus(t *testing.T) {
	testCFR(t, cfr.Params{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
		UseRegretMatchingPlus: true,
	}, 100000)
}

func TestPoker_LinearCFR(t *testing.T) {
	testCFR(t, cfr.Params{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
		LinearWeighting:       true,
	}, 100000)
}

func TestPoker_DiscountedCFR(t *testing.T) {
	testCFR(t, cfr.Params{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
		// From https://arxiv.org/pdf/1809.04040.pdf
		//   we found that setting α=3/2, β=0, and γ=2
		//   led to performance that was consistently stronger than CFR+
		DiscountAlpha: 1.5,
		DiscountBeta:  0.0,
		DiscountGamma: 2.0,
	}, 100000)
}

func testCFR(t *testing.T, params cfr.Params, nIter int) {
	root := NewGame()
	cfr := cfr.New(params)
	var expectedValue float32
	for i := 1; i <= nIter; i++ {
		expectedValue += cfr.Run(root)
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}
	}

	tree.VisitInfoSets(root, func(player int, infoSet string) {
		strat := cfr.GetStrategy(player, infoSet)
		if strat != nil {
			t.Logf("[player %d] %6s: check=%.2f bet=%.2f", player, infoSet, strat[0], strat[1])
		}
	})
}

func TestPoker_LoadSave(t *testing.T) {
	root := NewGame()
	csCFR := cfr.New(cfr.Params{})
	var expectedValue float32
	for i := 1; i <= 10; i++ {
		expectedValue += csCFR.Run(root)
	}

	strategy := make(map[string][]float32)
	tree.VisitInfoSets(root, func(player int, infoSet string) {
		strategy[infoSet] = csCFR.GetStrategy(player, infoSet)
	})

	var buf bytes.Buffer
	csCFR.Save(&buf)

	var err error
	csCFR, err = cfr.Load(&buf)
	if err != nil {
		t.Fatal(err)
	}

	tree.VisitInfoSets(root, func(player int, infoSet string) {
		strat := csCFR.GetStrategy(player, infoSet)
		prevStrat := strategy[infoSet]
		if prevStrat == nil && strat == nil {
			return
		} else if len(strat) != len(prevStrat) || strat[0] != prevStrat[0] || strat[1] != prevStrat[1] {
			t.Errorf("failed to reload policy: expected %v, got %v", prevStrat, strat)
		}
	})

	csCFR.Run(root)
}
