package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

// indexerSingle implements both memdb.Indexer and memdb.SingleIndexer. It may
// be used in a memdb.IndexSchema to specify functions that generate the index
// value for memdb.Txn operations.
type indexerSingle struct {
	// readIndex is used by memdb for Txn.Get, Txn.First, and other operations
	// that read data.
	readIndex
	// writeIndex is used by memdb for Txn.Insert, Txn.Delete, for operations
	// that write data to the index.
	writeIndex
}

// indexerMulti implements both memdb.Indexer and memdb.MultiIndexer. It may
// be used in a memdb.IndexSchema to specify functions that generate the index
// value for memdb.Txn operations.
type indexerMulti struct {
	// readIndex is used by memdb for Txn.Get, Txn.First, and other operations
	// that read data.
	readIndex
	// writeIndexMulti is used by memdb for Txn.Insert, Txn.Delete, for operations
	// that write data to the index.
	writeIndexMulti
}

// indexerSingleWithPrefix is a indexerSingle which also supports prefix queries.
type indexerSingleWithPrefix struct {
	readIndex
	writeIndex
	prefixIndex
}

// readIndex implements memdb.Indexer. It exists so that a function can be used
// to provide the interface.
//
// Unlike memdb.Indexer, a readIndex function accepts only a single argument. To
// generate an index from multiple values, use a struct type with multiple fields.
type readIndex func(arg interface{}) ([]byte, error)

func (f readIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("index supports only a single arg")
	}
	return f(args[0])
}

var errMissingValueForIndex = fmt.Errorf("object is missing a value for this index")

// writeIndex implements memdb.SingleIndexer. It exists so that a function
// can be used to provide this interface.
//
// Instead of a bool return value, writeIndex expects errMissingValueForIndex to
// indicate that an index could not be build for the object. It will translate
// this error into a false value to satisfy the memdb.SingleIndexer interface.
type writeIndex func(raw interface{}) ([]byte, error)

func (f writeIndex) FromObject(raw interface{}) (bool, []byte, error) {
	v, err := f(raw)
	if errors.Is(err, errMissingValueForIndex) {
		return false, nil, nil
	}
	return err == nil, v, err
}

// writeIndexMulti implements memdb.MultiIndexer. It exists so that a function
// can be used to provide this interface.
//
// Instead of a bool return value, writeIndexMulti expects errMissingValueForIndex to
// indicate that an index could not be build for the object. It will translate
// this error into a false value to satisfy the memdb.MultiIndexer interface.
type writeIndexMulti func(raw interface{}) ([][]byte, error)

func (f writeIndexMulti) FromObject(raw interface{}) (bool, [][]byte, error) {
	v, err := f(raw)
	if errors.Is(err, errMissingValueForIndex) {
		return false, nil, nil
	}
	return err == nil, v, err
}

// prefixIndex implements memdb.PrefixIndexer. It exists so that a function
// can be used to provide this interface.
type prefixIndex func(args interface{}) ([]byte, error)

func (f prefixIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("index supports only a single arg")
	}
	return f(args[0])
}

const null = "\x00"

// indexBuilder is a buffer used to construct memdb index values.
type indexBuilder bytes.Buffer

func newIndexBuilder(cap int) *indexBuilder {
	buff := make([]byte, 0, cap)
	b := bytes.NewBuffer(buff)
	return (*indexBuilder)(b)
}

// String appends the string and a null terminator to the buffer.
func (b *indexBuilder) String(v string) {
	(*bytes.Buffer)(b).WriteString(v)
	(*bytes.Buffer)(b).WriteString(null)
}

func (b *indexBuilder) Int64(v int64) {
	const size = binary.MaxVarintLen64

	// Get the value and encode it
	buf := make([]byte, size)
	binary.PutVarint(buf, v)
	b.Raw(buf)
}

// Raw appends the bytes without a null terminator to the buffer. Raw should
// only be used when v has a fixed length, or when building the last segment of
// a prefix index.
func (b *indexBuilder) Raw(v []byte) {
	(*bytes.Buffer)(b).Write(v)
}

func (b *indexBuilder) Bytes() []byte {
	return (*bytes.Buffer)(b).Bytes()
}

// singleValueID is an interface that may be implemented by any type that should
// be indexed by a single ID and a structs.EnterpriseMeta to scope the ID.
type singleValueID interface {
	IDValue() string
	PartitionOrDefault() string
	NamespaceOrDefault() string
}

type multiValueID interface {
	IDValue() []string
	PartitionOrDefault() string
	NamespaceOrDefault() string
}

var _ singleValueID = (*structs.DirEntry)(nil)
var _ singleValueID = (*Tombstone)(nil)
var _ singleValueID = (*Query)(nil)
var _ singleValueID = (*structs.Session)(nil)

// indexFromIDValue creates an index key from any struct that implements singleValueID
func indexFromIDValueLowerCase(raw interface{}) ([]byte, error) {
	e, ok := raw.(singleValueID)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T, does not implement singleValueID", raw)
	}

	v := strings.ToLower(e.IDValue())
	if v == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(v)
	return b.Bytes(), nil
}

// indexFromIDValue creates an index key from any struct that implements singleValueID
func indexFromMultiValueID(raw interface{}) ([]byte, error) {
	e, ok := raw.(multiValueID)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T, does not implement multiValueID", raw)
	}
	var b indexBuilder
	for _, v := range e.IDValue() {
		if v == "" {
			return nil, errMissingValueForIndex
		}
		b.String(strings.ToLower(v))
	}
	return b.Bytes(), nil
}

func (b *indexBuilder) Bool(v bool) {
	b.Raw([]byte{intFromBool(v)})
}

type TimeQuery struct {
	Value time.Time
	structs.EnterpriseMeta
}

// NamespaceOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q TimeQuery) NamespaceOrDefault() string {
	return q.EnterpriseMeta.NamespaceOrDefault()
}

// PartitionOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q TimeQuery) PartitionOrDefault() string {
	return q.EnterpriseMeta.PartitionOrDefault()
}

func (b *indexBuilder) Time(t time.Time) {
	val := t.Unix()
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(val))
	(*bytes.Buffer)(b).Write(buf)
}
