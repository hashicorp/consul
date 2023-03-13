package conformance

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TestOptions struct {
	// NewBackend will be called to construct a storage.Backend to run the tests
	// against.
	NewBackend func(t *testing.T) storage.Backend
}

// Test runs a suite of tests against a storage.Backend implementation to check
// it correctly implements our required behaviours.
//
// Note: it currently checks for stronger consistency than we actually need. We
// will need to handle eventual consistency when we implement the Raft backend.
func Test(t *testing.T, opts TestOptions) {
	require.NotNil(t, opts.NewBackend, "NewBackend method is required")

	t.Run("Read", func(t *testing.T) { testRead(t, opts) })
	t.Run("CAS Write", func(t *testing.T) { testCASWrite(t, opts) })
	t.Run("CAS Delete", func(t *testing.T) { testCASDelete(t, opts) })
	t.Run("OwnerReferences", func(t *testing.T) { testOwnerReferences(t, opts) })

	testListWatch(t, opts)
}

func testRead(t *testing.T, opts TestOptions) {
	ctx := testContext(t)
	backend := opts.NewBackend(t)

	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typeAv1,
			Tenancy: tenancyDefault,
			Name:    "web",
			Uid:     "a",
		},
		Version: "1",
	}
	_, err := backend.WriteCAS(ctx, res, "")
	require.NoError(t, err)

	t.Run("simple", func(t *testing.T) {
		output, err := backend.Read(ctx, res.Id)
		require.NoError(t, err)
		require.Equal(t, res, output)
	})

	t.Run("no uid", func(t *testing.T) {
		id := clone(res.Id)
		id.Uid = ""

		output, err := backend.Read(ctx, id)
		require.NoError(t, err)
		require.Equal(t, res, output)
	})

	t.Run("different id", func(t *testing.T) {
		id := clone(res.Id)
		id.Name = "different"

		_, err := backend.Read(ctx, id)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("different uid", func(t *testing.T) {
		id := clone(res.Id)
		id.Uid = "b"

		_, err := backend.Read(ctx, id)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("different GroupVersion", func(t *testing.T) {
		id := clone(res.Id)
		id.Type = typeAv2

		_, err := backend.Read(ctx, id)
		require.Error(t, err)

		var e storage.GroupVersionMismatchError
		if errors.As(err, &e) {
			require.Equal(t, id.Type, e.RequestedType)
			require.Equal(t, res, e.Stored)
		} else {
			t.Fatalf("expected storage.GroupVersionMismatchError, got: %T", err)
		}
	})
}

func testCASWrite(t *testing.T, opts TestOptions) {
	t.Run("version-based CAS", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		v1 := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeB,
				Tenancy: tenancyDefault,
				Name:    "web",
				Uid:     "a",
			},
			Version: "1",
		}

		_, err := backend.WriteCAS(ctx, v1, "some-version")
		require.ErrorIs(t, err, storage.ErrConflict)

		_, err = backend.WriteCAS(ctx, v1, "")
		require.NoError(t, err)

		v2 := clone(v1)
		v2.Version = "2"

		_, err = backend.WriteCAS(ctx, v2, v1.Version)
		require.NoError(t, err)

		v3 := clone(v2)
		v3.Version = "3"

		_, err = backend.WriteCAS(ctx, v3, "")
		require.ErrorIs(t, err, storage.ErrConflict)

		_, err = backend.WriteCAS(ctx, v3, v1.Version)
		require.ErrorIs(t, err, storage.ErrConflict)
	})

	t.Run("uid immutability", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		v1 := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeB,
				Tenancy: tenancyDefault,
				Name:    "web",
				Uid:     "a",
			},
			Version: "1",
		}
		_, err := backend.WriteCAS(ctx, v1, "")
		require.NoError(t, err)

		// Uid cannot change.
		v2 := clone(v1)
		v2.Version = "2"

		v2.Id.Uid = ""
		_, err = backend.WriteCAS(ctx, v2, v1.Version)
		require.Error(t, err)

		v2.Id.Uid = "b"
		_, err = backend.WriteCAS(ctx, v2, v1.Version)
		require.ErrorIs(t, err, storage.ErrConflict)

		v2.Id.Uid = v1.Id.Uid
		_, err = backend.WriteCAS(ctx, v2, v1.Version)
		require.NoError(t, err)

		// Uid can change after original resource is deleted.
		require.NoError(t, backend.DeleteCAS(ctx, v2.Id, v2.Version))

		v3 := clone(v2)
		v3.Version = "3"
		v3.Id.Uid = "b"

		_, err = backend.WriteCAS(ctx, v2, "")
		require.NoError(t, err)
	})
}

