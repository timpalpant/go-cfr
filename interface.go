package cfr

type NodeType int

const (
	ChanceNode NodeType = iota
	TerminalNode
	PlayerNode
)

// GameTreeNode is the interface for a node in an extensive-form game tree.
type GameTreeNode interface {
	// NodeType returns the type of game node.
	Type() NodeType

	// NumChildren is the number of direct descendants of this node.
	NumChildren() int
	// GetChild returns the i'th child of this node.
	GetChild(i int) GameTreeNode
	// GetChildProbability returns the probability of proceeding
	// to the i'th child of this node.
	//
	// GetChildProbability may only be called for nodes with type == Chance.
	GetChildProbability(i int) float64

	// Player returns this current node's acting player.
	//
	// It may only be called for nodes with IsChance() == false.
	Player() int
	// InfoSet returns the information set for the given player.
	InfoSet(player int) string
	// Utility returns this node's utility for the given player.
	// It must only be called for nodes with type == Terminal.
	Utility(player int) float64

	// Reset may be called to free temporary resources associated with this
	// node when an algorithm is done using it.
	Reset()
}

// CFR is the interface implemented by the various CFR algorithms.
type CFR interface {
	// Run one iteration of CFR on the tree rooted at the given GameNode
	// and return the expected value for the first player.
	Run(node GameTreeNode) (expectedValue float64)

	// Get the current strategy for the given player's information set.
	GetStrategy(player int, infoSet string) []float64
}
