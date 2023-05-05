package resource_test

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestEqualType(t *testing.T) {
	t.Run("same pointer", func(t *testing.T) {
		typ := &pbresource.Type{
			Group:        "foo",
			GroupVersion: "v1",
			Kind:         "bar",
		}
		require.True(t, resource.EqualType(typ, typ))
	})

	t.Run("equal", func(t *testing.T) {
		a := &pbresource.Type{
			Group:        "foo",
			GroupVersion: "v1",
			Kind:         "bar",
		}
		b := clone(a)
		require.True(t, resource.EqualType(a, b))
	})

	t.Run("different Group", func(t *testing.T) {
		a := &pbresource.Type{
			Group:        "foo",
			GroupVersion: "v1",
			Kind:         "bar",
		}
		b := clone(a)
		b.Group = "bar"
		require.False(t, resource.EqualType(a, b))
	})

	t.Run("different GroupVersion", func(t *testing.T) {
		a := &pbresource.Type{
			Group:        "foo",
			GroupVersion: "v1",
			Kind:         "bar",
		}
		b := clone(a)
		b.GroupVersion = "v2"
		require.False(t, resource.EqualType(a, b))
	})

	t.Run("different Kind", func(t *testing.T) {
		a := &pbresource.Type{
			Group:        "foo",
			GroupVersion: "v1",
			Kind:         "bar",
		}
		b := clone(a)
		b.Kind = "baz"
		require.False(t, resource.EqualType(a, b))
	})
}

func TestEqualTenancy(t *testing.T) {
	t.Run("same pointer", func(t *testing.T) {
		ten := &pbresource.Tenancy{
			Partition: "foo",
			PeerName:  "bar",
			Namespace: "baz",
		}
		require.True(t, resource.EqualTenancy(ten, ten))
	})

	t.Run("equal", func(t *testing.T) {
		a := &pbresource.Tenancy{
			Partition: "foo",
			PeerName:  "bar",
			Namespace: "baz",
		}
		b := clone(a)
		require.True(t, resource.EqualTenancy(a, b))
	})

	t.Run("different Partition", func(t *testing.T) {
		a := &pbresource.Tenancy{
			Partition: "foo",
			PeerName:  "bar",
			Namespace: "baz",
		}
		b := clone(a)
		b.Partition = "qux"
		require.False(t, resource.EqualTenancy(a, b))
	})

	t.Run("different PeerName", func(t *testing.T) {
		a := &pbresource.Tenancy{
			Partition: "foo",
			PeerName:  "bar",
			Namespace: "baz",
		}
		b := clone(a)
		b.PeerName = "qux"
		require.False(t, resource.EqualTenancy(a, b))
	})

	t.Run("different Namespace", func(t *testing.T) {
		a := &pbresource.Tenancy{
			Partition: "foo",
			PeerName:  "bar",
			Namespace: "baz",
		}
		b := clone(a)
		b.Namespace = "qux"
		require.False(t, resource.EqualTenancy(a, b))
	})
}

func TestEqualID(t *testing.T) {
	t.Run("same pointer", func(t *testing.T) {
		id := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "demo",
				GroupVersion: "v2",
				Kind:         "artist",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "foo",
				PeerName:  "bar",
				Namespace: "baz",
			},
			Name: "qux",
			Uid:  ulid.Make().String(),
		}
		require.True(t, resource.EqualID(id, id))
	})

	t.Run("equal", func(t *testing.T) {
		a := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "demo",
				GroupVersion: "v2",
				Kind:         "artist",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "foo",
				PeerName:  "bar",
				Namespace: "baz",
			},
			Name: "qux",
			Uid:  ulid.Make().String(),
		}
		b := clone(a)
		require.True(t, resource.EqualID(a, b))
	})

	t.Run("different type", func(t *testing.T) {
		a := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "demo",
				GroupVersion: "v2",
				Kind:         "artist",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "foo",
				PeerName:  "bar",
				Namespace: "baz",
			},
			Name: "qux",
			Uid:  ulid.Make().String(),
		}
		b := clone(a)
		b.Type.Kind = "album"
		require.False(t, resource.EqualID(a, b))
	})

	t.Run("different tenancy", func(t *testing.T) {
		a := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "demo",
				GroupVersion: "v2",
				Kind:         "artist",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "foo",
				PeerName:  "bar",
				Namespace: "baz",
			},
			Name: "qux",
			Uid:  ulid.Make().String(),
		}
		b := clone(a)
		b.Tenancy.Namespace = "qux"
		require.False(t, resource.EqualID(a, b))
	})

	t.Run("different name", func(t *testing.T) {
		a := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "demo",
				GroupVersion: "v2",
				Kind:         "artist",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "foo",
				PeerName:  "bar",
				Namespace: "baz",
			},
			Name: "qux",
			Uid:  ulid.Make().String(),
		}
		b := clone(a)
		b.Name = "boom"
		require.False(t, resource.EqualID(a, b))
	})

	t.Run("different uid", func(t *testing.T) {
		a := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "demo",
				GroupVersion: "v2",
				Kind:         "artist",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "foo",
				PeerName:  "bar",
				Namespace: "baz",
			},
			Name: "qux",
			Uid:  ulid.Make().String(),
		}
		b := clone(a)
		b.Uid = ulid.Make().String()
		require.False(t, resource.EqualID(a, b))
	})
}

func BenchmarkEqualType(b *testing.B) {
	// cpu: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz
	// BenchmarkEqualType/ours-16              197336331                5.877 ns/op
	// BenchmarkEqualType/reflection-16         2424975               492.3 ns/op
	typeA := &pbresource.Type{
		Group:        "foo",
		GroupVersion: "v1",
		Kind:         "bar",
	}
	typeB := &pbresource.Type{
		Group:        "foo",
		GroupVersion: "v1",
		Kind:         "baz",
	}
	b.ResetTimer()

	b.Run("ours", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resource.EqualType(typeA, typeB)
		}
	})

	b.Run("reflection", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = proto.Equal(typeA, typeB)
		}
	})
}

func BenchmarkEqualTenancy(b *testing.B) {
	// cpu: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz
	// BenchmarkEqualTenancy/ours-16                   163274274                7.229 ns/op
	// BenchmarkEqualTenancy/reflection-16              2474611               495.1 ns/op
	tenA := &pbresource.Tenancy{
		Partition: "foo",
		PeerName:  "bar",
		Namespace: "baz",
	}
	tenB := &pbresource.Tenancy{
		Partition: "foo",
		PeerName:  "bar",
		Namespace: "qux",
	}
	b.ResetTimer()

	b.Run("ours", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resource.EqualTenancy(tenA, tenB)
		}
	})

	b.Run("reflection", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = proto.Equal(tenA, tenB)
		}
	})
}

func BenchmarkEqualID(b *testing.B) {
	// cpu: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz
	// BenchmarkEqualID/ours-16                        74521321                15.61 ns/op
	// BenchmarkEqualID/reflection-16                   3794371               292.4 ns/op
	idA := &pbresource.ID{
		Type: &pbresource.Type{
			Group:        "demo",
			GroupVersion: "v2",
			Kind:         "artist",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "foo",
			PeerName:  "bar",
			Namespace: "baz",
		},
		Name: "qux",
		Uid:  ulid.Make().String(),
	}
	idB := clone(idA)
	idB.Uid = ulid.Make().String()
	b.ResetTimer()

	b.Run("ours", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resource.EqualID(idA, idB)
		}
	})

	b.Run("reflection", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = proto.Equal(idA, idB)
		}
	})
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