func testCASDelete(t *testing.T, opts TestOptions) {
	t.Run("version-based CAS", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		res := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeB,
				Tenancy: tenancyDefault,
				Name:    "web",
				Uid:     "a",
			},
			Version: "1",
		}
		_, err := backend.WriteCAS(ctx, res, "")
		require.NoError(t, err)

		require.ErrorIs(t, backend.DeleteCAS(ctx, res.Id, ""), storage.ErrConflict)
		require.ErrorIs(t, backend.DeleteCAS(ctx, res.Id, "2"), storage.ErrConflict)

		require.NoError(t, backend.DeleteCAS(ctx, res.Id, res.Version))

		_, err = backend.Read(ctx, res.Id)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("uid must match", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		res := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeB,
				Tenancy: tenancyDefault,
				Name:    "web",
				Uid:     "a",
			},
			Version: "1",
		}
		_, err := backend.WriteCAS(ctx, res, "")
		require.NoError(t, err)

		id := clone(res.Id)
		id.Uid = "b"
		require.NoError(t, backend.DeleteCAS(ctx, id, res.Version))

		_, err = backend.Read(ctx, res.Id)
		require.NoError(t, err)
	})
}

func testListWatch(t *testing.T, opts TestOptions) {
	testCases := map[string]struct {
		resourceType storage.UnversionedType
		tenancy      *pbresource.Tenancy
		namePrefix   string
		results      []*pbresource.Resource
	}{
		"simple #1": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy:      tenancyDefault,
			namePrefix:   "",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
				seedData[2],
			},
		},
		"simple #2": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy:      tenancyOther,
			namePrefix:   "",
			results: []*pbresource.Resource{
				seedData[3],
			},
		},
		"fixed tenancy, name prefix": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy:      tenancyDefault,
			namePrefix:   "a",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
			},
		},
		"wildcard tenancy": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy: &pbresource.Tenancy{
				Partition: storage.Wildcard,
				PeerName:  storage.Wildcard,
				Namespace: storage.Wildcard,
			},
			namePrefix: "",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
				seedData[2],
				seedData[3],
				seedData[5],
				seedData[6],
			},
		},
		"fixed partition, wildcard peer, wildcard namespace": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy: &pbresource.Tenancy{
				Partition: "default",
				PeerName:  storage.Wildcard,
				Namespace: storage.Wildcard,
			},
			namePrefix: "",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
				seedData[2],
				seedData[5],
				seedData[6],
			},
		},
		"wildcard partition, fixed peer, wildcard namespace": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy: &pbresource.Tenancy{
				Partition: storage.Wildcard,
				PeerName:  "local",
				Namespace: storage.Wildcard,
			},
			namePrefix: "",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
				seedData[2],
				seedData[3],
				seedData[5],
			},
		},
		"wildcard partition, wildcard peer, fixed namespace": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy: &pbresource.Tenancy{
				Partition: storage.Wildcard,
				PeerName:  storage.Wildcard,
				Namespace: "default",
			},
			namePrefix: "",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
				seedData[2],
				seedData[6],
			},
		},
		"fixed partition, fixed peer, wildcard namespace": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy: &pbresource.Tenancy{
				Partition: "default",
				PeerName:  "local",
				Namespace: storage.Wildcard,
			},
			namePrefix: "",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
				seedData[2],
				seedData[5],
			},
		},
		"wildcard tenancy, name prefix": {
			resourceType: storage.UnversionedTypeFrom(typeAv1),
			tenancy: &pbresource.Tenancy{
				Partition: storage.Wildcard,
				PeerName:  storage.Wildcard,
				Namespace: storage.Wildcard,
			},
			namePrefix: "a",
			results: []*pbresource.Resource{
				seedData[0],
				seedData[1],
				seedData[3],
				seedData[5],
				seedData[6],
			},
		},
	}

	t.Run("List", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		for _, r := range seedData {
			_, err := backend.WriteCAS(ctx, r, "")
			require.NoError(t, err)
		}

		for desc, tc := range testCases {
			t.Run(desc, func(t *testing.T) {
				res, err := backend.List(ctx, tc.resourceType, tc.tenancy, tc.namePrefix)
				require.NoError(t, err)
				require.ElementsMatch(t, res, tc.results)
			})
		}
	})

	t.Run("WatchList", func(t *testing.T) {
		for desc, tc := range testCases {
			t.Run(fmt.Sprintf("%s - initial snapshot", desc), func(t *testing.T) {
				backend := opts.NewBackend(t)
				ctx := testContext(t)

				// Write the seed data before the watch has been established.
				for _, r := range seedData {
					_, err := backend.WriteCAS(ctx, r, "")
					require.NoError(t, err)
				}

				watch, err := backend.WatchList(ctx, tc.resourceType, tc.tenancy, tc.namePrefix)
				require.NoError(t, err)

				for i := 0; i < len(tc.results); i++ {
					ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					t.Cleanup(cancel)

					event, err := watch.Next(ctx)
					require.NoError(t, err)

					require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, event.Operation)
					require.Containsf(t, tc.results, event.Resource, "resource not in expected results: %s", event.Resource.Id)
				}
			})

			t.Run(fmt.Sprintf("%s - following events", desc), func(t *testing.T) {
				backend := opts.NewBackend(t)
				ctx := testContext(t)

				watch, err := backend.WatchList(ctx, tc.resourceType, tc.tenancy, tc.namePrefix)
				require.NoError(t, err)

				// Write the seed data after the watch has been established.
				for _, r := range seedData {
					_, err := backend.WriteCAS(ctx, r, "")
					require.NoError(t, err)
				}

				for i := 0; i < len(tc.results); i++ {
					ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					t.Cleanup(cancel)

					event, err := watch.Next(ctx)
					require.NoError(t, err)

					require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, event.Operation)
					require.Containsf(t, tc.results, event.Resource, "resource not in expected results: %s", event.Resource.Id)
				}

				// Delete a random resource to check we get an event.
				del := tc.results[rand.Intn(len(tc.results))]
				require.NoError(t, backend.DeleteCAS(ctx, del.Id, del.Version))

				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				t.Cleanup(cancel)

				event, err := watch.Next(ctx)
				require.NoError(t, err)

				require.Equal(t, pbresource.WatchEvent_OPERATION_DELETE, event.Operation)
				require.Equal(t, del, event.Resource)
			})
		}
	})
}

