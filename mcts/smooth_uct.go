package mcts

import (
	"math"
	"math/rand"
	"sync"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/sampling"
)

type mctsNode struct {
	mx           sync.Mutex
	visits       []int
	totalRewards []float32
}

func newMCTSNode(n int) *mctsNode {
	return &mctsNode{
		visits:       make([]int, n),
		totalRewards: make([]float32, n),
	}
}

func (s *mctsNode) update(action int, reward float32) {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.visits[action] = s.visits[action] + 1
	s.totalRewards[action] += reward
}

// Follows the notation of Heinrich and Silver (2015).
func (s *mctsNode) selectAction(c, gamma, eta, d float32) int {
	s.mx.Lock()
	defer s.mx.Unlock()
	totalVisits := 0
	for i, v := range s.visits {
		if v == 0 {
			return i // Pull unpulled arm.
		}

		totalVisits += v
	}

	etaK := eta / (1.0 + d*float32(math.Sqrt(float64(totalVisits))))
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
			q := s.totalRewards[i] / float32(n)
			v := q + c*float32(math.Sqrt(math.Log(float64(totalVisits))/float64(n)))
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
	s.fillAverageStrategyUnsafe(p, 1.0)
	selected := sampling.SampleOne(p, rand.Float32())
	return selected
}

func (s *mctsNode) averageStrategy(temperature float32) []float32 {
	s.mx.Lock()
	defer s.mx.Unlock()
	p := make([]float32, len(s.visits))
	s.fillAverageStrategyUnsafe(p, temperature)
	return p
}

func (s *mctsNode) fillAverageStrategyUnsafe(p []float32, temperature float32) {
	var nTotal float64
	for _, n := range s.visits {
		nTotal += math.Pow(float64(n), 1.0/float64(temperature))
	}

	if nTotal == 0 {
		for i := range p {
			p[i] = 1.0 / float32(len(p))
		}
	} else {
		for i, n := range s.visits {
			x := math.Pow(float64(n), 1.0/float64(temperature))
			p[i] = float32(x / nTotal)
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
	c     float32
	gamma float32
	eta   float32
	d     float32

	mx   sync.Mutex
	tree map[string]*mctsNode
}

func NewSmoothUCT(c, gamma, eta, d float32) *SmoothUCT {
	return &SmoothUCT{
		c:     c,
		gamma: gamma,
		eta:   eta,
		d:     d,

		tree: make(map[string]*mctsNode),
	}
}

func (s *SmoothUCT) Run(node cfr.GameTreeNode) float32 {
	return s.simulate(node, node.Player(), [2]bool{false, false})
}

func (s *SmoothUCT) GetPolicy(node cfr.GameTreeNode, temperature float32) []float32 {
	s.mx.Lock()
	defer s.mx.Unlock()
	u := node.InfoSet(node.Player()).Key()
	treeNode, ok := s.tree[u]
	if ok {
		return treeNode.averageStrategy(temperature)
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

func (s *SmoothUCT) simulate(node cfr.GameTreeNode, lastPlayer int, isOutOfTree [2]bool) float32 {
	var ev float32
	switch node.Type() {
	case cfr.TerminalNodeType:
		ev = float32(node.Utility(lastPlayer))
	case cfr.ChanceNodeType:
		child, _ := node.SampleChild()
		ev = s.simulate(child, lastPlayer, isOutOfTree)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * s.handlePlayerNode(node, isOutOfTree)
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

func (s *SmoothUCT) handlePlayerNode(node cfr.GameTreeNode, isOutOfTree [2]bool) float32 {
	i := node.Player()
	if isOutOfTree[i] {
		return s.rollout(node, isOutOfTree)
	}

	u := node.InfoSet(i).Key()
	s.mx.Lock()
	treeNode, ok := s.tree[u]
	if !ok { // Expand tree.
		numChildren := node.NumChildren()
		treeNode = newMCTSNode(numChildren)
		s.tree[u] = treeNode
		isOutOfTree[i] = true
	}
	s.mx.Unlock()

	action := treeNode.selectAction(s.c, s.gamma, s.eta, s.d)
	child := node.GetChild(action)
	reward := s.simulate(child, i, isOutOfTree)
	treeNode.update(action, reward)
	return reward
}

func (s *SmoothUCT) rollout(node cfr.GameTreeNode, isOutOfTree [2]bool) float32 {
	action := rand.Intn(node.NumChildren())
	child := node.GetChild(action)
	return s.simulate(child, node.Player(), isOutOfTree)
}
