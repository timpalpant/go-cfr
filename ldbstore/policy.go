package ldbstore

import (
	"bytes"
	"encoding/gob"

	"github.com/golang/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/policy"
)

func init() {
	gob.Register(&PolicyTable{})
}

// PolicyTable is a tabular CFR policy table that keeps all node policies
// on disk in a LevelDB database. PolicyTable implements cfr.StrategyProfile.
//
// It is functionally equivalent to a cfr.PolicyTable. In practice, it is significantly
// slower but will use constant amount of memory since all policies are kept on disk.
type PolicyTable struct {
	path   string
	opts   *opt.Options
	params cfr.DiscountParams
	iter   int

	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

// New creates a new PolicyTable backed by a LevelDB database at the given path.
func New(path string, opts *opt.Options, params cfr.DiscountParams) (*PolicyTable, error) {
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		return nil, err
	}

	return &PolicyTable{
		path:   path,
		opts:   opts,
		params: params,
		iter:   1,
		db:     db,
	}, nil
}

// GobEncode implements gob.GobEncoder.
func (pt *PolicyTable) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(pt.path); err != nil {
		return nil, err
	}

	if err := enc.Encode(pt.opts); err != nil {
		return nil, err
	}

	if err := enc.Encode(pt.params); err != nil {
		return nil, err
	}

	if err := enc.Encode(pt.iter); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GobEncode implements gob.GobDecoder.
func (pt *PolicyTable) GobDecode(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&pt.path); err != nil {
		return err
	}

	if err := dec.Decode(&pt.opts); err != nil {
		return err
	}

	if err := dec.Decode(&pt.params); err != nil {
		return err
	}

	if err := dec.Decode(&pt.iter); err != nil {
		return err
	}

	pt.opts.ErrorIfMissing = true
	db, err := leveldb.OpenFile(pt.path, pt.opts)
	if err != nil {
		return err
	}

	pt.db = db
	return nil
}

// Close implements io.Closer.
func (pt *PolicyTable) Close() error {
	return pt.db.Close()
}

// Iter implements cfr.StrategyProfile.
func (pt *PolicyTable) Iter() int {
	return pt.iter
}

// Update implements cfr.StrategyProfile.
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

// GetPolicy implements cfr.StrategyProfile.
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

// ldbPolicy implements cfr.NodePolicy, with all updates immediately persisted
// to the underlying LevelDB database.
type ldbPolicy struct {
	*policy.Policy
	db    *leveldb.DB
	key   []byte
	wOpts *opt.WriteOptions
}

// AddRegret implements cfr.NodePolicy.
func (l *ldbPolicy) AddRegret(w float32, instantaneousRegrets []float32) {
	l.Policy.AddRegret(w, instantaneousRegrets)
	l.save()
}

// AddStrategyWeight implements cfr.NodePolicy.
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
