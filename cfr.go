package cfr

import (
	"math"
	"math/rand"

	"github.com/golang/glog"
)

const eps = 1e-3

// Params are the configuration options for CFR sampling
// and regret matching. An empty Params struct is valid and
// corresponds to "vanilla" CFR.
type Params struct {
	SampleChanceNodes     bool    // Chance Sampling
	SamplePlayerActions   bool    // Outcome Sampling
	SampleOpponentActions bool    // External Sampling
	UseRegretMatchingPlus bool    // CFR+
	LinearWeighting       bool    // Linear CFR
	DiscountAlpha         float32 // Discounted CFR
	DiscountBeta          float32 // Discounted CFR
	DiscountGamma         float32 // Discounted CFR
	// PolicyStore, if provided, can be used to implement a custom
	// model that receives instantaneous regrets and predicts the current strategy.
	// (for example: Deep CFR). If not provided, the default tabular policy
	// store (as in most CFR) will be used, which looks up policies based
	// on the current InfoSet's key.
	PolicyStore PolicyStore
}

type CFR struct {
	params      Params
	policyStore PolicyStore
	iter        int
	visited     []NodePolicy
	slicePool   *floatSlicePool
}

func New(params Params) *CFR {
	store := params.PolicyStore
	if store == nil {
		store = newPolicyStore()
	}

	return &CFR{
		params:      params,
		policyStore: store,
		iter:        1,
		slicePool:   &floatSlicePool{},
	}
}

func (c *CFR) GetPolicyStore() PolicyStore {
	return c.policyStore
}

func (c *CFR) Run(node GameTreeNode) float32 {
	expectedValue := c.runHelper(node, node.Player(), 1.0, 1.0, 1.0)
	c.nextStrategyProfile()
	return expectedValue
}

func (c *CFR) nextStrategyProfile() {
	discountPos, discountNeg, discountSum := getDiscountFactors(c.params, c.iter)
	glog.V(1).Infof("Updating %d policies", len(c.visited))
	for _, p := range c.visited {
		p.NextStrategy(discountPos, discountNeg, discountSum)
	}

	c.visited = c.visited[:0]
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

	policy := c.policyStore.GetPolicy(node)
	if c.params.SampleOpponentActions && c.traversingPlayer() != player {
		// Sample according to current strategy profile.
		i := sampleOne(policy, node.NumChildren())
		child := node.GetChild(i)
		p := policy.GetActionProbability(i)
		if player == 0 {
			return c.runHelper(child, player, p*reachP0, reachP1, reachChance)
		} else {
			return c.runHelper(child, player, reachP0, p*reachP1, reachChance)
		}
	}

	actionUtils := c.slicePool.alloc(node.NumChildren())
	var expectedUtil float32
	for i := 0; i < node.NumChildren(); i++ {
		child := node.GetChild(i)
		p := policy.GetActionProbability(i)
		var util float32
		if player == 0 {
			util = c.runHelper(child, player, p*reachP0, reachP1, reachChance)
		} else {
			util = c.runHelper(child, player, reachP0, p*reachP1, reachChance)
		}

		actionUtils[i] = util
		expectedUtil += p * util
	}

	reachP := reachProb(player, reachP0, reachP1, reachChance)
	counterFactualP := counterFactualProb(player, reachP0, reachP1, reachChance)
	instantaneousRegrets := c.slicePool.alloc(node.NumChildren())
	for i, util := range actionUtils {
		instantaneousRegrets[i] = counterFactualP * (util - expectedUtil)
	}

	policy.AddRegret(reachP, instantaneousRegrets)
	c.visited = append(c.visited, policy)
	c.slicePool.free(actionUtils)
	c.slicePool.free(instantaneousRegrets)
	return expectedUtil
}

func getSign(lastPlayer int, child GameTreeNode) float32 {
	if child.Type() == PlayerNode || child.Player() != lastPlayer {
		return -1.0
	}

	return 1.0
}

func sampleOne(policy NodePolicy, numActions int) int {
	x := rand.Float32()
	var cumProb float32
	for i := 0; i < numActions; i++ {
		cumProb += policy.GetActionProbability(i)
		if cumProb > x {
			return i
		}
	}

	if cumProb < 1.0-eps { // Leave room for floating point error.
		panic("probability distribution does not sum to 1!")
	}

	return numActions - 1
}

func (c *CFR) traversingPlayer() int {
	return c.iter % 2
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
