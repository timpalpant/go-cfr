package tree

import (
	"github.com/timpalpant/go-cfr"
)

func Visit(root cfr.GameTreeNode, visitor func(node cfr.GameTreeNode)) {
	visitor(root)

	for i := 0; i < root.NumChildren(); i++ {
		Visit(root.GetChild(i), visitor)
	}

	root.Close()
}

func VisitInfoSets(root cfr.GameTreeNode, visitor func(player int, infoSet cfr.InfoSet)) {
	seen := make(map[string]struct{})
	Visit(root, func(node cfr.GameTreeNode) {
		if node.Type() == cfr.PlayerNodeType {
			player := node.Player()
			infoSet := node.InfoSet(player)
			key := infoSet.Key()
			if _, ok := seen[key]; ok {
				return
			}

			visitor(player, infoSet)
			seen[key] = struct{}{}
		}
	})
}

func CountTerminalNodes(root cfr.GameTreeNode) int {
	total := 0
	Visit(root, func(node cfr.GameTreeNode) {
		if node.Type() == cfr.TerminalNodeType {
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
	VisitInfoSets(root, func(player int, infoSet cfr.InfoSet) { total++ })
	return total
}
