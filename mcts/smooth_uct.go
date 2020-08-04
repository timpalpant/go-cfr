package mcts

import (
	"math"
	"math/rand"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/sampling"
)

type mctsNode struct {
	visits       []int
	totalRewards []float32
}

func newMCTSNode(n int) mctsNode {
	return mctsNode{
		visits:       make([]int, n),
		totalRewards: make([]float32, n),
	}
}

func (s mctsNode) totalVisits() int {
	n := 0
	for _, v := range s.visits {
		n += v
	}
	return n
}

func (s mctsNode) value() float32 {
	var result float32
	for i, n := range s.visits {
		result += s.totalRewards[i] / float32(n)
	}

	return result
}

func (s mctsNode) rewards() []float32 {
	result := make([]float32, len(s.totalRewards))
	for i, n := range s.visits {
		result[i] = s.totalRewards[i] / float32(n)
	}

	return result
}

func (s mctsNode) update(action int, reward float32) {
	s.visits[action] = s.visits[action] + 1
	s.totalRewards[action] += reward
}

// Follows the notation of Heinrich and Silver (2015).
func (s mctsNode) selectAction(c, gamma, eta, d float32) int {
	etaK := eta / (1.0 + d*float32(math.Sqrt(float64(s.totalVisits()))))
	if etaK < gamma {
		etaK = gamma
	}

	z := rand.Float32()
	if z < etaK {
		var selected int
		vMax := -float32(math.MaxFloat32)
		qs := make([]float32, len(s.visits))
		vs := make([]float32, len(s.visits))
		for i, n := range s.visits {
			if n == 0 {
				return i // Pull unpulled arm.
			}

			q := s.totalRewards[i] / float32(n)
			v := q + c*float32(math.Sqrt(math.Log(float64(s.totalVisits()))/float64(n)))
			if v > vMax {
				selected = i
				vMax = v
			} else if v == vMax && rand.Intn(2) == 0 {
				// Break ties uniformly at random.
				selected = i
			}

			qs[i] = q
			vs[i] = v
		}

		return selected
	}

	p := stackalloc(len(s.visits))
	s.fillAverageStrategy(p)
	selected := sampling.SampleOne(p, rand.Float32())
	return selected
}

func (s mctsNode) averageStrategy() []float32 {
	p := make([]float32, len(s.visits))
	s.fillAverageStrategy(p)
	return p
}

func (s mctsNode) fillAverageStrategy(p []float32) {
	nTotal := float32(s.totalVisits())
	if nTotal == 0 {
		for i := range p {
			p[i] = 1.0 / float32(len(p))
		}
	} else {
		for i, n := range s.visits {
			p[i] = float32(n) / nTotal
		}
	}
}

const maxOnStack = 128

func stackalloc(n int) []float32 {
	if n < maxOnStack {
		v := make([]float32, maxOnStack)
		return v[:n]
	}

	return make([]float32, n)
}

// Implements Smooth UCT.
type SmoothUCT struct {
	iterations int
	c          float32
	gamma      float32
	eta        float32
	d          float32

	tree        map[string]mctsNode
	isOutOfTree [2]bool
}

func NewSmoothUCT(iterations int, c, gamma, eta, d float32) *SmoothUCT {
	return &SmoothUCT{
		iterations: iterations,
		c:          c,
		gamma:      gamma,
		eta:        eta,
		d:          d,

		tree: make(map[string]mctsNode),
	}
}

func (s *SmoothUCT) Run(node cfr.GameTreeNode) float32 {
	rootPlayer := node.Player()

	var ev float32
	for i := 0; i < s.iterations; i++ {
		ev += s.simulate(node, rootPlayer) / float32(s.iterations)
	}

	return ev
}

func (s *SmoothUCT) GetPolicy(node cfr.GameTreeNode) []float32 {
	u := node.InfoSet(node.Player()).Key()
	treeNode, ok := s.tree[u]
	if ok {
		return treeNode.averageStrategy()
	}

	return uniform(node.NumChildren())
}

func uniform(n int) []float32 {
	result := make([]float32, n)
	for i := range result {
		result[i] = 1.0 / float32(n)
	}
	return result
}

func (s *SmoothUCT) simulate(node cfr.GameTreeNode, lastPlayer int) float32 {
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

func getSign(player1, player2 int) float32 {
	if player1 == player2 {
		return 1.0
	}

	return -1.0
}

func (s *SmoothUCT) handlePlayerNode(node cfr.GameTreeNode) float32 {
	i := node.Player()
	if s.isOutOfTree[i] {
		return s.rollout(node)
	}

	u := node.InfoSet(i).Key()
	treeNode, ok := s.tree[u]
	var action int
	if !ok { // Expand tree.
		numChildren := node.NumChildren()
		treeNode = newMCTSNode(numChildren)
		s.tree[u] = treeNode
		action = rand.Intn(numChildren)
		s.isOutOfTree[i] = true
		defer func() { s.isOutOfTree[i] = false }()
	} else {
		action = treeNode.selectAction(s.c, s.gamma, s.eta, s.d)
	}

	child := node.GetChild(action)
	reward := s.simulate(child, i)
	treeNode.update(action, reward)
	return reward
}

func (s *SmoothUCT) rollout(node cfr.GameTreeNode) float32 {
	action := rand.Intn(node.NumChildren())
	child := node.GetChild(action)
	result := s.simulate(child, node.Player())
	return result
}
