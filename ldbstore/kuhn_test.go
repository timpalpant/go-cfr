package ldbstore

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/kuhn"
	"github.com/timpalpant/go-cfr/tree"
)

func TestVanilla(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "cfr-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := &opt.Options{}
	policy, err := New(tmpDir, opts, cfr.DiscountParams{})
	if err != nil {
		t.Fatal(err)
	}

	opt := cfr.New(policy)
	testCFR(t, opt, policy, 1000)
}

func BenchmarkVanilla(b *testing.B) {
	tmpDir, err := ioutil.TempDir("", "cfr-test-")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	opts := &opt.Options{}
	policy, err := New(tmpDir, opts, cfr.DiscountParams{})
	opt := cfr.New(policy)

	b.ResetTimer()
	runCFR(b, opt, policy, b.N)
}

type logger interface {
	Logf(string, ...interface{})
}

type cfrImpl interface {
	Run(cfr.GameTreeNode) float32
}

func testCFR(t *testing.T, opt cfrImpl, policy cfr.StrategyProfile, nIter int) {
	root := runCFR(t, opt, policy, nIter)
	seen := make(map[string]struct{})
	tree.Visit(root, func(node cfr.GameTreeNode) {
		if node.Type() != cfr.PlayerNode {
			return
		}

		key := node.InfoSet(node.Player()).Key()
		if _, ok := seen[key]; ok {
			return
		}

		actionProbs := policy.GetPolicy(node).GetAverageStrategy()
		if actionProbs != nil {
			t.Logf("%6s: check=%.2f bet=%.2f", node, actionProbs[0], actionProbs[1])
		}

		seen[key] = struct{}{}
	})
}

func runCFR(log logger, opt cfrImpl, policy cfr.StrategyProfile, nIter int) cfr.GameTreeNode {
	root := kuhn.NewGame()
	var expectedValue float32
	for i := 1; i <= nIter; i++ {
		expectedValue += opt.Run(root)
		if nIter/10 > 0 && i%(nIter/10) == 0 {
			log.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}

		policy.Update()
	}

	return root
}
