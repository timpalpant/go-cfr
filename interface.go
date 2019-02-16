package cfr

type ExtensiveFormGame interface {
	// The number of non-chance players in the game.
	NumPlayers() int
	// The root node at the start of the game.
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
	//
	// It may only be called for nodes with IsChance() == false.
	Player() int
	// InfoSet returns the information set for the given player.
	InfoSet(player int) string
	// Utility returns this node's utility for the given player.
	// It must only be called for terminal nodes.
	Utility(player int) float64
}

type CFR interface {
	// Run one iteration of CFR on the tree rooted at the given GameNode
	// and return the expected value for the first player.
	Run(node GameTreeNode) (expectedValue float64)

	// Get the current strategy for the given player's information set.
	GetStrategy(player int, infoSet string) []float64
}

// IsTerminal returns true if this node is an end-game node.
func IsTerminal(node GameTreeNode) bool {
	return node.NumChildren() == 0
}
