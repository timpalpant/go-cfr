package cfr

import (
	"testing"
)

const (
	chance  = -1
	player0 = 0
	player1 = 1

	random = 'r'
	check  = 'c'
	bet    = 'b'

	jack = iota
	queen
	king
)

type KuhnPoker struct {
	root KuhnPokerNode
}

func (k KuhnPoker) NumPlayers() int {
	return 2
}

func (k KuhnPoker) RootNode() GameTreeNode {
	return k.root
}

// KuhnPokerNode implements GameTreeNode for Kuhn Poker.
// Loosely adapted from: https://justinsermeno.com/posts/cfr/.
type KuhnPokerNode struct {
	player        int
	children      []KuhnPokerNode
	probabilities []float64
	history       []byte

	// Private card held by either player.
	p0Card, p1Card byte
}

func NewKuhnPokerTree() KuhnPokerNode {
	deals := buildP0Deals()
	return KuhnPokerNode{
		player:        chance,
		children:      deals,
		probabilities: uniformDist(len(deals)),
	}
}

func buildP0Deals() []KuhnPokerNode {
	var result []KuhnPokerNode
	for _, card := range []byte{jack, queen, king} {
		child := KuhnPokerNode{
			player:  chance,
			history: []byte{random},
			p0Card:  card,
		}

		child.children = buildP1Deals(child)
		child.probabilities = uniformDist(len(child.children))
		result = append(result, child)
	}

	return result
}

func buildP1Deals(parent KuhnPokerNode) []KuhnPokerNode {
	var result []KuhnPokerNode
	for _, card := range []byte{jack, queen, king} {
		if card == parent.p0Card {
			continue
		}

		child := parent
		child.player = player0
		child.p1Card = card
		child.history = append([]byte(nil), parent.history...)
		child.history = append(child.history, random)
		child.children = buildRound1Children(child)
		result = append(result, child)
	}

	return result

}

func buildRound1Children(parent KuhnPokerNode) []KuhnPokerNode {
	var result []KuhnPokerNode
	for _, choice := range []byte{check, bet} {
		child := parent
		child.player = player1
		child.history = append([]byte(nil), parent.history...)
		child.history = append(child.history, choice)
		child.children = buildRound2Children(child)
		result = append(result, child)
	}
	return result
}

func buildRound2Children(parent KuhnPokerNode) []KuhnPokerNode {
	var result []KuhnPokerNode
	for _, choice := range []byte{check, bet} {
		child := parent
		child.player = player0
		child.history = append([]byte(nil), parent.history...)
		child.history = append(child.history, choice)
		child.children = buildFinalChildren(child)
		result = append(result, child)
	}
	return result
}

func buildFinalChildren(parent KuhnPokerNode) []KuhnPokerNode {
	var result []KuhnPokerNode
	if parent.history[2] == check && parent.history[3] == bet {
		for _, choice := range []byte{check, bet} {
			child := parent
			child.history = append([]byte(nil), parent.history...)
			child.history = append(child.history, choice)
			result = append(result, child)
		}
	}

	return result
}

func (k KuhnPokerNode) NumChildren() int {
	return len(k.children)
}

func (k KuhnPokerNode) GetChild(i int) GameTreeNode {
	return k.children[i]
}

func (k KuhnPokerNode) IsChance() bool {
	return k.player == chance
}

func (k KuhnPokerNode) GetChildProbability(i int) float64 {
	return k.probabilities[i]
}

func (k KuhnPokerNode) Player() int {
	return k.player
}

func (k KuhnPokerNode) Utility() float64 {
	var cardPlayer, cardOpponent byte
	if k.player == player1 {
		cardPlayer = k.p0Card
		cardOpponent = k.p1Card
	} else {
		cardPlayer = k.p1Card
		cardOpponent = k.p0Card
	}

	h := string(k.history)
	if h == "rrcbc" || h == "rrbc" {
		// Last player folded. The current player wins.
		return 1.0
	} else if h == "rrcc" {
		// Showdown with no bets.
		if cardPlayer > cardOpponent {
			return 1.0
		}

		return -1.0
	}

	// Showdown with 1 bet.
	if h != "rrcbb" && h != "rrbb" {
		panic("unexpected history: " + h)
	}

	if cardPlayer > cardOpponent {
		return 2.0
	}

	return -2.0
}

func (k KuhnPokerNode) InfoSet() [20]byte {
	var result [20]byte
	if k.player == player0 {
		result[0] = k.p0Card
	} else if k.player == player1 {
		result[0] = k.p1Card
	}

	copy(result[1:], k.history)
	return result
}

func TestKuhnPoker_GameTree(t *testing.T) {
	kuhn := NewKuhnPokerTree()

	nNodes := CountNodes(kuhn)
	if nNodes != 58 {
		t.Errorf("expected %d nodes, got %d", 58, nNodes)
	}

	nTerminal := CountTerminalNodes(kuhn)
	if nTerminal != 30 {
		t.Errorf("expected %d terminal nodes, got %d", 30, nTerminal)
	}
}

func TestKuhnPoker_InfoSets(t *testing.T) {
	kuhn := NewKuhnPokerTree()
	nInfoSets := CountInfoSets(kuhn)
	if nInfoSets != 12 {
		t.Errorf("expected %d nodes, got %d", 12, nInfoSets)
	}
}

func TestKuhnPoker_VanillaCFR(t *testing.T) {
	root := NewKuhnPokerTree()
	kuhn := KuhnPoker{root}
	cfr := NewVanilla(kuhn)
	expectedValue := 0.0
	nIter := 10000
	for i := 1; i <= nIter; i++ {
		expectedValue += cfr.Run(root) / float64(nIter)
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue)
		}
	}
}
