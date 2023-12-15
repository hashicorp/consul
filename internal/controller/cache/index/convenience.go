// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func IndexFromID(id *pbresource.ID, includeUid bool) []byte {
	var b Builder
	b.Raw(IndexFromType(id.Type))
	b.Raw(IndexFromTenancy(id.Tenancy))
	b.String(id.Name)
	if includeUid {
		b.String(id.Uid)
	}
	return b.Bytes()
}

func IndexFromRefOrID(ref resource.ReferenceOrID) []byte {
	var b Builder
	b.Raw(IndexFromType(ref.GetType()))
	b.Raw(IndexFromTenancy(ref.GetTenancy()))
	b.String(ref.GetName())
	return b.Bytes()
}

func PrefixIndexFromRefOrID(ref resource.ReferenceOrID) []byte {
	var b Builder
	b.Raw(IndexFromType(ref.GetType()))

	raw, done := prefixIndexFromTenancy(ref.GetTenancy())
	b.Raw(raw)

	if done {
		return b.Bytes()
	}

	b.Raw([]byte(ref.GetName()))
	return b.Bytes()
}

func prefixIndexFromTenancy(t *pbresource.Tenancy) ([]byte, bool) {
	var b Builder
	partition := t.GetPartition()
	if partition == "" || partition == storage.Wildcard {
		return b.Bytes(), true
	}

	b.String(partition)

	namespace := t.GetNamespace()

	if namespace == "" || namespace == storage.Wildcard {
		return b.Bytes(), true
	}

	b.String(namespace)

	return b.Bytes(), false
}

func IndexFromType(t *pbresource.Type) []byte {
	var b Builder
	b.String(t.Group)
	b.String(t.Kind)
	return b.Bytes()
}

func IndexFromTenancy(t *pbresource.Tenancy) []byte {
	var b Builder
	b.String(t.GetPartition())
	b.String(t.GetNamespace())
	return b.Bytes()
}

var ReferenceOrIDFromArgs = SingleValueFromArgs[resource.ReferenceOrID](func(r resource.ReferenceOrID) ([]byte, error) {
	return IndexFromRefOrID(r), nil
})

var PrefixReferenceOrIDFromArgs = SingleValueFromArgs[resource.ReferenceOrID](func(r resource.ReferenceOrID) ([]byte, error) {
	return PrefixIndexFromRefOrID(r), nil
})

func SingleValueFromArgs[T any](indexer func(value T) ([]byte, error)) func(args ...any) ([]byte, error) {
	return func(args ...any) ([]byte, error) {
		var zero T
		if l := len(args); l != 1 {
			return nil, fmt.Errorf("expected 1 arg, got: %d", l)
		}

		value, ok := args[0].(T)
		if !ok {
			return nil, fmt.Errorf("expected %T, got: %T", zero, args[0])
		}

		return indexer(value)
	}
}

func SingleValueFromOneOrTwoArgs[T1 any, T2 any](indexer func(value T1, optional T2) ([]byte, error)) func(args ...any) ([]byte, error) {
	return func(args ...any) ([]byte, error) {
		var value T1
		var optional T2

		l := len(args)
		switch l {
		case 2:
			val, ok := args[1].(T2)
			if !ok {
				return nil, fmt.Errorf("expected second argument type of %T, got: %T", optional, args[1])
			}
			optional = val
			fallthrough
		case 1:
			val, ok := args[0].(T1)
			if !ok {
				return nil, fmt.Errorf("expected first argument type of %T, got: %T", value, args[1])
			}
			value = val
		default:
			return nil, fmt.Errorf("expected 1 or 2 args, got: %d", l)
		}

		return indexer(value, optional)
	}
}
