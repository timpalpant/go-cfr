package cfr

type NodeType int

const (
	ChanceNode NodeType = iota
	TerminalNode
	PlayerNode
)

// Visitor is a function to observe each of the children of a GameTreeNode.
// For nodes that are not ChanceNode, the probability is undefined.
type Visitor func(node GameTreeNode, p float64)

// GameTreeNode is the interface for a node in an extensive-form game tree.
type GameTreeNode interface {
	// NodeType returns the type of game node.
	Type() NodeType

	// Visit each of the direct children of this node once.
	VisitChildren(Visitor)

	// Player returns this current node's acting player.
	//
	// It may only be called for nodes with IsChance() == false.
	Player() int
	// InfoSet returns the information set for the given player.
	InfoSet(player int) string
	// Utility returns this node's utility for the given player.
	// It must only be called for nodes with type == Terminal.
	Utility(player int) float64
}

// CFR is the interface implemented by the various CFR algorithms.
type CFR interface {
	// Run one iteration of CFR on the tree rooted at the given GameNode
	// and return the expected value for the first player.
	Run(node GameTreeNode) (expectedValue float64)

	// Get the current strategy for the given player's information set.
	GetStrategy(player int, infoSet string) []float64
}
