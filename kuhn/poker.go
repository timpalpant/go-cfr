// Package kuhn implements an extensive-form game tree for Kuhn Poker,
// adapted from: https://justinsermeno.com/posts/cfr/.
package kuhn

import (
	"fmt"

	"github.com/timpalpant/go-cfr"
)

const (
	chance  = -1
	player0 = 0
	player1 = 1
)

type Action byte

const (
	Random = 'r'
	Check  = 'c'
	Bet    = 'b'
)

type Card int

const (
	Jack Card = iota
	Queen
	King
)

var cardStr = [...]string{
	"J",
	"Q",
	"K",
}

func (c Card) String() string {
	return cardStr[c]
}

// Poker implements cfr.ExtensiveFormGame for Kuhn Poker.
type Poker struct {
	root PokerNode
}

// PokerNode implements cfr.GameTreeNode for Kuhn Poker.
type PokerNode struct {
	player        int
	children      []PokerNode
	probabilities []float64
	history       string

	// Private card held by either player.
	p0Card, p1Card Card
}

func NewGame() PokerNode {
	deals := buildP0Deals()
	return PokerNode{
		player:        chance,
		children:      deals,
		probabilities: uniformDist(len(deals)),
	}
}

func buildP0Deals() []PokerNode {
	var result []PokerNode
	for _, card := range []Card{Jack, Queen, King} {
		child := PokerNode{
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

func buildP1Deals(parent PokerNode) []PokerNode {
	var result []PokerNode
	for _, card := range []Card{Jack, Queen, King} {
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

func buildRound1Children(parent PokerNode) []PokerNode {
	var result []PokerNode
	for _, choice := range []byte{Check, Bet} {
		child := parent
		child.player = player1
		child.history += string([]byte{choice})
		child.children = buildRound2Children(child)
		result = append(result, child)
	}
	return result
}

func buildRound2Children(parent PokerNode) []PokerNode {
	var result []PokerNode
	for _, choice := range []byte{Check, Bet} {
		child := parent
		child.player = player0
		child.history += string([]byte{choice})
		child.children = buildFinalChildren(child)
		result = append(result, child)
	}
	return result
}

func buildFinalChildren(parent PokerNode) []PokerNode {
	var result []PokerNode
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

// String implements fmt.Stringer.
func (k PokerNode) String() string {
	return fmt.Sprintf("Player %v's turn. History: %s [Cards: P0 - %s, P1 - %s]",
		k.player, k.history, k.p0Card, k.p1Card)
}

// NumChildren implements cfr.GameTreeNode.
func (k PokerNode) NumChildren() int {
	return len(k.children)
}

// GetChild implements cfr.GameTreeNode.
func (k PokerNode) GetChild(i int) cfr.GameTreeNode {
	return k.children[i]
}

// IsChance implements cfr.GameTreeNode.
func (k PokerNode) IsChance() bool {
	return k.player == chance
}

// GetChildProbability implements cfr.GameTreeNode.
func (k PokerNode) GetChildProbability(i int) float64 {
	return k.probabilities[i]
}

// Player implements cfr.GameTreeNode.
func (k PokerNode) Player() int {
	return k.player
}

// Utility implements cfr.GameTreeNode.
func (k PokerNode) Utility(player int) float64 {
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

// InfoSet implements cfr.GameTreeNode.
func (k PokerNode) InfoSet(player int) string {
	return k.playerCard(player).String() + "-" + k.history
}

func (k PokerNode) playerCard(player int) Card {
	if player == player0 {
		return k.p0Card
	}

	return k.p1Card
}

func uniformDist(n int) []float64 {
	result := make([]float64, n)
	for i := range result {
		result[i] = 1.0 / float64(n)
	}
	return result
}
