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

func TestExternalSampling(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "cfr-test-")
	defer os.RemoveAll(tmpDir)

	opts := &opt.Options{}
	db, err := leveldb.OpenFile(tmpDir, opts)
	if err != nil {
		t.Fatal(err)
	}

	policy := New(db)
	opt := cfr.NewExternalSampling(policy)
	runCFR(t, opt, policy, 100000)
}

type cfrImpl interface {
	Run(cfr.GameTreeNode) float32
}

func runCFR(t *testing.T, opt cfrImpl, policy cfr.StrategyProfile, nIter int) cfr.GameTreeNode {
	root := kuhn.NewGame()
	var expectedValue float32
	for i := 1; i <= nIter; i++ {
		expectedValue += opt.Run(root)
		if nIter/10 > 0 && i%(nIter/10) == 0 {
			t.Logf("[iter=%d] Expected game value: %.4f", i, expectedValue/float32(i))
		}

		policy.Update()
	}

	return root
}
