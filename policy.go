package cfr

import (
	"fmt"
	"sync"

	"github.com/golang/glog"

	"github.com/timpalpant/go-cfr/internal/f32"
)

// StrategyTable implements traditional CFR by storing accumulated
// regrets and strategy sums for each InfoSet, which is looked up by its Key().
type StrategyTable struct {
	params DiscountParams
	iter   int

	// Map of InfoSet Key -> strategy for that infoset.
	mu            sync.Mutex
	strategies    map[string]*strategy
	mayNeedUpdate []*strategy
}

func NewStrategyTable(params DiscountParams) *StrategyTable {
	return &StrategyTable{
		params:     params,
		iter:       1,
		strategies: make(map[string]*strategy),
	}
}

func (st *StrategyTable) Update() {
	st.mu.Lock()
	defer st.mu.Unlock()
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

func (st *StrategyTable) GetStrategy(node GameTreeNode) NodeStrategy {
	p := node.Player()
	is := node.InfoSet(p)
	key := is.Key()

	st.mu.Lock()
	defer st.mu.Unlock()
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
	mu          sync.RWMutex
	reachProb   float32
	regretSum   []float32
	current     []float32
	strategySum []float32
}

func newStrategy(nActions int) *strategy {
	return &strategy{
		reachProb:   0.0,
		regretSum:   make([]float32, nActions),
		current:     uniformDist(nActions),
		strategySum: make([]float32, nActions),
	}
}

func (s *strategy) GetActionProbability(i int) float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current[i]
}

func (s *strategy) nextStrategy(discountPositiveRegret, discountNegativeRegret, discountStrategySum float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if discountStrategySum != 1.0 {
		f32.ScalUnitary(discountStrategySum, s.strategySum)
	}

	f32.AxpyUnitary(s.reachProb, s.current, s.strategySum)

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

	s.calcStrategy()
	s.reachProb = 0.0
}

func (s *strategy) needsUpdate() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reachProb > 0
}

func (s *strategy) numActions() int {
	return len(s.current)
}

func (s *strategy) calcStrategy() {
	copy(s.current, s.regretSum)
	makePositive(s.current)
	total := f32.Sum(s.current)
	if total > 0 {
		f32.ScalUnitary(1.0/total, s.current)
		return
	}

	for i := range s.current {
		s.current[i] = 1.0 / float32(len(s.current))
	}
}

func (s *strategy) GetAverageStrategy() []float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := f32.Sum(s.strategySum)
	if total > 0 {
		avgStrat := make([]float32, len(s.strategySum))
		f32.ScalUnitaryTo(avgStrat, 1.0/total, s.strategySum)
		return avgStrat
	}

	return uniformDist(len(s.current))
}

func (s *strategy) AddRegret(reachProb, counterFactualProb float32, instantaneousRegrets []float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
