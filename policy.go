package cfr

import (
	"fmt"
	"io"
	"sync"

	"github.com/golang/glog"

	"github.com/timpalpant/go-cfr/internal/f32"
)

type updateableNodeStrategy interface {
	NodeStrategy
	getStrategySum(int) float32
	numActions() int
	needsUpdate() bool
	nextStrategy(discountPositiveRegret, discountNegativeRegret, discountStrategySum float32)
}

// StrategyTable implements traditional (tabular) CFR by storing accumulated
// regrets and strategy sums for each InfoSet, which is looked up by its Key().
type StrategyTable struct {
	params DiscountParams
	iter   int

	// Map of InfoSet Key -> strategy for that infoset.
	strategies    map[string]updateableNodeStrategy
	mayNeedUpdate []updateableNodeStrategy
}

// NewStrategyTable creates a new StrategyTable with the given DiscountParams.
func NewStrategyTable(params DiscountParams) *StrategyTable {
	return &StrategyTable{
		params:     params,
		iter:       1,
		strategies: make(map[string]updateableNodeStrategy),
	}
}

// Update performs regret matching for all nodes within this strategy profile that have
// been touched since the last call to Update().
func (st *StrategyTable) Update() {
	glog.V(1).Infof("Updating %d policies", len(st.mayNeedUpdate))
	discountPos, discountNeg, discountSum := st.params.GetDiscountFactors(st.iter)
	for _, p := range st.mayNeedUpdate {
		if p.needsUpdate() {
			p.nextStrategy(discountPos, discountNeg, discountSum)
		}
	}

	st.mayNeedUpdate = st.mayNeedUpdate[:0]
	st.iter++
}

func (st *StrategyTable) Iter() int {
	return st.iter
}

// GetStrategy returns the NodeStrategy corresponding to the given game node.
// The strategy is looked up for the current player at that node based on its InfoSet.Key().
func (st *StrategyTable) GetStrategy(node GameTreeNode) NodeStrategy {
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

	st.mayNeedUpdate = append(st.mayNeedUpdate, s)
	return s
}

type strategy struct {
	reachProb   float32
	regretSum   []float32
	strategySum []float32
}

func newStrategy(nActions int) *strategy {
	return &strategy{
		reachProb:   0.0,
		regretSum:   make([]float32, nActions),
		strategySum: make([]float32, nActions),
	}
}

func (s *strategy) GetPolicy(p []float32) []float32 {
	if len(p) < s.numActions() {
		needed := s.numActions() - len(p)
		p = append(p, make([]float32, needed)...)
	} else if len(p) > s.numActions() {
		p = p[:s.numActions()]
	}

	copy(p, s.regretSum)
	makePositive(p)
	total := f32.Sum(p)
	if total > 0 {
		f32.ScalUnitary(1.0/total, p)
		return p
	}

	for i := range p {
		p[i] = 1.0 / float32(len(p))
	}

	return p
}

func (s *strategy) getStrategySum(i int) float32 {
	return s.strategySum[i]
}

func (s *strategy) nextStrategy(discountPositiveRegret, discountNegativeRegret, discountStrategySum float32) {
	if discountStrategySum != 1.0 {
		f32.ScalUnitary(discountStrategySum, s.strategySum)
	}

	// TODO: Use a pool here and return the slice.
	current := s.GetPolicy(nil)
	f32.AxpyUnitary(s.reachProb, current, s.strategySum)

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

	s.reachProb = 0.0
}

func (s *strategy) needsUpdate() bool {
	return s.reachProb > 0
}

func (s *strategy) numActions() int {
	return len(s.regretSum)
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

func (s *strategy) AddRegret(reachProb, counterFactualProb float32, instantaneousRegrets []float32) {
	s.reachProb += reachProb
	f32.AxpyUnitary(counterFactualProb, instantaneousRegrets, s.regretSum)
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

func (st *ThreadSafeStrategyTable) GetStrategy(node GameTreeNode) NodeStrategy {
	// We want to do this outside the lock because it may be relatively slow.
	p := node.Player()
	is := node.InfoSet(p)
	key := is.Key()

	st.mu.Lock()
	defer st.mu.Unlock()
	s, ok := st.st.strategies[key]
	if !ok {
		s = newThreadSafeStrategy(node.NumChildren())
		st.st.strategies[key] = s
	}

	if s.numActions() != node.NumChildren() {
		panic(fmt.Errorf("strategy has n_actions=%v but node has n_children=%v: %v",
			s.numActions(), node.NumChildren(), node))
	}

	st.st.mayNeedUpdate = append(st.st.mayNeedUpdate, s)
	return s
}

func (st *ThreadSafeStrategyTable) MarshalTo(w io.Writer) error {
	return st.st.MarshalTo(w)
}

type threadSafeStrategy struct {
	mu sync.RWMutex
	s  *strategy
}

func newThreadSafeStrategy(nActions int) *threadSafeStrategy {
	return &threadSafeStrategy{s: newStrategy(nActions)}
}

func (s *threadSafeStrategy) GetPolicy(p []float32) []float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s.GetPolicy(p)
}

func (s *threadSafeStrategy) getStrategySum(i int) float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s.getStrategySum(i)
}

func (s *threadSafeStrategy) AddRegret(reachProb, counterFactualProb float32, instantaneousRegrets []float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.s.AddRegret(reachProb, counterFactualProb, instantaneousRegrets)
}

func (s *threadSafeStrategy) GetAverageStrategy() []float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s.GetAverageStrategy()
}

func (s *threadSafeStrategy) nextStrategy(discountPositiveRegret, discountNegativeRegret, discountStrategySum float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.s.nextStrategy(discountPositiveRegret, discountNegativeRegret, discountStrategySum)
}

func (s *threadSafeStrategy) needsUpdate() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s.needsUpdate()
}

func (s *threadSafeStrategy) numActions() int {
	return len(s.s.regretSum)
}
