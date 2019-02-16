package tree

import (
	"github.com/timpalpant/go-cfr"
)

func CountTerminalNodes(root cfr.GameTreeNode) int {
	if cfr.IsTerminal(root) {
		return 1
	}

	total := 0
	for i := 0; i < root.NumChildren(); i++ {
		child := root.GetChild(i)
		total += CountTerminalNodes(child)
	}

	return total
}

func CountNodes(root cfr.GameTreeNode) int {
	total := 1
	for i := 0; i < root.NumChildren(); i++ {
		child := root.GetChild(i)
		total += CountNodes(child)
	}

	return total
}

func CountInfoSets(root cfr.GameTreeNode) int {
	seen := make(map[string]struct{})
	walkInfoSets(root, seen)
	return len(seen)
}

func walkInfoSets(node cfr.GameTreeNode, seen map[string]struct{}) {
	if !node.IsChance() && !cfr.IsTerminal(node) {
		seen[node.InfoSet(node.Player())] = struct{}{}
	}

	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		walkInfoSets(child, seen)
	}
}
