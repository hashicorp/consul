package cache

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func IndexFromID(id *pbresource.ID, includeUid bool) []byte {
	var b IndexBuilder
	b.Raw(IndexFromType(id.Type))
	b.Raw(IndexFromTenancy(id.Tenancy))
	b.String(id.Name)
	if includeUid {
		b.String(id.Uid)
	}
	return b.Bytes()
}

func IndexFromRefOrID(ref resource.ReferenceOrID) []byte {
	var b IndexBuilder
	b.Raw(IndexFromType(ref.GetType()))
	b.Raw(IndexFromTenancy(ref.GetTenancy()))
	b.String(ref.GetName())
	return b.Bytes()
}

func IndexFromType(t *pbresource.Type) []byte {
	var b IndexBuilder
	b.String(t.Group)
	b.String(t.Kind)
	return b.Bytes()
}

func IndexFromTenancy(t *pbresource.Tenancy) []byte {
	var b IndexBuilder
	b.String(t.Partition)
	b.String(t.PeerName)
	b.String(t.Namespace)
	return b.Bytes()
}

type IndexBuilder bytes.Buffer

func (i *IndexBuilder) Raw(v []byte) {
	(*bytes.Buffer)(i).Write(v)
}

func (i *IndexBuilder) String(s string) {
	(*bytes.Buffer)(i).WriteString(s)
	(*bytes.Buffer)(i).WriteString(indexSeparator)
}

func (i *IndexBuilder) Bytes() []byte {
	return (*bytes.Buffer)(i).Bytes()
}

func ReferenceOrIDFromArgs(args ...any) ([]byte, error) {
	if l := len(args); l != 1 {
		return nil, fmt.Errorf("expected 1 arg, got: %d", l)
	}
	ref, ok := args[0].(resource.ReferenceOrID)
	if !ok {
		return nil, fmt.Errorf("expected ReferenceOrID, got: %T", args[0])
	}

	return IndexFromRefOrID(ref), nil
}

func PrefixReferenceOrIDFromArgs(args ...any) ([]byte, error) {
	if l := len(args); l != 1 {
		return nil, fmt.Errorf("expected 1 arg, got: %d", l)
	}
	ref, ok := args[0].(resource.ReferenceOrID)
	if !ok {
		return nil, fmt.Errorf("expected ReferenceOrID, got: %T", args[0])
	}

	return IndexFromRefOrID(ref), nil
}
