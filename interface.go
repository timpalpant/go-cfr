package cfr

import "io"

// NodeType is the type of node in an extensive-form game tree.
type NodeType int

const (
	ChanceNode NodeType = iota
	TerminalNode
	PlayerNode
)

// InfoSet is the observable game history from the point of view of one player.
type InfoSet interface {
	// Key is an identifier used to uniquely look up this InfoSet
	// when accumulating probabilities in tabular CFR.
	//
	// It may be an arbitrary string of bytes and does not need to be
	// human-readable. For example, it could be a simplified abstraction
	// or hash of the full game history.
	Key() string
}

// GameTreeNode is the interface for a node in an extensive-form game tree.
type GameTreeNode interface {
	// NodeType returns the type of game node.
	Type() NodeType

	// Release resources held by this node (including any children).
	Close()

	// The number of direct children of this node.
	NumChildren() int
	// Get the ith child of this node.
	GetChild(i int) GameTreeNode
	// Get the probability of the ith child of this node.
	// May only be called for nodes with Type == Chance.
	GetChildProbability(i int) float64
	// Sample a single child from this Chance node according to the probability
	// distribution over children.
	// Implementations may use SampleChanceNode to sample from the CDF,
	// or implement their own sampling.
	SampleChild() (child GameTreeNode, p float64)

	// Player returns this current node's acting player.
	// It may only be called for nodes with IsChance() == false.
	Player() int
	// InfoSet returns the information set for this node for the given player.
	InfoSet(player int) InfoSet
	// Utility returns this node's utility for the given player.
	// It must only be called for nodes with type == Terminal.
	Utility(player int) float64
}

// StrategyProfile maintains a collection of regret-matching policies for each
// player node in the game tree.
type StrategyProfile interface {
	// AddRegret provides new observed instantaneous regrets
	// to add to the total accumulated regret.
	AddRegret(node GameTreeNode, instantaneousRegrets []float32)
	// GetPolicy gets the current vector of probabilities with which the ith
	// available action should be played.
	//
	// If the provided slice is not nil, it will be used and extended
	// to the necessary capacity (to allow reuse).
	GetPolicy(node GameTreeNode) []float32
	// AddStrategyWeight adds the current strategy with weight w to the average.
	AddStrategyWeight(node GameTreeNode, w float32)
	// GetAverageStrategy returns the average strategy over all iterations.
	GetAverageStrategy(node GameTreeNode) []float32

	// Calculate the next strategy profile for all visited nodes.
	Update()
	// Get the current iteration (number of times update has been called).
	Iter() int
	// Serialize the current state of the strategy profile to the given io.Writer.
	MarshalTo(w io.Writer) error
}
