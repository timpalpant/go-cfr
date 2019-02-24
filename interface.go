package cfr

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

	// Prepare children of this node.
	// Must be called before using NumChildren, GetChild, or GetChildProbability.
	// Implementations are free to ignore the request and generate children
	// on the fly as needed if they choose.
	BuildChildren()
	// Release resources allocated by calling BuildChildren.
	// After calling FreeChildren, NumChildren, GetChild, and GetChildProbability
	// may no longer be called (unless the node is rebuilt).
	FreeChildren()

	// The number of direct children of this node.
	NumChildren() int
	// Get the ith child of this node.
	GetChild(i int) GameTreeNode
	// Get the probability of the ith child of this node.
	// May only be called for nodes with Type == Chance.
	GetChildProbability(i int) float32
	// Sample one of the children of this node, according to the probability
	// distribution. Only applicable for nodes with Type == Chance.
	SampleChild() GameTreeNode

	// Player returns this current node's acting player.
	// It may only be called for nodes with IsChance() == false.
	Player() int
	// InfoSet returns the information set for this node for the given player.
	InfoSet(player int) InfoSet
	// Utility returns this node's utility for the given player.
	// It must only be called for nodes with type == Terminal.
	Utility(player int) float32
}

// NodePolicy learns a strategy for play at a given GameTreeNode.
type NodePolicy interface {
	// GetActionProbability gets the probability with which the ith
	// available action should be played.
	GetActionProbability(i int) float32
	// AddRegret provides new observed instantaneous regrets (with probability p)
	// to add to the total accumulated regret.
	AddRegret(reachP, counterfactualP float32, instantaneousAdvantages []float32)
	// NextStrategy calculates new strategy action probabilities based on the
	// accumulated regret.
	//
	// The provided discount factors correspond to α, β, and γ
	// as configured in by the CFR Params. NodePolicies are free to ignore them.
	NextStrategy(discountPos, discountNeg, discountSum float32)
	// GetAverageStrategy returns the average strategy over all iterations.
	GetAverageStrategy() []float32
}

// PolicyStore maintains a collection of NodePolicy for each node that
// is visited in a traversal of the game tree.
type PolicyStore interface {
	// GetPolicy returns the NodePolicy for the given GameTreeNode.
	GetPolicy(GameTreeNode) NodePolicy
}
