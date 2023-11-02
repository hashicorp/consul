// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

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

	t.Run("nil", func(t *testing.T) {
		a := &pbresource.Type{
			Group:        "foo",
			GroupVersion: "v1",
			Kind:         "bar",
		}
		require.False(t, resource.EqualType(a, nil))
		require.False(t, resource.EqualType(nil, a))
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

	t.Run("nil", func(t *testing.T) {
		a := &pbresource.Tenancy{
			Partition: "foo",
			PeerName:  "bar",
			Namespace: "baz",
		}
		require.False(t, resource.EqualTenancy(a, nil))
		require.False(t, resource.EqualTenancy(nil, a))
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

	t.Run("nil", func(t *testing.T) {
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
		require.False(t, resource.EqualID(a, nil))
		require.False(t, resource.EqualID(nil, a))
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

func TestEqualReference(t *testing.T) {
	t.Run("same pointer", func(t *testing.T) {
		id := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		require.True(t, resource.EqualReference(id, id))
	})

	t.Run("equal", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := clone(a)
		require.True(t, resource.EqualReference(a, b))
	})

	t.Run("nil", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		require.False(t, resource.EqualReference(a, nil))
		require.False(t, resource.EqualReference(nil, a))
	})

	t.Run("different type", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := clone(a)
		b.Type.Kind = "album"
		require.False(t, resource.EqualReference(a, b))
	})

	t.Run("different tenancy", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := clone(a)
		b.Tenancy.Namespace = "qux"
		require.False(t, resource.EqualReference(a, b))
	})

	t.Run("different name", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := clone(a)
		b.Name = "boom"
		require.False(t, resource.EqualReference(a, b))
	})

	t.Run("different section", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := clone(a)
		b.Section = "not-blah"
		require.False(t, resource.EqualReference(a, b))
	})
}

func TestReferenceOrIDMatch(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := &pbresource.ID{
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
		require.True(t, resource.ReferenceOrIDMatch(a, b))
	})

	t.Run("nil", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := &pbresource.ID{
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
		require.False(t, resource.ReferenceOrIDMatch(a, nil))
		require.False(t, resource.ReferenceOrIDMatch(nil, b))
	})

	t.Run("different type", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := &pbresource.ID{
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
		b.Type.Kind = "album"
		require.False(t, resource.ReferenceOrIDMatch(a, b))
	})

	t.Run("different tenancy", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := &pbresource.ID{
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
		b.Tenancy.Namespace = "qux"
		require.False(t, resource.ReferenceOrIDMatch(a, b))
	})

	t.Run("different name", func(t *testing.T) {
		a := &pbresource.Reference{
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
			Name:    "qux",
			Section: "blah",
		}
		b := &pbresource.ID{
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
		b.Name = "boom"
		require.False(t, resource.ReferenceOrIDMatch(a, b))
	})
}

func TestEqualStatus(t *testing.T) {
	orig := &pbresource.Status{
		ObservedGeneration: ulid.Make().String(),
		Conditions: []*pbresource.Condition{
			{
				Type:    "FooType",
				State:   pbresource.Condition_STATE_TRUE,
				Reason:  "FooReason",
				Message: "Foo is true",
				Resource: &pbresource.Reference{
					Type: &pbresource.Type{
						Group:        "foo-group",
						GroupVersion: "foo-group-version",
						Kind:         "foo-kind",
					},
					Tenancy: &pbresource.Tenancy{
						Partition: "foo-partition",
						PeerName:  "foo-peer-name",
						Namespace: "foo-namespace",
					},
					Name:    "foo-name",
					Section: "foo-section",
				},
			},
		},
	}

	// Equal cases.
	t.Run("same pointer", func(t *testing.T) {
		require.True(t, resource.EqualStatus(orig, orig, true))
	})

	t.Run("equal", func(t *testing.T) {
		require.True(t, resource.EqualStatus(orig, clone(orig), true))
	})

	// Not equal cases.
	t.Run("nil", func(t *testing.T) {
		require.False(t, resource.EqualStatus(orig, nil, true))
		require.False(t, resource.EqualStatus(nil, orig, true))
	})

	testCases := map[string]func(*pbresource.Status){
		"different ObservedGeneration": func(s *pbresource.Status) {
			s.ObservedGeneration = ""
		},
		"different Conditions": func(s *pbresource.Status) {
			s.Conditions = append(s.Conditions, s.Conditions...)
		},
		"nil Condition": func(s *pbresource.Status) {
			s.Conditions[0] = nil
		},
		"different Condition.Type": func(s *pbresource.Status) {
			s.Conditions[0].Type = "BarType"
		},
		"different Condition.State": func(s *pbresource.Status) {
			s.Conditions[0].State = pbresource.Condition_STATE_FALSE
		},
		"different Condition.Reason": func(s *pbresource.Status) {
			s.Conditions[0].Reason = "BarReason"
		},
		"different Condition.Message": func(s *pbresource.Status) {
			s.Conditions[0].Reason = "Bar if false"
		},
		"different Condition.Resource": func(s *pbresource.Status) {
			s.Conditions[0].Resource = nil
		},
		"different Condition.Resource.Type": func(s *pbresource.Status) {
			s.Conditions[0].Resource.Type.Group = "bar-group"
		},
		"different Condition.Resource.Tenancy": func(s *pbresource.Status) {
			s.Conditions[0].Resource.Tenancy.Partition = "bar-partition"
		},
		"different Condition.Resource.Name": func(s *pbresource.Status) {
			s.Conditions[0].Resource.Name = "bar-name"
		},
		"different Condition.Resource.Section": func(s *pbresource.Status) {
			s.Conditions[0].Resource.Section = "bar-section"
		},
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			a, b := clone(orig), clone(orig)
			modFn(b)

			require.False(t, resource.EqualStatus(a, b, true))
			require.False(t, resource.EqualStatus(b, a, true))
		})
	}

	t.Run("compareUpdatedAt = true", func(t *testing.T) {
		a, b := clone(orig), clone(orig)
		b.UpdatedAt = timestamppb.New(b.UpdatedAt.AsTime().Add(1 * time.Minute))
		require.False(t, resource.EqualStatus(a, b, true))
		require.False(t, resource.EqualStatus(b, a, true))
	})

	t.Run("compareUpdatedAt = false", func(t *testing.T) {
		a, b := clone(orig), clone(orig)
		b.UpdatedAt = timestamppb.New(b.UpdatedAt.AsTime().Add(1 * time.Minute))
		require.True(t, resource.EqualStatus(a, b, false))
		require.True(t, resource.EqualStatus(b, a, false))
	})
}

