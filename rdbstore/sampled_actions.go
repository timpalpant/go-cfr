package rdbstore

import (
	"encoding/binary"
	"fmt"

	rocksdb "github.com/tecbot/gorocksdb"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/internal/sampling"
)

// RDBSampledActions implements cfr.SampledActions by storing
// all sampled actions in a LevelDB database on disk.
//
// It is functionally equivalent to cfr.SampledActionMap. In practice, it will be
// much slower but use a constant amount of memory even if the game tree is very large.
type RDBSampledActions struct {
	path  string
	db    *rocksdb.DB
	opts  *rocksdb.Options
	rOpts *rocksdb.ReadOptions
	wOpts *rocksdb.WriteOptions
}

// NewRDBSampledActions creates a new RDBSampledActions backed by
// the given LevelDB database.
func NewRDBSampledActions(path string, opts *rocksdb.Options) (*RDBSampledActions, error) {
	db, err := rocksdb.OpenDb(opts, path)
	if err != nil {
		return nil, err
	}

	return &RDBSampledActions{
		path:  path,
		db:    db,
		opts:  opts,
		rOpts: rocksdb.NewDefaultReadOptions(),
		wOpts: rocksdb.NewDefaultWriteOptions(),
	}, nil
}

// Close implements io.Closer.
func (r *RDBSampledActions) Close() error {
	r.db.Close()
	r.rOpts.Destroy()
	r.wOpts.Destroy()
	return rocksdb.DestroyDb(r.path, r.opts)
}

// Get implements cfr.SampledActions.
func (r *RDBSampledActions) Get(node cfr.GameTreeNode, policy cfr.NodePolicy) int {
	key := []byte(node.InfoSet(node.Player()).Key())
	result, err := r.db.Get(r.rOpts, key)
	if err != nil { // TODO: We're assuming the error is not found.
		panic(err)
	}
	defer result.Free()

	if len(result.Data()) == 0 {
		i := sampling.SampleOne(policy.GetStrategy())
		r.put(key, i)
		return i
	}

	i, ok := binary.Uvarint(result.Data())
	if ok <= 0 {
		panic(fmt.Errorf("error decoding buffer (%d): %v", ok, result.Data()))
	}

	return int(i)
}

func (r *RDBSampledActions) put(key []byte, selected int) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(selected))
	if err := r.db.Put(r.wOpts, key, buf[:n]); err != nil {
		panic(err)
	}
}
