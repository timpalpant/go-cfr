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
	root := NewGame()
	vanillaCFR := cfr.New(cfr.Params{})
	var expectedValue float32
	nIter := 10000
	for i := 1; i <= nIter; i++ {
		expectedValue += vanillaCFR.Run(root)
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}
	}

	tree.VisitInfoSets(root, func(player int, infoSet string) {
		strat := vanillaCFR.GetStrategy(player, infoSet)
		if strat != nil {
			t.Logf("[player %d] %6s: check=%.2f bet=%.2f", player, infoSet, strat[0], strat[1])
		}
	})
}

func TestPoker_ChanceSamplingCFR(t *testing.T) {
	root := NewGame()
	csCFR := cfr.New(cfr.Params{SampleChanceNodes: true})
	var expectedValue float32
	nIter := 100000
	for i := 1; i <= nIter; i++ {
		expectedValue += csCFR.Run(root)
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}
	}

	tree.VisitInfoSets(root, func(player int, infoSet string) {
		strat := csCFR.GetStrategy(player, infoSet)
		if strat != nil {
			t.Logf("[player %d] %6s: check=%.2f bet=%.2f", player, infoSet, strat[0], strat[1])
		}
	})
}

func TestPoker_ExternalSamplingCFR(t *testing.T) {
	root := NewGame()
	esCFR := cfr.New(cfr.Params{
		SampleChanceNodes:     true,
		SampleOpponentActions: true,
	})
	var expectedValue float32
	nIter := 100000
	for i := 1; i <= nIter; i++ {
		expectedValue += esCFR.Run(root)
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}
	}

	tree.VisitInfoSets(root, func(player int, infoSet string) {
		strat := esCFR.GetStrategy(player, infoSet)
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
