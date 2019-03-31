package ldbstore

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"math/rand"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/timpalpant/go-cfr"
	"github.com/timpalpant/go-cfr/deepcfr"
)

// ReservoirBuffer implements a reservoir sampling buffer in which samples are
// stored in a LevelDB database.
//
// It is functionally equivalent to deepcfr.ReservoirBuffer. In practice, it will
// be somewhat slower but use less memory since all samples are kept on disk.
type ReservoirBuffer struct {
	path    string
	opts    *opt.Options
	maxSize int

	mx sync.Mutex
	n  int

	db    *leveldb.DB
	rOpts *opt.ReadOptions
	wOpts *opt.WriteOptions
}

// NewReservoirBuffer returns a new ReservoirBuffer with the given max number of samples,
// backed by a LevelDB database at the given directory path.
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

// Close implements io.Closer.
func (b *ReservoirBuffer) Close() error {
	return b.db.Close()
}

// AddSample implements deepcfr.Buffer.
func (b *ReservoirBuffer) AddSample(infoSet cfr.InfoSet, advantages []float32, weight float32) {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.n++

	if b.n <= b.maxSize {
		s := deepcfr.NewSample(infoSet, advantages, weight)
		b.putSample(b.n-1, s)
	} else {
		m := rand.Intn(b.n)
		if m < b.maxSize {
			s := deepcfr.NewSample(infoSet, advantages, weight)
			b.putSample(m, s)
		}
	}
}

func (b *ReservoirBuffer) putSample(idx int, s deepcfr.Sample) {
	var buf [binary.MaxVarintLen64]byte
	m := binary.PutUvarint(buf[:], uint64(idx))
	key := buf[:m]

	value, err := s.MarshalBinary()
	if err != nil {
		panic(err)
	}

	if err := b.db.Put(key, value, b.wOpts); err != nil {
		panic(err)
	}
}

// GetSamples implements deepcfr.Buffer.
func (b *ReservoirBuffer) GetSamples() []deepcfr.Sample {
	iter := b.db.NewIterator(nil, b.rOpts)
	var samples []deepcfr.Sample
	for iter.Next() {
		var sample deepcfr.Sample
		if err := sample.UnmarshalBinary(iter.Value()); err != nil {
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

// MarshalBinary implements encoding.BinaryMarshaler.
func (b *ReservoirBuffer) MarshalBinary() ([]byte, error) {
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

// UnmarshalBinary implements encoding.BinaryMarshaler.
func (b *ReservoirBuffer) UnmarshalBinary(buf []byte) error {
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

func init() {
	gob.Register(&ReservoirBuffer{})
}
