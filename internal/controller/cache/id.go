package cache

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

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
	id, ok := args[0].(resource.ReferenceOrID)
	if !ok {
		return nil, fmt.Errorf("expected *pbresource.ID or *pbresource.Reference, got: %T", args[0])
	}
	return IndexFromRefOrID(id), nil
}

// FromObject constructs a radix tree key from a Resource at write-time, or an
// ID at delete-time.
func (i idIndexer) FromResource(r *pbresource.Resource) (bool, []byte, error) {
	return true, IndexFromRefOrID(r.GetId()), nil
}

// PrefixFromArgs constructs a radix tree key prefix from a query for listing.
// func (i idIndexer) PrefixFromArgs(args ...any) ([]byte, error) {
// 	if len(args) > 2 {
// 		return nil, fmt.Errorf("expected 1 (tenancy) or 2 (tenancy + name prefix) arguments: got %d", len(args))
// 	}

// 	tenancy, ok := args[0].(*pbresource.Tenancy)
// 	if !ok {
// 		return nil, fmt.Errorf("expected *pbresource.Tenancy as the first argument but got %T", args[0])
// 	}

// 	var b IndexBuilder

// 	if v := tenancy.Partition; v == storage.Wildcard {
// 		return b.Bytes(), nil
// 	} else {
// 		b.String(v)
// 	}

// 	if v := tenancy.PeerName; v == storage.Wildcard {
// 		return b.Bytes(), nil
// 	} else {
// 		b.String(v)
// 	}

// 	if v := tenancy.Namespace; v == storage.Wildcard {
// 		return b.Bytes(), nil
// 	} else {
// 		b.String(v)
// 	}

// 	if len(args) == 2 {
// 		namePrefix, ok := args[1].(string)
// 		if !ok {
// 			return nil, fmt.Errorf("expected string as the second argument but got %T", args[1])
// 		}
// 		b.Raw([]byte(namePrefix))
// 	}

// 	return b.Bytes(), nil
// }
