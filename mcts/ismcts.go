package mcts

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/sampling"
)

type Policy interface {
	GetPolicy(node cfr.GameTreeNode) []float32
}

type Evaluator interface {
	Evaluate(node cfr.GameTreeNode) (policy []float32, value float32)
}

// Implements one-sided IS-MCTS. The opponent will always use the provided policy
// to select actions.
type OneSidedISMCTS struct {
	player    int
	opponent  Policy
	evaluator Evaluator
	c         float32

	mx   sync.Mutex
	tree map[string]*mctsNode
}

func NewOneSidedISMCTS(player int, opponent Policy, evaluator Evaluator, c float32) *OneSidedISMCTS {
	return &OneSidedISMCTS{
		player:    player,
		opponent:  opponent,
		evaluator: evaluator,
		c:         c,

		tree: make(map[string]*mctsNode),
	}
}

func (s *OneSidedISMCTS) Run(node cfr.GameTreeNode) float32 {
	return s.simulate(node, node.Player())
}

func (s *OneSidedISMCTS) GetPolicy(node cfr.GameTreeNode, temperature float32) []float32 {
	if node.Player() != s.player {
		panic(fmt.Errorf("Trying to get policy for player %d from one-sided policy for player %d",
			node.Player(), s.player))
	}

	s.mx.Lock()
	defer s.mx.Unlock()
	u := node.InfoSet(node.Player()).Key()
	treeNode, ok := s.tree[u]
	if ok {
		return treeNode.averageStrategy(temperature)
	}

	return uniformDistribution(node.NumChildren())
}

func (s *OneSidedISMCTS) simulate(node cfr.GameTreeNode, lastPlayer int) float32 {
	var ev float32
	switch node.Type() {
	case cfr.TerminalNodeType:
		ev = float32(node.Utility(lastPlayer))
	case cfr.ChanceNodeType:
		child, _ := node.SampleChild()
		ev = s.simulate(child, lastPlayer)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * s.handlePlayerNode(node)
	}

	node.Close()
	return ev
}

func (s *OneSidedISMCTS) handlePlayerNode(node cfr.GameTreeNode) float32 {
	i := node.Player()
	if i != s.player {
		return s.handleOpponentNode(node)
	}

	u := node.InfoSet(i).Key()
	s.mx.Lock()
	treeNode, ok := s.tree[u]
	if !ok { // Expand tree.
		p, v := s.evaluator.Evaluate(node)
		treeNode = newMCTSNode(p)
		s.tree[u] = treeNode
		s.mx.Unlock()
		return v
	}
	s.mx.Unlock()

	action := treeNode.selectActionPUCT(s.c)
	child := node.GetChild(action)
	reward := s.simulate(child, i)
	treeNode.update(action, reward)
	return reward
}

func (s *OneSidedISMCTS) handleOpponentNode(node cfr.GameTreeNode) float32 {
	p := s.opponent.GetPolicy(node)
	selected := sampling.SampleOne(p, rand.Float32())
	child := node.GetChild(selected)
	return s.simulate(child, node.Player())
}
