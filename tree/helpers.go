package tree

import (
	"github.com/timpalpant/go-cfr"
)

func Visit(root cfr.GameTreeNode, visitor func(node cfr.GameTreeNode)) {
	visitor(root)
	for i := 0; i < root.NumChildren(); i++ {
		child := root.GetChild(i)
		Visit(child, visitor)
	}
}

func VisitInfoSets(root cfr.GameTreeNode, visitor func(player int, infoSet string)) {
	seen := make(map[string]struct{})
	Visit(root, func(node cfr.GameTreeNode) {
		if !node.IsChance() && !cfr.IsTerminal(node) {
			player := node.Player()
			infoSet := node.InfoSet(player)
			if _, ok := seen[infoSet]; ok {
				return
			}

			visitor(player, infoSet)
			seen[infoSet] = struct{}{}
		}
	})
}

func CountTerminalNodes(root cfr.GameTreeNode) int {
	total := 0
	Visit(root, func(node cfr.GameTreeNode) {
		if cfr.IsTerminal(node) {
			total++
		}
	})

	return total
}

func CountNodes(root cfr.GameTreeNode) int {
	total := 0
	Visit(root, func(node cfr.GameTreeNode) { total++ })
	return total
}

func CountInfoSets(root cfr.GameTreeNode) int {
	total := 0
	VisitInfoSets(root, func(player int, infoSet string) { total++ })
	return total
}
