// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package inmem

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	tableNameMetadata  = "metadata"
	tableNameResources = "resources"

	indexNameID    = "id"
	indexNameOwner = "owner"

	metaKeyEventIndex = "index"
)

func newDB() (*memdb.MemDB, error) {
	return memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			tableNameMetadata: {
				Name: tableNameMetadata,
				Indexes: map[string]*memdb.IndexSchema{
					indexNameID: {
						Name:         indexNameID,
						AllowMissing: false,
						Unique:       true,
						Indexer:      &memdb.StringFieldIndex{Field: "Key"},
					},
				},
			},
			tableNameResources: {
				Name: tableNameResources,
				Indexes: map[string]*memdb.IndexSchema{
					indexNameID: {
						Name:         indexNameID,
						AllowMissing: false,
						Unique:       true,
						Indexer:      idIndexer{},
					},
					indexNameOwner: {
						Name:         indexNameOwner,
						AllowMissing: true,
						Unique:       false,
						Indexer:      ownerIndexer{},
					},
				},
			},
		},
	})
}

// indexSeparator delimits the segments of our radix tree keys.
const indexSeparator = "\x00"

// idIndexer implements the memdb.Indexer, memdb.SingleIndexer and
// memdb.PrefixIndexer interfaces. It is used for indexing resources
// by their IDs.
type idIndexer struct{}

// FromArgs constructs a radix tree key from an ID for lookup.
func (i idIndexer) FromArgs(args ...any) ([]byte, error) {
	if l := len(args); l != 1 {
		return nil, fmt.Errorf("expected 1 arg, got: %d", l)
	}
	id, ok := args[0].(*pbresource.ID)
	if !ok {
		return nil, fmt.Errorf("expected *pbresource.ID, got: %T", args[0])
	}
	return indexFromID(id, false), nil
}

// FromObject constructs a radix tree key from a Resource at write-time, or an
// ID at delete-time.
func (i idIndexer) FromObject(raw any) (bool, []byte, error) {
	switch t := raw.(type) {
	case *pbresource.ID:
		return true, indexFromID(t, false), nil
	case *pbresource.Resource:
		return true, indexFromID(t.Id, false), nil
	}
	return false, nil, fmt.Errorf("expected *pbresource.Resource or *pbresource.ID, got: %T", raw)
}

// PrefixFromArgs constructs a radix tree key prefix from a query for listing.
func (i idIndexer) PrefixFromArgs(args ...any) ([]byte, error) {
	if l := len(args); l != 1 {
		return nil, fmt.Errorf("expected 1 arg, got: %d", l)
	}

	q, ok := args[0].(query)
	if !ok {
		return nil, fmt.Errorf("expected query, got: %T", args[0])
	}
	return q.indexPrefix(), nil
}

// ownerIndexer implements the memdb.Indexer and memdb.SingleIndexer interfaces.
// It is used for indexing resources by their owners.
type ownerIndexer struct{}

// FromArgs constructs a radix tree key from an ID for lookup.
func (i ownerIndexer) FromArgs(args ...any) ([]byte, error) {
	if l := len(args); l != 1 {
		return nil, fmt.Errorf("expected 1 arg, got: %d", l)
	}
	id, ok := args[0].(*pbresource.ID)
	if !ok {
		return nil, fmt.Errorf("expected *pbresource.ID, got: %T", args[0])
	}
	return indexFromID(id, true), nil
}

// FromObject constructs a radix key tree from a Resource at write-time.
func (i ownerIndexer) FromObject(raw any) (bool, []byte, error) {
	res, ok := raw.(*pbresource.Resource)
	if !ok {
		return false, nil, fmt.Errorf("expected *pbresource.Resource, got: %T", raw)
	}
	if res.Owner == nil {
		return false, nil, nil
	}
	return true, indexFromID(res.Owner, true), nil
}

func indexFromType(t storage.UnversionedType) []byte {
	var b indexBuilder
	b.String(t.Group)
	b.String(t.Kind)
	return b.Bytes()
}

func indexFromTenancy(t *pbresource.Tenancy) []byte {
	var b indexBuilder
	b.String(t.Partition)
	b.String(t.PeerName)
	b.String(t.Namespace)
	return b.Bytes()
}

func indexFromID(id *pbresource.ID, includeUid bool) []byte {
	var b indexBuilder
	b.Raw(indexFromType(storage.UnversionedTypeFrom(id.Type)))
	b.Raw(indexFromTenancy(id.Tenancy))
	b.String(id.Name)
	if includeUid {
		b.String(id.Uid)
	}
	return b.Bytes()
}

type indexBuilder bytes.Buffer

func (i *indexBuilder) Raw(v []byte) {
	(*bytes.Buffer)(i).Write(v)
}

func (i *indexBuilder) String(s string) {
	(*bytes.Buffer)(i).WriteString(s)
	(*bytes.Buffer)(i).WriteString(indexSeparator)
}

func (i *indexBuilder) Bytes() []byte {
	return (*bytes.Buffer)(i).Bytes()
}

type query struct {
	resourceType storage.UnversionedType
	tenancy      *pbresource.Tenancy
	namePrefix   string
}

// indexPrefix is called by idIndexer.PrefixFromArgs to construct a radix tree
// key prefix for list queries.
//
// Our radix tree keys are structured like so:
//
//	<type><partition><peer><namespace><name>
//
// Where each segment is followed by a NULL terminator.
//
// In order to handle wildcard queries, we return a prefix up to the wildcarded
// field. For example:
//
//	 Query: type={mesh,v1,service}, partition=default, peer=*, namespace=default
//	Prefix: mesh[NULL]v1[NULL]service[NULL]default[NULL]
//
// Which means that we must manually apply filters after the wildcard (i.e.
// namespace in the above example) in the matches method.
func (q query) indexPrefix() []byte {
	var b indexBuilder
	b.Raw(indexFromType(q.resourceType))

	if v := q.tenancy.Partition; v == storage.Wildcard {
		return b.Bytes()
	} else {
		b.String(v)
	}

	if v := q.tenancy.PeerName; v == storage.Wildcard {
		return b.Bytes()
	} else {
		b.String(v)
	}

	if v := q.tenancy.Namespace; v == storage.Wildcard {
		return b.Bytes()
	} else {
		b.String(v)
	}

	if q.namePrefix != "" {
		b.Raw([]byte(q.namePrefix))
	}

	return b.Bytes()
}

// matches applies filters that couldn't be applied by just doing a radix tree
// prefix scan, because an earlier segment of the key prefix was wildcarded.
//
// See docs on query.indexPrefix for an example.
func (q query) matches(res *pbresource.Resource) bool {
	if q.tenancy.Partition != storage.Wildcard && res.Id.Tenancy.Partition != q.tenancy.Partition {
		return false
	}

	if q.tenancy.PeerName != storage.Wildcard && res.Id.Tenancy.PeerName != q.tenancy.PeerName {
		return false
	}

	if q.tenancy.Namespace != storage.Wildcard && res.Id.Tenancy.Namespace != q.tenancy.Namespace {
		return false
	}

	if len(q.namePrefix) != 0 && !strings.HasPrefix(res.Id.Name, q.namePrefix) {
		return false
	}

	return true
}
