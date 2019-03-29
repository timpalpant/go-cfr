// Package ldbpolicy implements a tabular CFR policy table that keeps data
// on disk in a LevelDB database, rather than in memory.
//
// It is substantially slower than an in-memory PolicyTable but can scale
// to games that do not fit in memory.
package ldbpolicy

import (
	"github.com/golang/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/policy"
)

type PolicyTable struct {
	params cfr.DiscountParams
	iter   int

	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

func New(db *leveldb.DB, params cfr.DiscountParams) *PolicyTable {
	return &PolicyTable{
		params: params,
		iter:   1,
		db:     db,
	}
}

func (pt *PolicyTable) Close() error {
	return pt.db.Close()
}

func (pt *PolicyTable) Iter() int {
	return pt.iter
}

func (pt *PolicyTable) Update() {
	discountPos, discountNeg, discountSum := pt.params.GetDiscountFactors(pt.iter)
	iter := pt.db.NewIterator(nil, pt.rOpts)
	n := 0
	for iter.Next() {
		n++
		var policy policy.Policy
		if err := policy.GobDecode(iter.Value()); err != nil {
			panic(err)
		}

		policy.NextStrategy(discountPos, discountNeg, discountSum)
		buf, err := policy.GobEncode()
		if err != nil {
			panic(err)
		}

		if err := pt.db.Put(iter.Key(), buf, pt.wOpts); err != nil {
			panic(err)
		}
	}

	iter.Release()
	if err := iter.Error(); err != nil {
		panic(err)
	}

	glog.V(1).Infof("Updated %d strategies", n)
	pt.iter++
}

func (pt *PolicyTable) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	key := []byte(node.InfoSet(node.Player()).Key())
	buf, err := pt.db.Get(key, pt.rOpts)
	policy := policy.New(node.NumChildren())
	if err != nil {
		if err != leveldb.ErrNotFound {
			panic(err)
		}
	} else {
		if err := policy.GobDecode(buf); err != nil {
			panic(err)
		}
	}

	return &ldbPolicy{
		Policy: policy,
		db:     pt.db,
		key:    key,
		wOpts:  pt.wOpts,
	}
}

type ldbPolicy struct {
	*policy.Policy
	db    *leveldb.DB
	key   []byte
	wOpts *opt.WriteOptions
}

func (l *ldbPolicy) AddRegret(w float32, instantaneousRegrets []float32) {
	l.Policy.AddRegret(w, instantaneousRegrets)
	l.save()
}

func (l *ldbPolicy) AddStrategyWeight(w float32) {
	l.Policy.AddStrategyWeight(w)
	l.save()
}

func (l *ldbPolicy) save() {
	buf, err := l.Policy.GobEncode()
	if err != nil {
		panic(err)
	}

	if err := l.db.Put(l.key, buf, l.wOpts); err != nil {
		panic(err)
	}
}
