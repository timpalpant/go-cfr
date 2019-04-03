package cfr

import (
	"math/rand"

	"github.com/timpalpant/go-cfr/internal/f32"
	"github.com/timpalpant/go-cfr/internal/policy"
)

type ASSamplingParams struct {
	Epsilon float32
	Tau     float32
	Beta    float32
}

type AverageStrategySamplingCFR struct {
	params          ASSamplingParams
	strategyProfile *PolicyTable
	slicePool       *floatSlicePool
	mapPool         *stringIntMapPool
	rng             *rand.Rand

	traversingPlayer int
	sampledActions   map[string]int
}

func NewAverageStrategySampling(strategyProfile *PolicyTable, params ASSamplingParams) *AverageStrategySamplingCFR {
	return &AverageStrategySamplingCFR{
		params:          params,
		strategyProfile: strategyProfile,
		slicePool:       &floatSlicePool{},
		mapPool:         &stringIntMapPool{},
		rng:             rand.New(rand.NewSource(rand.Int63())),
	}
}

func (c *AverageStrategySamplingCFR) Run(node GameTreeNode) float32 {
	iter := c.strategyProfile.Iter()
	c.traversingPlayer = int(iter % 2)
	c.sampledActions = c.mapPool.alloc()
	defer c.mapPool.free(c.sampledActions)
	return c.runHelper(node, node.Player(), 1.0)
}

func (c *AverageStrategySamplingCFR) runHelper(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	var ev float32
	switch node.Type() {
	case TerminalNode:
		ev = float32(node.Utility(lastPlayer)) / sampleProb
	case ChanceNode:
		ev = c.handleChanceNode(node, lastPlayer, sampleProb)
	default:
		sgn := getSign(lastPlayer, node.Player())
		ev = sgn * c.handlePlayerNode(node, sampleProb)
	}

	node.Close()
	return ev
}

func (c *AverageStrategySamplingCFR) handleChanceNode(node GameTreeNode, lastPlayer int, sampleProb float32) float32 {
	child, _ := node.SampleChild()
	// Sampling probabilities cancel out in the calculation of counterfactual value.
	return c.runHelper(child, lastPlayer, sampleProb)
}

func (c *AverageStrategySamplingCFR) handlePlayerNode(node GameTreeNode, sampleProb float32) float32 {
	if node.Player() == c.traversingPlayer {
		return c.handleTraversingPlayerNode(node, sampleProb)
	} else {
		return c.handleSampledPlayerNode(node, sampleProb)
	}
}

func (c *AverageStrategySamplingCFR) handleTraversingPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	player := node.Player()
	nChildren := node.NumChildren()
	regrets := c.slicePool.alloc(nChildren)
	oldSampledActions := c.sampledActions
	c.sampledActions = c.mapPool.alloc()

	x := c.rng.Float32()
	policy := c.strategyProfile.GetPolicy(node).(*policy.Policy)
	strategy := policy.GetStrategy()
	s := policy.GetStrategySum()
	sSum := f32.Sum(s)

	var cfValue float32
	for i := 0; i < nChildren; i++ {
		child := node.GetChild(i)
		p := strategy[i]
		rho := computeRho(s[i], sSum, c.params)
		if x < rho {
			util := c.runHelper(child, player, minF32(rho, 1.0)*sampleProb)
			regrets[i] = util
			cfValue += p * util
		}
	}

	f32.AddConst(-cfValue, regrets)
	policy.AddRegret(1.0, regrets)

	c.slicePool.free(regrets)
	c.mapPool.free(c.sampledActions)
	c.sampledActions = oldSampledActions
	return cfValue
}

func minF32(x, y float32) float32 {
	if x < y {
		return x
	}

	return y
}

// Sample player action according to strategy, do not update policy.
// Save selected action so that they are reused if this infoset is hit again.
func (c *AverageStrategySamplingCFR) handleSampledPlayerNode(node GameTreeNode, sampleProb float32) float32 {
	policy := c.strategyProfile.GetPolicy(node)

	// Update average strategy for this node.
	// We perform "stochastic" updates as described in the MC-CFR paper.
	if sampleProb > 0 {
		policy.AddStrategyWeight(1.0 / sampleProb)
	}

	// Sampling probabilities cancel out in the calculation of counterfactual value,
	// so we don't include them here.
	child := getOrSample(c.sampledActions, node, policy, c.rng)
	return c.runHelper(child, node.Player(), sampleProb)
}

func computeRho(s, sSum float32, params ASSamplingParams) float32 {
	rho := params.Beta + params.Tau*s
	rho /= params.Beta + sSum
	if rho < params.Epsilon {
		return params.Epsilon
	}

	return rho
}
