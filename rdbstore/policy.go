package rdbstore

import (
	"bytes"
	"encoding/gob"

	"github.com/golang/glog"
	rocksdb "github.com/tecbot/gorocksdb"

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
	params    Params
	discounts cfr.DiscountParams

	db   *rocksdb.DB
	iter int
}

// New creates a new PolicyTable backed by a LevelDB database at the given path.
func New(params Params, discounts cfr.DiscountParams) (*PolicyTable, error) {
	db, err := rocksdb.OpenDb(params.Options, params.Path)
	if err != nil {
		return nil, err
	}

	return &PolicyTable{
		params:    params,
		discounts: discounts,
		db:        db,
		iter:      1,
	}, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (pt *PolicyTable) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(pt.params.Path); err != nil {
		return nil, err
	}

	if err := enc.Encode(pt.discounts); err != nil {
		return nil, err
	}

	if err := enc.Encode(pt.iter); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (pt *PolicyTable) UnmarshalBinary(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	var path string
	if err := dec.Decode(&path); err != nil {
		return err
	}

	// TODO: Serialize and reload RocksDB options.
	pt.params = DefaultParams(path)

	if err := dec.Decode(&pt.discounts); err != nil {
		return err
	}

	if err := dec.Decode(&pt.iter); err != nil {
		return err
	}

	pt.params.Options.SetCreateIfMissing(false)
	db, err := rocksdb.OpenDb(pt.params.Options, pt.params.Path)
	if err != nil {
		return err
	}

	pt.db = db
	return nil
}

// Close implements io.Closer.
func (pt *PolicyTable) Close() error {
	pt.db.Close()
	return nil
}

// Iter implements cfr.StrategyProfile.
func (pt *PolicyTable) Iter() int {
	return pt.iter
}

// Update implements cfr.StrategyProfile.
func (pt *PolicyTable) Update() {
	discountPos, discountNeg, discountSum := pt.discounts.GetDiscountFactors(pt.iter)
	it := pt.db.NewIterator(pt.params.ReadOptions)
	defer it.Close()

	n := 0
	// TODO(palpant): Figure out a way to keep track of which policies need updating.
	for it.SeekToFirst(); it.Valid(); it.Next() {
		key := it.Key()
		value := it.Value()
		n++

		var policy policy.Policy
		if err := policy.UnmarshalBinary(value.Data()); err != nil {
			panic(err)
		}

		policy.NextStrategy(discountPos, discountNeg, discountSum)
		buf, err := policy.MarshalBinary()
		if err != nil {
			panic(err)
		}

		if err := pt.db.Put(pt.params.WriteOptions, key.Data(), buf); err != nil {
			panic(err)
		}

		key.Free()
		value.Free()
	}

	if err := it.Err(); err != nil {
		panic(err)
	}

	glog.V(1).Infof("Updated %d strategies", n)
	pt.iter++
}

// GetPolicy implements cfr.StrategyProfile.
func (pt *PolicyTable) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	key := node.InfoSet(node.Player()).Key()
	policy := policy.New(node.NumChildren())

	result, err := pt.db.Get(pt.params.ReadOptions, []byte(key))
	if err != nil {
		panic(err)
	}
	defer result.Free()

	if len(result.Data()) > 0 {
		if err := policy.UnmarshalBinary(result.Data()); err != nil {
			panic(err)
		}
	}

	return &ldbPolicy{
		Policy: policy,
		db:     pt.db,
		key:    []byte(key),
		wOpts:  pt.params.WriteOptions,
	}
}

// ldbPolicy implements cfr.NodePolicy, with all updates immediately persisted
// to the underlying LevelDB database.
type ldbPolicy struct {
	*policy.Policy
	db    *rocksdb.DB
	key   []byte
	wOpts *rocksdb.WriteOptions
}

// AddRegret implements cfr.NodePolicy.
func (l *ldbPolicy) AddRegret(w float32, samplingQ, instantaneousRegrets []float32) {
	l.Policy.AddRegret(w, samplingQ, instantaneousRegrets)
	l.save()
}

// AddStrategyWeight implements cfr.NodePolicy.
func (l *ldbPolicy) AddStrategyWeight(w float32) {
	l.Policy.AddStrategyWeight(w)
	l.save()
}

func (l *ldbPolicy) save() {
	buf, err := l.Policy.MarshalBinary()
	if err != nil {
		panic(err)
	}

	if err := l.db.Put(l.wOpts, l.key, buf); err != nil {
		panic(err)
	}
}
