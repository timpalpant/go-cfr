package kuhn

import (
	"testing"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/tree"
)

func TestPoker_GameTree(t *testing.T) {
	root := newRootNode()

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
	root := newRootNode()
	nInfoSets := tree.CountInfoSets(root)
	if nInfoSets != 12 {
		t.Errorf("expected %d nodes, got %d", 12, nInfoSets)
	}
}

func TestPoker_VanillaCFR(t *testing.T) {
	game := NewGame()
	vanillaCFR := cfr.NewVanilla(game)
	expectedValue := 0.0
	nIter := 10000
	for i := 1; i <= nIter; i++ {
		expectedValue += vanillaCFR.Run(game.RootNode())
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float64(i))
		}
	}
}
