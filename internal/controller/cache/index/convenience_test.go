// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
)

func TestIndexFromID(t *testing.T) {
	id := &pbresource.ID{
		Type: &pbresource.Type{
			Group:        "test",
			GroupVersion: "v1",
			Kind:         "fake",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "foo",
			Namespace: "bar",
		},
		Name: "baz",
		Uid:  "01G9R78VJ5VXAJRA2K7V0RXB87",
	}

	val := IndexFromID(id, false)
	expected := []byte("test\x00fake\x00foo\x00bar\x00baz\x00")
	require.Equal(t, expected, val)

	val = IndexFromID(id, true)
	expected = []byte("test\x00fake\x00foo\x00bar\x00baz\x0001G9R78VJ5VXAJRA2K7V0RXB87\x00")
	require.Equal(t, expected, val)
}

func TestIndexFromRefOrID(t *testing.T) {
	id := &pbresource.ID{
		Type: &pbresource.Type{
			Group:        "test",
			GroupVersion: "v1",
			Kind:         "fake",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "foo",
			Namespace: "bar",
		},
		Name: "baz",
		Uid:  "01G9R78VJ5VXAJRA2K7V0RXB87",
	}

	ref := resource.Reference(id, "")

	idVal := IndexFromRefOrID(id)
	refVal := IndexFromRefOrID(ref)

	expected := "test\x00fake\x00foo\x00bar\x00baz\x00"
	require.Equal(t, []byte(expected), idVal)
	require.Equal(t, []byte(expected), refVal)
}

func TestPrefixIndexFromRefOrID(t *testing.T) {
	t.Run("no partition", func(t *testing.T) {
		ref := &pbresource.Reference{
			Type: &pbresource.Type{
				Group:        "test",
				GroupVersion: "v1",
				Kind:         "fake",
			},
		}

		require.Equal(t, []byte("test\x00fake\x00"), PrefixIndexFromRefOrID(ref))
	})

	t.Run("no namespace", func(t *testing.T) {
		ref := &pbresource.Reference{
			Type: &pbresource.Type{
				Group:        "test",
				GroupVersion: "v1",
				Kind:         "fake",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "test",
			},
		}

		require.Equal(t, []byte("test\x00fake\x00test\x00"), PrefixIndexFromRefOrID(ref))
	})

	t.Run("name prefix", func(t *testing.T) {
		id := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "test",
				GroupVersion: "v1",
				Kind:         "fake",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "test",
				Namespace: "test",
			},
			Name: "prefix",
		}

		require.Equal(t, []byte("test\x00fake\x00test\x00test\x00prefix"), PrefixIndexFromRefOrID(id))
	})
}

func TestPrefixIndexFromTenancy(t *testing.T) {
	t.Run("nil tenancy", func(t *testing.T) {
		idx, done := prefixIndexFromTenancy(nil)
		require.Len(t, idx, 0)
		require.True(t, done)
	})

	t.Run("partition empty", func(t *testing.T) {
		tenant := &pbresource.Tenancy{}
		idx, done := prefixIndexFromTenancy(tenant)
		require.Len(t, idx, 0)
		require.True(t, done)
	})

	t.Run("partition wildcard", func(t *testing.T) {
		tenant := &pbresource.Tenancy{
			Partition: storage.Wildcard,
		}
		idx, done := prefixIndexFromTenancy(tenant)
		require.Len(t, idx, 0)
		require.True(t, done)
	})

	t.Run("namespace empty", func(t *testing.T) {
		tenant := &pbresource.Tenancy{
			Partition: "test",
			Namespace: "",
		}
		idx, done := prefixIndexFromTenancy(tenant)
		require.Equal(t, []byte("test\x00"), idx)
		require.True(t, done)
	})

	t.Run("namespace wildcard", func(t *testing.T) {
		tenant := &pbresource.Tenancy{
			Partition: "test",
			Namespace: storage.Wildcard,
		}
		idx, done := prefixIndexFromTenancy(tenant)
		require.Equal(t, []byte("test\x00"), idx)
		require.True(t, done)
	})

	t.Run("namespace populated", func(t *testing.T) {
		tenant := &pbresource.Tenancy{
			Partition: "test",
			Namespace: "blah",
		}
		idx, done := prefixIndexFromTenancy(tenant)
		require.Equal(t, []byte("test\x00blah\x00"), idx)
		require.False(t, done)
	})
}

