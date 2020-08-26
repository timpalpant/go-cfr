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
	Evaluate(node cfr.GameTreeNode, opponent Policy) (policy []float32, value float32)
}

type RandomRollout struct {
	nRollouts int
}

func NewRandomRollout(nRollouts int) *RandomRollout {
	return &RandomRollout{nRollouts}
}

func (rr *RandomRollout) Evaluate(node cfr.GameTreeNode, opponent Policy) (policy []float32, value float32) {
	player := node.Player()
	var ev float64
	for i := 0; i < rr.nRollouts; i++ {
		current := node
		for current.Type() != cfr.TerminalNodeType {
			if current.Type() == cfr.ChanceNodeType {
				current, _ = current.SampleChild()
			} else if current.Player() == player {
				action := rand.Intn(current.NumChildren())
				current = current.GetChild(action)
			} else {
				p := opponent.GetPolicy(current)
				if len(p) != current.NumChildren() {
					panic(fmt.Errorf("policy returned wrong number of actions: expected %d, got %d",
						current.NumChildren(), len(p)))
				}

				action := sampling.SampleOne(p, rand.Float32())
				current = current.GetChild(action)
			}
		}

		ev += current.Utility(node.Player()) / float64(rr.nRollouts)
	}

	policy = uniformDistribution(node.NumChildren())
	node.Close()
	return policy, float32(ev)
}

// Implements one-sided IS-MCTS. The opponent will always use the provided policy
// to select actions.
type OneSidedISMCTS struct {
	player      int
	opponent    Policy
	evaluator   Evaluator
	c           float32
	temperature float32

	mx   sync.Mutex
	tree map[string]*mctsNode
}

func NewOneSidedISMCTS(player int, evaluator Evaluator, c, temperature float32) *OneSidedISMCTS {
	return &OneSidedISMCTS{
		player:      player,
		evaluator:   evaluator,
		c:           c,
		temperature: temperature,

		tree: make(map[string]*mctsNode),
	}
}

func (s *OneSidedISMCTS) Run(node cfr.GameTreeNode, opponent Policy) float32 {
	return s.simulate(node, opponent, node.Player())
}

func (s *OneSidedISMCTS) GetPolicy(node cfr.GameTreeNode) []float32 {
	if node.Player() != s.player {
		panic(fmt.Errorf("Trying to get policy for player %d from one-sided policy for player %d",
			node.Player(), s.player))
	}

	u := node.InfoSet(node.Player()).Key()
	s.mx.Lock()
	treeNode, ok := s.tree[u]
	s.mx.Unlock()

	if ok {
		return treeNode.averageStrategy(s.temperature)
	}

	return uniformDistribution(node.NumChildren())
}

func (s *OneSidedISMCTS) simulate(node cfr.GameTreeNode, opponent Policy, lastPlayer int) float32 {
	var ev float32
	switch node.Type() {
	case cfr.TerminalNodeType:
		ev = float32(node.Utility(lastPlayer))
	case cfr.ChanceNodeType:
		child, _ := node.SampleChild()
		ev = s.simulate(child, opponent, lastPlayer)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * s.handlePlayerNode(node, opponent)
	}

	node.Close()
	return ev
}

func (s *OneSidedISMCTS) handlePlayerNode(node cfr.GameTreeNode, opponent Policy) float32 {
	i := node.Player()
	if i != s.player {
		return s.handleOpponentNode(node, opponent)
	}

	u := node.InfoSet(i).Key()
	s.mx.Lock()
	treeNode, ok := s.tree[u]
	if !ok { // Expand tree.
		// Unlock so that other evaluations can be batched together.
		// If we race here and try to expand the same node twice, it's ok
		// since the prior and values will be the same.
		s.mx.Unlock()
		p, v := s.evaluator.Evaluate(node, opponent)
		treeNode = newMCTSNode(p)
		s.mx.Lock()
		s.tree[u] = treeNode
		s.mx.Unlock()
		return v
	}
	s.mx.Unlock()

	action := treeNode.selectActionPUCT(s.c)
	child := node.GetChild(action)
	reward := s.simulate(child, opponent, i)
	treeNode.update(action, reward)
	return reward
}

func (s *OneSidedISMCTS) handleOpponentNode(node cfr.GameTreeNode, opponent Policy) float32 {
	p := opponent.GetPolicy(node)
	selected := sampling.SampleOne(p, rand.Float32())
	child := node.GetChild(selected)
	return s.simulate(child, opponent, node.Player())
}
