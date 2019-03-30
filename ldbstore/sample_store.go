package ldbstore

import (
	"encoding/binary"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// LDBSampledActionStore implements cfr.SampledActionStore by storing
// all sampled actions in a LevelDB database on disk.
//
// It is functionally equivalent to cfr.SampledActionMap. In practice, it will be
// much slower but use a constant amount of memory even if the game tree is very large.
type LDBSampledActionStore struct {
	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

// NewLDBSampledActionStore creates a new LDBSampledActionStore backed by
// the given LevelDB database.
func NewLDBSampledActionStore(db *leveldb.DB) *LDBSampledActionStore {
	return &LDBSampledActionStore{db: db}
}

// Close implements io.Closer.
func (l *LDBSampledActionStore) Close() error {
	return l.db.Close()
}

// Get implements cfr.SampledActionStore.
func (l *LDBSampledActionStore) Get(key string) (int, bool) {
	buf, err := l.db.Get([]byte(key), l.rOpts)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return -1, false
		}

		panic(err)
	}

	i, ok := binary.Uvarint(buf)
	if ok <= 0 {
		panic(fmt.Errorf("error decoding buffer: %v", ok))
	}

	return int(i), true
}

// Put implements cfr.SampledActionStore.
func (l *LDBSampledActionStore) Put(key string, selected int) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(selected))
	if err := l.db.Put([]byte(key), buf[:n], l.wOpts); err != nil {
		panic(err)
	}
}