func TestIndexFromType(t *testing.T) {
	val := IndexFromType(&pbresource.Type{
		Group:        "test",
		GroupVersion: "v1",
		Kind:         "fake",
	})

	require.Equal(t, []byte("test\x00fake\x00"), val)

	val = IndexFromType(&pbresource.Type{
		Group:        "test",
		GroupVersion: "v1",
	})

	require.Equal(t, []byte("test\x00\x00"), val)
}

func TestReferenceOrIDFromArgs(t *testing.T) {
	ref := &pbresource.Reference{
		Type: &pbresource.Type{
			Group:        "test",
			GroupVersion: "v1",
			Kind:         "fake",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: "foo",
	}
	t.Run("invalid length", func(t *testing.T) {
		// the second arg will cause a validation error
		val, err := ReferenceOrIDFromArgs(ref, 2)
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("invalid type", func(t *testing.T) {
		val, err := ReferenceOrIDFromArgs("string type unexpected")
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		val, err := ReferenceOrIDFromArgs(ref)
		require.NoError(t, err)
		require.Equal(t, IndexFromRefOrID(ref), val)
	})
}

func TestPrefixReferenceOrIDFromArgs(t *testing.T) {
	ref := &pbresource.Reference{
		Type: &pbresource.Type{
			Group:        "test",
			GroupVersion: "v1",
			Kind:         "fake",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
		},
	}
	t.Run("invalid length", func(t *testing.T) {
		// the second arg will cause a validation error
		val, err := PrefixReferenceOrIDFromArgs(ref, 2)
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("invalid type", func(t *testing.T) {
		val, err := PrefixReferenceOrIDFromArgs("string type unexpected")
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		val, err := PrefixReferenceOrIDFromArgs(ref)
		require.NoError(t, err)
		require.Equal(t, PrefixIndexFromRefOrID(ref), val)
	})
}

func TestSingleValueFromArgs(t *testing.T) {
	injectedError := errors.New("injected test error")

	idx := SingleValueFromArgs(func(val string) ([]byte, error) {
		return []byte(val), nil
	})

	errIdx := SingleValueFromArgs(func(val string) ([]byte, error) {
		return nil, injectedError
	})

	t.Run("invalid length", func(t *testing.T) {
		// the second arg will cause a validation error
		val, err := idx("blah", 2)
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("invalid type", func(t *testing.T) {
		val, err := idx(3)
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("propagate error", func(t *testing.T) {
		val, err := errIdx("blah")
		require.Nil(t, val)
		require.ErrorIs(t, err, injectedError)
	})

	t.Run("ok", func(t *testing.T) {
		val, err := idx("blah")
		require.NoError(t, err)
		require.Equal(t, []byte("blah"), val)
	})
}

func TestSingleValueFromOneOrTwoArgs(t *testing.T) {
	injectedError := errors.New("injected test error")

	idx := SingleValueFromOneOrTwoArgs(func(val string, optional bool) ([]byte, error) {
		if optional {
			return []byte(val + val), nil
		}
		return []byte(val), nil
	})

	errIdx := SingleValueFromOneOrTwoArgs(func(val string, optional bool) ([]byte, error) {
		return nil, injectedError
	})

	t.Run("invalid length", func(t *testing.T) {
		// the third arg will cause a validation error
		val, err := idx("blah", true, 4)
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("invalid first type", func(t *testing.T) {
		val, err := idx(4, true)
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("invalid second type", func(t *testing.T) {
		val, err := idx("blah", 3)
		require.Nil(t, val)
		require.Error(t, err)
	})

	t.Run("propagate error", func(t *testing.T) {
		val, err := errIdx("blah")
		require.Nil(t, val)
		require.ErrorIs(t, err, injectedError)
	})

	t.Run("one arg ok", func(t *testing.T) {
		val, err := idx("blah")
		require.NoError(t, err)
		require.Equal(t, []byte("blah"), val)
	})

	t.Run("two arg ok", func(t *testing.T) {
		val, err := idx("blah", true)
		require.NoError(t, err)
		require.Equal(t, []byte("blahblah"), val)
	})

}
