package cfr

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/golang/glog"
)

const eps = 1e-3

type Params struct {
	SampleChanceNodes     bool    // Chance Sampling
	SamplePlayerActions   bool    // Outcome Sampling
	SampleOpponentActions bool    // External Sampling
	UseRegretMatchingPlus bool    // CFR+
	LinearWeighting       bool    // Linear CFR
	DiscountAlpha         float32 // Discounted CFR
	DiscountBeta          float32 // Discounted CFR
	DiscountGamma         float32 // Discounted CFR
	// Strategy probabilities below this value will be set to zero.
	PurificationThreshold float32
}

type CFR struct {
	params Params

	iter int

	// Map of player -> InfoSet -> Strategy for that InfoSet.
	strategyProfile map[int]map[string]*policy
	needsUpdate     []*policy
	slicePool       *floatSlicePool
}

func New(params Params) *CFR {
	return &CFR{
		params: params,
		iter:   1,
		strategyProfile: map[int]map[string]*policy{
			0: make(map[string]*policy),
			1: make(map[string]*policy),
		},
		slicePool: &floatSlicePool{},
	}
}

func (c *CFR) GetStrategy(player int, infoSet string) []float32 {
	policy := c.strategyProfile[player][infoSet]
	if policy == nil {
		return nil
	}

	return policy.getAverageStrategy(c.params.PurificationThreshold)
}

func (c *CFR) Run(node GameTreeNode) float32 {
	expectedValue := c.runHelper(node, node.Player(), 1.0, 1.0, 1.0)
	c.nextStrategyProfile()
	return expectedValue
}

func (c *CFR) nextStrategyProfile() {
	discountPos, discountNeg, discountSum := getDiscountFactors(c.params, c.iter)
	glog.V(1).Infof("Updating %d policies", len(c.needsUpdate))
	for _, p := range c.needsUpdate {
		p.nextStrategy(discountPos, discountNeg, discountSum)
	}

	c.needsUpdate = c.needsUpdate[:0]
	c.iter++
}

func (c *CFR) runHelper(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachChance float32) float32 {
	node.BuildChildren()

	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = node.Utility(lastPlayer)
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, reachP0, reachP1, reachChance)
	default:
		sgn := getSign(lastPlayer, node)
		ev = sgn * c.handlePlayerNode(node, reachP0, reachP1, reachChance)
	}

	node.FreeChildren()
	return ev
}

func (c *CFR) handleChanceNode(node GameTreeNode, lastPlayer int, reachP0, reachP1, reachChance float32) float32 {
	if c.params.SampleChanceNodes {
		child := node.SampleChild()
		return c.runHelper(child, lastPlayer, reachP0, reachP1, reachChance)
	}

	var expectedValue float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := node.GetChildProbability(i)
		expectedValue += c.runHelper(child, lastPlayer, reachP0, reachP1, reachChance*p)
	}

	return expectedValue / float32(node.NumChildren())
}

func (c *CFR) handlePlayerNode(node GameTreeNode, reachP0, reachP1, reachChance float32) float32 {
	player := node.Player()

	if node.NumChildren() == 1 { // Fast path for trivial nodes with no real choice.
		child := node.GetChild(0)
		return c.runHelper(child, player, reachP0, reachP1, reachChance)
	}

	policy := c.getPolicy(node)
	if c.params.SampleOpponentActions && c.traversingPlayer() != player {
		// Sample according to current strategy profile.
		i := sampleDist(policy.strategy)
		child := node.GetChild(i)
		p := policy.strategy[i]
		if player == 0 {
			return c.runHelper(child, player, p*reachP0, reachP1, reachChance)
		} else {
			return c.runHelper(child, player, reachP0, p*reachP1, reachChance)
		}
	}

	actionUtils := c.slicePool.alloc(node.NumChildren())
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := policy.strategy[i]
		if player == 0 {
			actionUtils[i] = c.runHelper(child, player, p*reachP0, reachP1, reachChance)
		} else {
			actionUtils[i] = c.runHelper(child, player, reachP0, p*reachP1, reachChance)
		}
	}

	reachP := reachProb(player, reachP0, reachP1, reachChance)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	cfUtility := policy.update(actionUtils, reachP, counterFactualP)
	c.needsUpdate = append(c.needsUpdate, policy)
	c.slicePool.free(actionUtils)
	return cfUtility
}

func getSign(lastPlayer int, child GameTreeNode) float32 {
	if child.Type() == PlayerNode || child.Player() != lastPlayer {
		return -1.0
	}

	return 1.0
}

func sampleDist(probDist []float32) int {
	x := rand.Float32()
	var cumProb float32
	for i, p := range probDist {
		cumProb += p
		if cumProb > x {
			return i
		}
	}

	if cumProb < 1.0-eps { // Leave room for floating point error.
		panic("probability distribution does not sum to 1!")
	}

	return len(probDist) - 1
}

func (c *CFR) traversingPlayer() int {
	return c.iter % 2
}

func (c *CFR) getPolicy(node GameTreeNode) *policy {
	p := node.Player()
	is := node.InfoSet(p)

	if policy, ok := c.strategyProfile[p][is]; ok {
		if policy.numActions() != node.NumChildren() {
			panic(fmt.Errorf("policy has n_actions=%v but node has n_children=%v: %v",
				policy.numActions(), node.NumChildren(), node))
		}
		return policy
	}

	policy := newPolicy(node.NumChildren())
	c.strategyProfile[p][is] = policy
	if len(c.strategyProfile[p])%100000 == 0 {
		glog.V(2).Infof("Player %d - %d infosets", p, len(c.strategyProfile[p]))
	}
	return policy
}

func reachProb(player int, reachP0, reachP1, reachChance float32) float32 {
	if player == 0 {
		return reachP0 * reachChance
	} else {
		return reachP1 * reachChance
	}
}

// The probability of reaching this node, assuming that the current player
// tried to reach it.
func counterFactualProb(player int, reachP0, reachP1, reachChance float32) float32 {
	if player == 0 {
		return reachP1 * reachChance
	} else {
		return reachP0 * reachChance
	}
}

// Gets the discount factors as configured by the parameters for the
// various CFR weighting schemes: CFR+, linear CFR, etc.
func getDiscountFactors(params Params, iter int) (positive, negative, sum float32) {
	positive = float32(1.0)
	negative = float32(1.0)
	sum = float32(1.0)

	// See: https://arxiv.org/pdf/1809.04040.pdf
	// Linear CFR is equivalent to weighting the reach prob on each
	// iteration by (t / (t+1)), and this reduces numerical instability.
	if params.LinearWeighting {
		sum = float32(iter) / float32(iter+1)
	}

	if params.UseRegretMatchingPlus {
		negative = 0.0 // No negative regrets.
	}

	if params.DiscountAlpha != 0 {
		// t^alpha / (t^alpha + 1)
		x := float32(math.Pow(float64(iter), float64(params.DiscountAlpha)))
		positive = x / (x + 1.0)
	}

	if params.DiscountBeta != 0 {
		// t^beta / (t^beta + 1)
		x := float32(math.Pow(float64(iter), float64(params.DiscountBeta)))
		negative = x / (x + 1.0)
	}

	if params.DiscountGamma != 0 {
		// (t / (t+1)) ^ gamma
		x := float64(iter) / float64(iter+1)
		sum = float32(math.Pow(x, float64(params.DiscountGamma)))
	}

	return
}
