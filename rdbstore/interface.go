// Package rdbstore implements CFR storage components that keep data
// in a RocksDB database, rather than in memory datastructures.
//
// These implementations are substantially slower than the corresponding in-memory
// components but can scale to games that do not fit in memory.
package rdbstore

import (
	rocksdb "github.com/tecbot/gorocksdb"
)

type Params struct {
	Path         string
	Options      *rocksdb.Options
	ReadOptions  *rocksdb.ReadOptions
	WriteOptions *rocksdb.WriteOptions
}

func DefaultParams(path string) Params {
	opts := rocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)

	return Params{
		Path:         path,
		Options:      opts,
		ReadOptions:  rocksdb.NewDefaultReadOptions(),
		WriteOptions: rocksdb.NewDefaultWriteOptions(),
	}
}

func (p Params) Close() {
	p.Options.Destroy()
	p.ReadOptions.Destroy()
	p.WriteOptions.Destroy()
}
