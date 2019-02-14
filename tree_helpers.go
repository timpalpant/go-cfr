package cfr

// IsTerminal returns true if this node is an end-game node.
func IsTerminal(node GameTreeNode) bool {
	return node.NumChildren() == 0
}

func CountTerminalNodes(root GameTreeNode) int {
	if IsTerminal(root) {
		return 1
	}

	total := 0
	for i := 0; i < root.NumChildren(); i++ {
		child := root.GetChild(i)
		total += CountTerminalNodes(child)
	}

	return total
}

func CountNodes(root GameTreeNode) int {
	total := 1
	for i := 0; i < root.NumChildren(); i++ {
		child := root.GetChild(i)
		total += CountNodes(child)
	}

	return total
}

func CountInfoSets(root GameTreeNode) int {
	seen := make(map[InfoSet]struct{})
	walkInfoSets(root, seen)
	return len(seen)
}

func walkInfoSets(node GameTreeNode, seen map[InfoSet]struct{}) {
	if !node.IsChance() && !IsTerminal(node) {
		seen[node.InfoSet()] = struct{}{}
	}

	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		walkInfoSets(child, seen)
	}
}
