package mcts

import (
	"bytes"
	"encoding/gob"
	"math"
	"math/rand"
	"sync"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/sampling"
)

// TODO(palpant): Make configurable; this assumes game outcomes are [-1, 1].
const virtualLoss = 1

type mctsNode struct {
	mx           sync.Mutex
	prior        []float32
	visits       []int
	totalRewards []float32
}

func newMCTSNode(prior []float32) *mctsNode {
	return &mctsNode{
		prior:        prior,
		visits:       make([]int, len(prior)),
		totalRewards: make([]float32, len(prior)),
	}
}

func (s *mctsNode) update(action int, reward float32) {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.visits[action] += 1 - virtualLoss
	s.totalRewards[action] += virtualLoss + reward
}

// NOTE: This uses the selection formula from the AlphaGo Zero paper.
// The main difference seems to be the weighting of the visit counts.
func (s *mctsNode) selectActionPUCT(rng *rand.Rand, c float32) (action int) {
	s.mx.Lock()
	defer func() {
		s.addVirtualLossUnsafe(action)
		s.mx.Unlock()
	}()
	totalVisits := 0
	for _, v := range s.visits {
		totalVisits += v
	}

	vMax := -float32(math.MaxFloat32)
	for i, n := range s.visits {
		p := s.prior[i]
		var q float32
		if n > 0 {
			q = s.totalRewards[i] / float32(n)
		}
		v := q + c*p*float32(math.Sqrt(float64(totalVisits)))/float32(1+n)
		if v > vMax {
			action = i
			vMax = v
		} else if v == vMax && rng.Intn(2) == 0 {
			// Break ties uniformly at random.
			action = i
		}
	}

	return
}

// Follows the notation of Heinrich and Silver (2015).
func (s *mctsNode) selectActionSmooth(rng *rand.Rand, c, gamma, eta, d float32) (action int) {
	s.mx.Lock()
	defer func() {
		s.addVirtualLossUnsafe(action)
		s.mx.Unlock()
	}()
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

	z := rng.Float32()
	if z < etaK {
		vMax := -float32(math.MaxFloat32)
		for i, n := range s.visits {
			q := s.totalRewards[i] / float32(n)
			v := q + c*float32(math.Sqrt(math.Log(float64(totalVisits))/float64(n)))
			if v > vMax {
				action = i
				vMax = v
			} else if v == vMax && rng.Intn(2) == 0 {
				// Break ties uniformly at random.
				action = i
			}
		}

		return
	}

	p := stackalloc(len(s.visits))
	s.fillAverageStrategyUnsafe(p, 1.0)
	return sampling.SampleOne(p, rng.Float32())
}

func (s *mctsNode) addVirtualLossUnsafe(action int) {
	s.visits[action] += virtualLoss
	s.totalRewards[action] -= virtualLoss
}

