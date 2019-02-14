package cfr

type InfoSet = [20]byte

type Game interface {
	NumPlayers() int
	RootNode() GameTreeNode
}

// GameTreeNode is the interface for a node in an extensive-form game tree.
type GameTreeNode interface {
	// NumChildren is the number of direct descendants of this node.
	NumChildren() int
	// GetChild returns the i'th child of this node.
	GetChild(i int) GameTreeNode

	// IsChance returns true if this node is controlled by Nature rather
	// than one of the players.
	IsChance() bool
	// GetChildProbability returns the probability of proceeding
	// to the i'th child of this node.
	//
	// GetChildProbability may only be called for nodes with IsChance() == true.
	GetChildProbability(i int) float64

	// Player returns this current node's acting player.
	Player() int
	// InfoSet returns the information set for the acting player.
	InfoSet() InfoSet
	// Utility returns this node's utility for the given player.
	// It must only be called for terminal nodes.
	Utility() float64
}

type CFR interface {
	// Run one iteration of CFR on the subtree rooted at the given GameNode
	// and return the expected value.
	Run(node GameTreeNode) (expectedValue float64)
}