func testOwnerReferences(t *testing.T, opts TestOptions) {
	backend := opts.NewBackend(t)
	ctx := testContext(t)

	owner := &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typeAv1,
			Tenancy: tenancyDefault,
			Name:    "owner",
			Uid:     "a",
		},
		Version: "1",
	}
	_, err := backend.WriteCAS(ctx, owner, "")
	require.NoError(t, err)

	r1 := &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typeB,
			Tenancy: tenancyDefault,
			Name:    "r1",
			Uid:     "a",
		},
		Owner:   owner.Id,
		Version: "1",
	}
	_, err = backend.WriteCAS(ctx, r1, "")
	require.NoError(t, err)

	r2 := &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typeAv2,
			Tenancy: tenancyDefault,
			Name:    "r2",
			Uid:     "a",
		},
		Owner:   owner.Id,
		Version: "1",
	}
	_, err = backend.WriteCAS(ctx, r2, "")
	require.NoError(t, err)

	refs, err := backend.OwnerReferences(ctx, owner.Id)
	require.NoError(t, err)
	require.ElementsMatch(t, refs, []*pbresource.ID{r1.Id, r2.Id})

	t.Run("references are anchored to a specific uid", func(t *testing.T) {
		id := clone(owner.Id)
		id.Uid = "different"

		refs, err := backend.OwnerReferences(ctx, id)
		require.NoError(t, err)
		require.Empty(t, refs)
	})

	t.Run("deleting the owner doesn't remove the references", func(t *testing.T) {
		require.NoError(t, backend.DeleteCAS(ctx, owner.Id, owner.Version))

		refs, err = backend.OwnerReferences(ctx, owner.Id)
		require.NoError(t, err)
		require.ElementsMatch(t, refs, []*pbresource.ID{r1.Id, r2.Id})
	})

	t.Run("deleting the owned resource removes its reference", func(t *testing.T) {
		require.NoError(t, backend.DeleteCAS(ctx, r2.Id, r2.Version))

		refs, err = backend.OwnerReferences(ctx, owner.Id)
		require.NoError(t, err)
		require.ElementsMatch(t, refs, []*pbresource.ID{r1.Id})
	})
}

var (
	typeAv1 = &pbresource.Type{
		Group:        "test",
		GroupVersion: "v1",
		Kind:         "a",
	}
	typeAv2 = &pbresource.Type{
		Group:        "test",
		GroupVersion: "v2",
		Kind:         "a",
	}
	typeB = &pbresource.Type{
		Group:        "test",
		GroupVersion: "v1",
		Kind:         "b",
	}
	tenancyDefault = &pbresource.Tenancy{
		Partition: "default",
		PeerName:  "local",
		Namespace: "default",
	}

	tenancyDefaultOtherNamespace = &pbresource.Tenancy{
		Partition: "default",
		PeerName:  "local",
		Namespace: "other",
	}
	tenancyDefaultOtherPeer = &pbresource.Tenancy{
		Partition: "default",
		PeerName:  "remote",
		Namespace: "default",
	}
	tenancyOther = &pbresource.Tenancy{
		Partition: "billing",
		PeerName:  "local",
		Namespace: "payments",
	}

	seedData = []*pbresource.Resource{
		resource(typeAv1, tenancyDefault, "admin"),                    // 0
		resource(typeAv1, tenancyDefault, "api"),                      // 1
		resource(typeAv2, tenancyDefault, "web"),                      // 2
		resource(typeAv1, tenancyOther, "api"),                        // 3
		resource(typeB, tenancyDefault, "admin"),                      // 4
		resource(typeAv1, tenancyDefaultOtherNamespace, "autoscaler"), // 5
		resource(typeAv1, tenancyDefaultOtherPeer, "amplifier"),       // 6
	}
)

func resource(typ *pbresource.Type, ten *pbresource.Tenancy, name string) *pbresource.Resource {
	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typ,
			Tenancy: ten,
			Name:    name,
			Uid:     "a",
		},
		Version: "1",
	}
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