func (s *mctsNode) totalVisits() int {
	if s == nil {
		return 0
	}

	s.mx.Lock()
	defer s.mx.Unlock()
	var totalVisits int
	for _, v := range s.visits {
		totalVisits += v
	}
	return totalVisits
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

const maxOnStack = 16

func stackalloc(n int) []float32 {
	v := make([]float32, 0, maxOnStack)
	if n < maxOnStack {
		return v[:n]
	}

	return make([]float32, n)
}

// Implements Smooth UCT.
type SmoothUCT struct {
	c           float32
	gamma       float32
	eta         float32
	d           float32
	temperature float32

	mx   sync.Mutex
	tree map[string]*mctsNode
}

func NewSmoothUCT(c, gamma, eta, d, temperature float32) *SmoothUCT {
	return &SmoothUCT{
		c:           c,
		gamma:       gamma,
		eta:         eta,
		d:           d,
		temperature: temperature,

		tree: make(map[string]*mctsNode),
	}
}

func (s *SmoothUCT) Run(rng *rand.Rand, node cfr.GameTreeNode) float32 {
	return s.simulate(rng, node, node.Player(), [2]bool{false, false})
}

func (s *SmoothUCT) GetVisitCount(node cfr.GameTreeNode) int {
	u := node.InfoSetKey(node.Player())
	s.mx.Lock()
	treeNode := s.tree[string(u)]
	s.mx.Unlock()

	return treeNode.totalVisits()
}

func (s *SmoothUCT) GetPolicy(node cfr.GameTreeNode) []float32 {
	u := node.InfoSetKey(node.Player())
	s.mx.Lock()
	treeNode, ok := s.tree[string(u)]
	s.mx.Unlock()

	if ok {
		return treeNode.averageStrategy(s.temperature)
	}

	return uniformDistribution(node.NumChildren())
}

func (s *SmoothUCT) GobDecode(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&s.c); err != nil {
		return err
	}
	if err := dec.Decode(&s.gamma); err != nil {
		return err
	}
	if err := dec.Decode(&s.eta); err != nil {
		return err
	}
	if err := dec.Decode(&s.d); err != nil {
		return err
	}

	var numNodes int
	if err := dec.Decode(&numNodes); err != nil {
		return err
	}
	s.tree = make(map[string]*mctsNode)
	for i := 0; i < numNodes; i++ {
		var key string
		if err := dec.Decode(&key); err != nil {
			return err
		}
		var node mctsNode
		if err := dec.Decode(&node.prior); err != nil {
			return err
		}
		if err := dec.Decode(&node.visits); err != nil {
			return err
		}
		if err := dec.Decode(&node.totalRewards); err != nil {
			return err
		}
		s.tree[key] = &node
	}

	return nil
}

func (s *SmoothUCT) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(s.c); err != nil {
		return nil, err
	}
	if err := enc.Encode(s.gamma); err != nil {
		return nil, err
	}
	if err := enc.Encode(s.eta); err != nil {
		return nil, err
	}
	if err := enc.Encode(s.d); err != nil {
		return nil, err
	}

	s.mx.Lock()
	defer s.mx.Unlock()
	if err := enc.Encode(len(s.tree)); err != nil {
		return nil, err
	}
	for key, node := range s.tree {
		if err := enc.Encode(key); err != nil {
			return nil, err
		}
		if err := enc.Encode(node.prior); err != nil {
			return nil, err
		}
		if err := enc.Encode(node.visits); err != nil {
			return nil, err
		}
		if err := enc.Encode(node.totalRewards); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func uniformDistribution(n int) []float32 {
	result := make([]float32, n)
	for i := range result {
		result[i] = 1.0 / float32(n)
	}
	return result
}

func (s *SmoothUCT) simulate(rng *rand.Rand, node cfr.GameTreeNode, lastPlayer int, isOutOfTree [2]bool) float32 {
	var ev float32
	switch node.Type() {
	case cfr.TerminalNodeType:
		ev = float32(node.Utility(lastPlayer))
	case cfr.ChanceNodeType:
		child, _ := node.SampleChild()
		ev = s.simulate(rng, child, lastPlayer, isOutOfTree)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * s.handlePlayerNode(rng, node, isOutOfTree)
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

func (s *SmoothUCT) handlePlayerNode(rng *rand.Rand, node cfr.GameTreeNode, isOutOfTree [2]bool) float32 {
	i := node.Player()
	if isOutOfTree[i] {
		return s.rollout(rng, node, isOutOfTree)
	}

	u := node.InfoSetKey(i)
	s.mx.Lock()
	treeNode, ok := s.tree[string(u)]
	if !ok { // Expand tree.
		prior := uniformDistribution(node.NumChildren())
		treeNode = newMCTSNode(prior)
		s.tree[string(u)] = treeNode
		isOutOfTree[i] = true
	}
	s.mx.Unlock()

	action := treeNode.selectActionSmooth(rng, s.c, s.gamma, s.eta, s.d)
	child := node.GetChild(action)
	reward := s.simulate(rng, child, i, isOutOfTree)
	treeNode.update(action, reward)
	return reward
}

func (s *SmoothUCT) rollout(rng *rand.Rand, node cfr.GameTreeNode, isOutOfTree [2]bool) float32 {
	action := rng.Intn(node.NumChildren())
	child := node.GetChild(action)
	return s.simulate(rng, child, node.Player(), isOutOfTree)
}
