package rdbstore

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"math/rand"
	"sync"

	rocksdb "github.com/tecbot/gorocksdb"

	"github.com/timpalpant/go-cfr/deepcfr"
)

// ReservoirBuffer implements a reservoir sampling buffer in which samples are
// stored in a LevelDB database.
//
// It is functionally equivalent to deepcfr.ReservoirBuffer. In practice, it will
// be somewhat slower but use less memory since all samples are kept on disk.
type ReservoirBuffer struct {
	params  Params
	db      *rocksdb.DB
	maxSize int

	mx sync.Mutex
	n  int
}

// NewReservoirBuffer returns a new ReservoirBuffer with the given max number of samples,
// backed by a LevelDB database at the given directory path.
func NewReservoirBuffer(params Params, maxSize int) (*ReservoirBuffer, error) {
	db, err := rocksdb.OpenDb(params.Options, params.Path)
	if err != nil {
		return nil, err
	}

	return &ReservoirBuffer{
		params:  params,
		db:      db,
		maxSize: maxSize,
	}, nil
}

// Close implements io.Closer.
func (b *ReservoirBuffer) Close() error {
	b.db.Close()
	return nil
}

// AddSample implements deepcfr.Buffer.
func (b *ReservoirBuffer) AddSample(s deepcfr.Sample) {
	b.mx.Lock()
	defer b.mx.Unlock()
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

	if err := b.db.Put(b.params.WriteOptions, key, value.Bytes()); err != nil {
		panic(err)
	}
}

// GetSamples implements deepcfr.Buffer.
func (b *ReservoirBuffer) GetSamples() []deepcfr.Sample {
	it := b.db.NewIterator(b.params.ReadOptions)
	defer it.Close()

	var samples []deepcfr.Sample
	for it.SeekToFirst(); it.Valid(); it.Next() {
		r := bytes.NewReader(it.Value().Data())
		dec := gob.NewDecoder(r)
		var sample deepcfr.Sample
		if err := dec.Decode(&sample); err != nil {
			panic(err)
		}

		samples = append(samples, sample)
		it.Key().Free()
		it.Value().Free()
	}

	if err := it.Err(); err != nil {
		panic(err)
	}

	return samples
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (b *ReservoirBuffer) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(b.params.Path); err != nil {
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

	// TODO: Serialize and reload RocksDB options.
	var path string
	if err := dec.Decode(&path); err != nil {
		return err
	}
	b.params = DefaultParams(path)

	if err := dec.Decode(&b.maxSize); err != nil {
		return err
	}

	if err := dec.Decode(&b.n); err != nil {
		return err
	}

	b.params.Options.SetCreateIfMissing(false)
	db, err := rocksdb.OpenDb(b.params.Options, b.params.Path)
	if err != nil {
		return err
	}

	b.db = db
	return nil
}

func init() {
	gob.Register(&ReservoirBuffer{})
}
