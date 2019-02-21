package kuhn

import (
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

	tree.VisitInfoSets(root, func(player int, infoSet cfr.InfoSet) {
		strat := vanillaCFR.GetStrategy(player, infoSet)
		if strat != nil {
			t.Logf("[player %d] %v: check=%.2f bet=%.2f", player, infoSet, strat[0], strat[1])
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

	tree.VisitInfoSets(root, func(player int, infoSet cfr.InfoSet) {
		strat := csCFR.GetStrategy(player, infoSet)
		if strat != nil {
			t.Logf("[player %d] %v: check=%.2f bet=%.2f", player, infoSet, strat[0], strat[1])
		}
	})
}
