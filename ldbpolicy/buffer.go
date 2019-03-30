package ldbpolicy

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"math/rand"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/timpalpant/go-cfr/deepcfr"
)

type ReservoirBuffer struct {
	path    string
	opts    *opt.Options
	maxSize int
	n       int

	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

func NewReservoirBuffer(path string, opts *opt.Options, maxSize int) (*ReservoirBuffer, error) {
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		return nil, err
	}

	return &ReservoirBuffer{
		path:    path,
		opts:    opts,
		maxSize: maxSize,
		db:      db,
	}, nil
}

func (b *ReservoirBuffer) Close() error {
	return b.db.Close()
}

func (b *ReservoirBuffer) AddSample(s deepcfr.Sample) {
	b.n++

	if b.n <= b.maxSize {
		b.putSample(b.n-1, s)
	} else {
		m := rand.Intn(b.n)
		if m < b.maxSize {
			b.putSample(m, s)
		}
	}
}

func (b *ReservoirBuffer) putSample(idx int, s deepcfr.Sample) {
	var buf [binary.MaxVarintLen64]byte
	m := binary.PutUvarint(buf[:], uint64(idx))
	key := buf[:m]

	var value bytes.Buffer
	enc := gob.NewEncoder(&value)
	if err := enc.Encode(s); err != nil {
		panic(err)
	}

	if err := b.db.Put(key, value.Bytes(), b.wOpts); err != nil {
		panic(err)
	}
}

func (b *ReservoirBuffer) GetSamples() []deepcfr.Sample {
	iter := b.db.NewIterator(nil, b.rOpts)
	var samples []deepcfr.Sample
	for iter.Next() {
		r := bytes.NewReader(iter.Value())
		dec := gob.NewDecoder(r)
		var sample deepcfr.Sample
		if err := dec.Decode(&sample); err != nil {
			panic(err)
		}

		samples = append(samples, sample)
	}

	iter.Release()
	if err := iter.Error(); err != nil {
		panic(err)
	}

	return samples
}

func (b *ReservoirBuffer) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(b.path); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.opts); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.maxSize); err != nil {
		return nil, err
	}

	if err := enc.Encode(b.n); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (b *ReservoirBuffer) GobDecode(buf []byte) error {
	r := bytes.NewReader(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&b.path); err != nil {
		return err
	}

	if err := dec.Decode(&b.opts); err != nil {
		return err
	}

	if err := dec.Decode(&b.maxSize); err != nil {
		return err
	}

	if err := dec.Decode(&b.n); err != nil {
		return err
	}

	b.opts.ErrorIfMissing = true
	db, err := leveldb.OpenFile(b.path, b.opts)
	if err != nil {
		return err
	}

	b.db = db
	return nil
}