func TestEqualStatusMap(t *testing.T) {
	generation := ulid.Make().String()

	for idx, tc := range []struct {
		a, b  map[string]*pbresource.Status
		equal bool
	}{
		{nil, nil, true},
		{nil, map[string]*pbresource.Status{}, true},
		{
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			true,
		},
		{
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_FALSE,
							Reason:  "Bar",
							Message: "Foo is false because of Bar",
						},
					},
				},
			},
			false,
		},
		{
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
				"consul.io/other-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			false,
		},
	} {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			require.Equal(t, tc.equal, resource.EqualStatusMap(tc.a, tc.b))
			require.Equal(t, tc.equal, resource.EqualStatusMap(tc.b, tc.a))
		})
	}
}

func BenchmarkEqualType(b *testing.B) {
	// cpu: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz
	// BenchmarkEqualType/ours-16      161532109                7.309 ns/op          0 B/op           0 allocs/op
	// BenchmarkEqualType/reflection-16                 1584954               748.4 ns/op           160 B/op          9 allocs/op
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
	// BenchmarkEqualTenancy/ours-16                   159998534                7.426 ns/op           0 B/op          0 allocs/op
	// BenchmarkEqualTenancy/reflection-16              2283500               550.3 ns/op           128 B/op          7 allocs/op
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
	// BenchmarkEqualID/ours-16                        57818125                21.40 ns/op            0 B/op          0 allocs/op
	// BenchmarkEqualID/reflection-16                   3596365               330.1 ns/op            96 B/op          5 allocs/op
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

func BenchmarkEqualStatus(b *testing.B) {
	// 	cpu: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz
	// BenchmarkEqualStatus/ours-16                    38648232                30.75 ns/op            0 B/op          0 allocs/op
	// BenchmarkEqualStatus/reflection-16                237694              5267 ns/op             944 B/op         51 allocs/op
	statusA := &pbresource.Status{
		ObservedGeneration: ulid.Make().String(),
		Conditions: []*pbresource.Condition{
			{
				Type:    "FooType",
				State:   pbresource.Condition_STATE_TRUE,
				Reason:  "FooReason",
				Message: "Foo is true",
				Resource: &pbresource.Reference{
					Type: &pbresource.Type{
						Group:        "foo-group",
						GroupVersion: "foo-group-version",
						Kind:         "foo-kind",
					},
					Tenancy: &pbresource.Tenancy{
						Partition: "foo-partition",
						PeerName:  "foo-peer-name",
						Namespace: "foo-namespace",
					},
					Name:    "foo-name",
					Section: "foo-section",
				},
			},
		},
	}
	statusB := clone(statusA)
	statusB.Conditions[0].Resource.Section = "bar-section"
	b.ResetTimer()

	b.Run("ours", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resource.EqualStatus(statusA, statusB, true)
		}
	})

	b.Run("reflection", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = proto.Equal(statusA, statusB)
		}
	})
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
