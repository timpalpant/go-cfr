package cfr

import (
	"fmt"
	"testing"
)

const (
	chance  = -1
	player0 = 0
	player1 = 1
)

type KuhnAction byte

const (
	Random = 'r'
	Check  = 'c'
	Bet    = 'b'
)

type KuhnCard int

const (
	Jack KuhnCard = iota
	Queen
	King
)

var cardStr = [...]string{
	"J",
	"Q",
	"K",
}

func (c KuhnCard) String() string {
	return cardStr[c]
}

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
	history       string

	// Private card held by either player.
	p0Card, p1Card KuhnCard
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
	for _, card := range []KuhnCard{Jack, Queen, King} {
		child := KuhnPokerNode{
			player:  chance,
			history: string(Random),
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
	for _, card := range []KuhnCard{Jack, Queen, King} {
		if card == parent.p0Card {
			continue // Both players can't be dealt the same card.
		}

		child := parent
		child.player = player0
		child.p1Card = card
		child.history += string([]byte{Random})
		child.children = buildRound1Children(child)
		result = append(result, child)
	}

	return result

}

func buildRound1Children(parent KuhnPokerNode) []KuhnPokerNode {
	var result []KuhnPokerNode
	for _, choice := range []byte{Check, Bet} {
		child := parent
		child.player = player1
		child.history += string([]byte{choice})
		child.children = buildRound2Children(child)
		result = append(result, child)
	}
	return result
}

func buildRound2Children(parent KuhnPokerNode) []KuhnPokerNode {
	var result []KuhnPokerNode
	for _, choice := range []byte{Check, Bet} {
		child := parent
		child.player = player0
		child.history += string([]byte{choice})
		child.children = buildFinalChildren(child)
		result = append(result, child)
	}
	return result
}

func buildFinalChildren(parent KuhnPokerNode) []KuhnPokerNode {
	var result []KuhnPokerNode
	if parent.history[2] == Check && parent.history[3] == Bet {
		for _, choice := range []byte{Check, Bet} {
			child := parent
			child.player = player1
			child.history += string([]byte{choice})
			result = append(result, child)
		}
	}

	return result
}

func (k KuhnPokerNode) String() string {
	return fmt.Sprintf("Player %v's turn. History: %s [Cards: P0 - %s, P1 - %s]",
		k.player, k.history, k.p0Card, k.p1Card)
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

func (k KuhnPokerNode) playerCard(player int) KuhnCard {
	if player == player0 {
		return k.p0Card
	}

	return k.p1Card
}

func (k KuhnPokerNode) Utility(player int) float64 {
	cardPlayer := k.playerCard(player)
	cardOpponent := k.playerCard(1 - player)

	// By convention, terminal nodes are labeled with the player whose
	// turn it would be (i.e. not the last acting player).

	if k.history == "rrcbc" || k.history == "rrbc" {
		// Last player folded. The current player wins.
		if k.player == player {
			return 1.0
		} else {
			return -1.0
		}
	} else if k.history == "rrcc" {
		// Showdown with no bets.
		if cardPlayer > cardOpponent {
			return 1.0
		} else {
			return -1.0
		}
	}

	// Showdown with 1 bet.
	if k.history != "rrcbb" && k.history != "rrbb" {
		panic("unexpected history: " + k.history)
	}

	if cardPlayer > cardOpponent {
		return 2.0
	}

	return -2.0
}

func (k KuhnPokerNode) InfoSet(player int) string {
	if player == player0 {
		return k.p0Card.String() + k.history
	} else {
		return k.p1Card.String() + k.history
	}
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
		expectedValue += cfr.Run(root)
		if i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float64(i))
		}
	}
}
