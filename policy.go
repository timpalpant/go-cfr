package cfr

import (
	"fmt"
	"io"
	"sync"

	"github.com/timpalpant/go-cfr/internal/f32"
)

// StrategyTable implements traditional (tabular) CFR by storing accumulated
// regrets and strategy sums for each InfoSet, which is looked up by its Key().
type StrategyTable struct {
	params DiscountParams
	iter   int

	// Map of InfoSet Key -> strategy for that infoset.
	strategies  map[string]*strategy
	needsUpdate map[*strategy]struct{}
}

// NewStrategyTable creates a new StrategyTable with the given DiscountParams.
func NewStrategyTable(params DiscountParams) *StrategyTable {
	return &StrategyTable{
		params:      params,
		iter:        1,
		strategies:  make(map[string]*strategy),
		needsUpdate: make(map[*strategy]struct{}),
	}
}

// Update performs regret matching for all nodes within this strategy profile that have
// been touched since the last call to Update().
func (st *StrategyTable) Update() {
	discountPos, discountNeg, discountSum := st.params.GetDiscountFactors(st.iter)
	for s := range st.needsUpdate {
		s.nextStrategy(discountPos, discountNeg, discountSum)
	}

	st.needsUpdate = make(map[*strategy]struct{})
	st.iter++
}

func (st *StrategyTable) Iter() int {
	return st.iter
}

func (st *StrategyTable) AddRegret(node GameTreeNode, instantaneousRegrets []float32) {
	s := st.getStrategy(node)
	s.AddRegret(instantaneousRegrets)
	st.needsUpdate[s] = struct{}{}
}

func (st *StrategyTable) GetPolicy(node GameTreeNode) []float32 {
	s := st.getStrategy(node)
	return s.GetPolicy()
}

func (st *StrategyTable) AddStrategyWeight(node GameTreeNode, w float32) {
	s := st.getStrategy(node)
	s.AddStrategyWeight(w)
	st.needsUpdate[s] = struct{}{}
}

func (st *StrategyTable) GetAverageStrategy(node GameTreeNode) []float32 {
	s := st.getStrategy(node)
	return s.GetAverageStrategy()
}

func (st *StrategyTable) GetStrategySum(node GameTreeNode) []float32 {
	s := st.getStrategy(node)
	return s.strategySum
}

func (st *StrategyTable) getStrategy(node GameTreeNode) *strategy {
	p := node.Player()
	is := node.InfoSet(p)
	key := is.Key()

	s, ok := st.strategies[key]
	if !ok {
		s = newStrategy(node.NumChildren())
		st.strategies[key] = s
	}

	if s.numActions() != node.NumChildren() {
		panic(fmt.Errorf("strategy has n_actions=%v but node has n_children=%v: %v",
			s.numActions(), node.NumChildren(), node))
	}

	return s
}

type strategy struct {
	currentStrategy       []float32
	currentStrategyWeight float32

	regretSum   []float32
	strategySum []float32
}

func newStrategy(nActions int) *strategy {
	return &strategy{
		currentStrategy:       uniformDist(nActions),
		currentStrategyWeight: 0.0,
		regretSum:             make([]float32, nActions),
		strategySum:           make([]float32, nActions),
	}
}

func (s *strategy) GetPolicy() []float32 {
	return s.currentStrategy
}

func (s *strategy) nextStrategy(discountPositiveRegret, discountNegativeRegret, discountStrategySum float32) {
	if discountStrategySum != 1.0 {
		f32.ScalUnitary(discountStrategySum, s.strategySum)
	}

	f32.AxpyUnitary(s.currentStrategyWeight, s.currentStrategy, s.strategySum)

	if discountPositiveRegret != 1.0 {
		for i, x := range s.regretSum {
			if x > 0 {
				s.regretSum[i] *= discountPositiveRegret
			}
		}
	}

	if discountNegativeRegret != 1.0 {
		for i, x := range s.regretSum {
			if x < 0 {
				s.regretSum[i] *= discountNegativeRegret
			}
		}
	}

	s.regretMatching()
	s.currentStrategyWeight = 0.0
}

func (s *strategy) regretMatching() {
	copy(s.currentStrategy, s.regretSum)
	makePositive(s.currentStrategy)
	total := f32.Sum(s.currentStrategy)
	if total > 0 {
		f32.ScalUnitary(1.0/total, s.currentStrategy)
	} else {
		for i := range s.currentStrategy {
			s.currentStrategy[i] = 1.0 / float32(len(s.currentStrategy))
		}
	}
}

func (s *strategy) numActions() int {
	return len(s.regretSum)
}

func (s *strategy) AddRegret(instantaneousRegrets []float32) {
	f32.Add(s.regretSum, instantaneousRegrets)
}

func (s *strategy) AddStrategyWeight(w float32) {
	s.currentStrategyWeight += w
}

func (s *strategy) GetAverageStrategy() []float32 {
	total := f32.Sum(s.strategySum)
	if total > 0 {
		avgStrat := make([]float32, len(s.strategySum))
		f32.ScalUnitaryTo(avgStrat, 1.0/total, s.strategySum)
		return avgStrat
	}

	return uniformDist(len(s.regretSum))
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}

// ThreadSafeStrategyTable wraps StrategyTable and is safe to use from multiple goroutines.
type ThreadSafeStrategyTable struct {
	mu sync.Mutex
	st *StrategyTable
}

func NewThreadSafeStrategyTable(params DiscountParams) *ThreadSafeStrategyTable {
	st := NewStrategyTable(params)
	return &ThreadSafeStrategyTable{st: st}
}

func (st *ThreadSafeStrategyTable) Update() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.st.Update()
}

func (st *ThreadSafeStrategyTable) Iter() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.st.Iter()
}

func (st *ThreadSafeStrategyTable) AddRegret(node GameTreeNode, instantaneousRegrets []float32) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.st.AddRegret(node, instantaneousRegrets)
}

func (st *ThreadSafeStrategyTable) GetPolicy(node GameTreeNode) []float32 {
	st.mu.Lock()
	defer st.mu.Unlock()
	// TODO: There is some work that can be done outside the lock, consider
	// duplicating this function here if it it's worth it.
	return st.st.GetPolicy(node)
}

func (st *ThreadSafeStrategyTable) AddStrategyWeight(node GameTreeNode, w float32) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.st.AddStrategyWeight(node, w)
}

func (st *ThreadSafeStrategyTable) GetAverageStrategy(node GameTreeNode) []float32 {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.st.GetAverageStrategy(node)
}

func (st *ThreadSafeStrategyTable) MarshalTo(w io.Writer) error {
	return st.st.MarshalTo(w)
}
