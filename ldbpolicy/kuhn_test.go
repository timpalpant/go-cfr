package ldbpolicy

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/kuhn"
)

func TestVanilla(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "cfr-test-")
	defer os.RemoveAll(tmpDir)

	opts := &opt.Options{}
	db, err := leveldb.OpenFile(tmpDir, opts)
	if err != nil {
		t.Fatal(err)
	}

	policy := New(db, cfr.DiscountParams{})
	opt := cfr.New(policy)
	runCFR(t, opt, policy, 10000)
}

func BenchmarkVanilla(b *testing.B) {
	tmpDir, err := ioutil.TempDir("", "cfr-test-")
	defer os.RemoveAll(tmpDir)

	opts := &opt.Options{}
	db, err := leveldb.OpenFile(tmpDir, opts)
	if err != nil {
		b.Fatal(err)
	}

	policy := New(db, cfr.DiscountParams{})
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
