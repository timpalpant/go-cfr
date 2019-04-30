package cfr

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/timpalpant/go-cfr/internal/policy"
)

func init() {
	gob.Register(&PolicyTable{})
}

// PolicyTable implements traditional (tabular) CFR by storing accumulated
// regrets and strategy sums for each InfoSet, which is looked up by its Key().
type PolicyTable struct {
	params DiscountParams
	iter   int

	policies []policy.Policy
	// Map of InfoSet Key -> index of the policy for that infoset.
	policiesByKey map[uint64]int
	mayNeedUpdate []int
}

// NewPolicyTable creates a new PolicyTable with the given DiscountParams.
func NewPolicyTable(params DiscountParams) *PolicyTable {
	return &PolicyTable{
		params:        params,
		iter:          1,
		policiesByKey: make(map[uint64]int),
	}
}

// Update performs regret matching for all nodes within this strategy profile that have
// been touched since the lapt call to Update().
func (pt *PolicyTable) Update() {
	discountPos, discountNeg, discountSum := pt.params.GetDiscountFactors(pt.iter)
	for idx := range pt.mayNeedUpdate {
		pt.policies[idx].NextStrategy(discountPos, discountNeg, discountSum)
	}

	pt.mayNeedUpdate = pt.mayNeedUpdate[:0]
	pt.iter++
}

func (pt *PolicyTable) Iter() int {
	return pt.iter
}

func (pt *PolicyTable) Close() error {
	return nil
}

func (pt *PolicyTable) GetPolicy(node GameTreeNode) NodePolicy {
	p := node.Player()
	is := node.InfoSet(p)
	key := is.Key()

	idx, ok := pt.policiesByKey[key]
	if !ok {
		np := policy.New(node.NumChildren())
		idx = len(pt.policies)
		pt.policies = append(pt.policies, np)
	} else if pt.policies[idx].NumActions() != node.NumChildren() {
		panic(fmt.Errorf("strategy has n_actions=%v but node has n_children=%v: %v",
			pt.policies[idx].NumActions(), node.NumChildren(), node))
	}

	pt.mayNeedUpdate = append(pt.mayNeedUpdate, idx)
	return &pt.policies[idx]
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (pt *PolicyTable) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&pt.params); err != nil {
		return err
	}

	if err := dec.Decode(&pt.iter); err != nil {
		return err
	}

	var nStrategies int
	if err := dec.Decode(&nStrategies); err != nil {
		return err
	}

	pt.policiesByKey = make(map[uint64]int, nStrategies)
	for i := 0; i < nStrategies; i++ {
		var key uint64
		if err := dec.Decode(&key); err != nil {
			return err
		}

		var p policy.Policy
		if err := dec.Decode(&p); err != nil {
			return err
		}

		pt.policies = append(pt.policies, p)
		pt.policiesByKey[key] = i
	}

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (pt *PolicyTable) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(pt.params); err != nil {
		return nil, err
	}

	if err := enc.Encode(pt.iter); err != nil {
		return nil, err
	}

	if err := enc.Encode(len(pt.policiesByKey)); err != nil {
		return nil, err
	}

	for key, p := range pt.policiesByKey {
		if err := enc.Encode(key); err != nil {
			return nil, err
		}

		if err := enc.Encode(p); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}
