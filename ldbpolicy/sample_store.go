package ldbpolicy

import (
	"encoding/binary"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type LDBSampledActionStore struct {
	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

func NewLDBSampledActionStore(db *leveldb.DB) *LDBSampledActionStore {
	return &LDBSampledActionStore{db: db}
}

func (l *LDBSampledActionStore) Close() error {
	return l.db.Close()
}

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

func (l *LDBSampledActionStore) Put(key string, selected int) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(selected))
	if err := l.db.Put([]byte(key), buf[:n], l.wOpts); err != nil {
		panic(err)
	}
}
