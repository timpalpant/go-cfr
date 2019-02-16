package tree

import (
	"github.com/timpalpant/go-cfr"
)

func VisitInfoSets(root cfr.GameTreeNode, visitor func(player int, infoSet string)) {
	seen := make(map[string]struct{})
	root.VisitChildren(func(node cfr.GameTreeNode, p float64) {
		if node.Type() == cfr.PlayerNode {
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
	root.VisitChildren(func(node cfr.GameTreeNode, p float64) {
		if node.Type() == cfr.TerminalNode {
			total++
		}
	})

	return total
}

func CountNodes(root cfr.GameTreeNode) int {
	total := 0
	root.VisitChildren(func(node cfr.GameTreeNode, p float64) { total++ })
	return total
}

func CountInfoSets(root cfr.GameTreeNode) int {
	total := 0
	VisitInfoSets(root, func(player int, infoSet string) { total++ })
	return total
}
