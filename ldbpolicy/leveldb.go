// Package ldbpolicy implements a tabular CFR policy table that keeps data
// on disk in a LevelDB database, rather than in memory.
//
// It is substantially slower than an in-memory PolicyTable but can scale
// to games that do not fit in memory.
package ldbpolicy

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/f32"
)

const (
	regretPrefix          = "rs:"
	currentStrategyPrefix = "cs:"
	strategySumPrefix     = "ss:"
	strategyWeightPrefix  = "sw:"
)

type PolicyTable struct {
	iter int

	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

func New(db *leveldb.DB) *PolicyTable {
	return &PolicyTable{
		iter: 1,
		db:   db,
	}
}

func (pt *PolicyTable) Close() error {
	return pt.db.Close()
}

func (pt *PolicyTable) Iter() int {
	return pt.iter
}

func (pt *PolicyTable) Update() {
	iter := pt.db.NewIterator(util.BytesPrefix([]byte(regretPrefix)), pt.rOpts)
	defer iter.Release()
	for iter.Next() {
		key := string(iter.Key())
		regretSum := decodeF32s(iter.Value())
		currentStrategy := regretMatching(regretSum)
		csKey := currentStrategyPrefix + key[len(regretPrefix):]
		buf := encodeF32s(currentStrategy)
		err := pt.db.Put([]byte(csKey), buf, pt.wOpts)
		if err != nil {
			panic(err)
		}
	}

	if err := iter.Error(); err != nil {
		panic(err)
	}

	pt.iter++
}

func (pt *PolicyTable) GetPolicy(node cfr.GameTreeNode) cfr.NodePolicy {
	return &ldbPolicy{
		key:      node.InfoSet(node.Player()).Key(),
		nActions: node.NumChildren(),
		db:       pt.db,
		rOpts:    pt.rOpts,
		wOpts:    pt.wOpts,
	}
}

func regretMatching(regretSum []float32) []float32 {
	makePositive(regretSum)
	total := f32.Sum(regretSum)
	if total > 0 {
		f32.ScalUnitary(1.0/total, regretSum)
	} else {
		for i := range regretSum {
			regretSum[i] = 1.0 / float32(len(regretSum))
		}
	}

	return regretSum
}

type ldbPolicy struct {
	key      string
	nActions int

	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

func (l *ldbPolicy) AddRegret(w float32, instantaneousRegrets []float32) {
	key := regretPrefix + l.key
	regretSum := l.getFloatSlice(key)
	f32.AxpyUnitary(w, regretSum, instantaneousRegrets)
	l.putFloatSlice(key, regretSum)
}

func (l *ldbPolicy) GetStrategy() []float32 {
	key := currentStrategyPrefix + l.key
	s := l.getFloatSlice(key)
	if len(s) == 0 {
		return uniformDist(l.nActions)
	}

	return s
}

func (l *ldbPolicy) AddStrategyWeight(w float32) {
	key := strategyWeightPrefix + l.key
	buf, err := l.db.Get([]byte(key), l.rOpts)
	if err != nil {
		if err != leveldb.ErrNotFound {
			panic(err)
		}

		buf = make([]byte, 4)
	} else {
		bits := binary.LittleEndian.Uint32(buf)
		w += math.Float32frombits(bits)
	}

	bits := math.Float32bits(w)
	binary.LittleEndian.PutUint32(buf, bits)
	if err := l.db.Put([]byte(key), buf, l.wOpts); err != nil {
		panic(err)
	}
}

func (l *ldbPolicy) GetAverageStrategy() []float32 {
	key := strategySumPrefix + l.key
	strategySum := l.getFloatSlice(key)
	if len(strategySum) > 0 {
		total := f32.Sum(strategySum)
		if total > 0 {
			f32.ScalUnitary(1.0/total, strategySum)
			return strategySum
		} else {
			for i := range strategySum {
				strategySum[i] = 1.0 / float32(len(strategySum))
			}
		}
	} else {
		strategySum = uniformDist(l.nActions)
	}

	return strategySum
}

func (l *ldbPolicy) getFloatSlice(key string) []float32 {
	buf, err := l.db.Get([]byte(key), l.rOpts)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil
		}

		panic(err)
	}

	return decodeF32s(buf)
}

func (l *ldbPolicy) putFloatSlice(key string, v []float32) {
	buf := encodeF32s(v)
	err := l.db.Put([]byte(key), buf, l.wOpts)
	if err != nil {
		panic(err)
	}
}

// TODO: Implement compression when encoding/decoding.
func encodeF32s(v []float32) []byte {
	result := make([]byte, 4*len(v))
	for i, v := range v {
		bits := math.Float32bits(v)
		buf := result[4*i : 4*(i+1)]
		binary.LittleEndian.PutUint32(buf, bits)
	}

	return result
}

func decodeF32s(buf []byte) []float32 {
	if len(buf)%4 != 0 {
		panic(fmt.Errorf("invalid encoded buffer of floats has len %d", len(buf)))
	}

	n := len(buf) / 4
	result := make([]float32, n)
	for i := 0; i < n; i++ {
		b := buf[4*i : 4*(i+1)]
		bits := binary.LittleEndian.Uint32(b)
		x := math.Float32frombits(bits)
		result[i] = x
	}

	return result
}

func uniformDist(n int) []float32 {
	result := make([]float32, n)
	p := 1.0 / float32(n)
	f32.AddConst(p, result)
	return result
}

func makePositive(v []float32) {
	for i := range v {
		if v[i] < 0 {
			v[i] = 0.0
		}
	}
}
