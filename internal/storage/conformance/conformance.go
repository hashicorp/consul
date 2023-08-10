// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type TestOptions struct {
	// NewBackend will be called to construct a storage.Backend to run the tests
	// against.
	NewBackend func(t *testing.T) storage.Backend

	// SupportsStronglyConsistentList indicates whether the given storage backend
	// supports strongly consistent list operations.
	SupportsStronglyConsistentList bool
}

// Test runs a suite of tests against a storage.Backend implementation to check
// it correctly implements our required behaviours.
func Test(t *testing.T, opts TestOptions) {
	require.NotNil(t, opts.NewBackend, "NewBackend method is required")

	t.Run("Read", func(t *testing.T) { testRead(t, opts) })
	t.Run("CAS Write", func(t *testing.T) { testCASWrite(t, opts) })
	t.Run("CAS Delete", func(t *testing.T) { testCASDelete(t, opts) })
	t.Run("ListByOwner", func(t *testing.T) { testListByOwner(t, opts) })

	testListWatch(t, opts)
}

func testRead(t *testing.T, opts TestOptions) {
	ctx := testContext(t)

	for consistency, check := range map[storage.ReadConsistency]consistencyChecker{
		storage.EventualConsistency: eventually,
		storage.StrongConsistency:   immediately,
	} {
		t.Run(consistency.String(), func(t *testing.T) {
			res := &pbresource.Resource{
				Id: &pbresource.ID{
					Type:    typeAv1,
					Tenancy: tenancyDefault,
					Name:    "web",
					Uid:     "a",
				},
			}

			t.Run("simple", func(t *testing.T) {
				backend := opts.NewBackend(t)

				_, err := backend.WriteCAS(ctx, res)
				require.NoError(t, err)

				check(t, func(t testingT) {
					output, err := backend.Read(ctx, consistency, res.Id)
					require.NoError(t, err)
					prototest.AssertDeepEqual(t, res, output, ignoreVersion)
				})
			})

			t.Run("no uid", func(t *testing.T) {
				backend := opts.NewBackend(t)

				_, err := backend.WriteCAS(ctx, res)
				require.NoError(t, err)

				id := clone(res.Id)
				id.Uid = ""

				check(t, func(t testingT) {
					output, err := backend.Read(ctx, consistency, id)
					require.NoError(t, err)
					prototest.AssertDeepEqual(t, res, output, ignoreVersion)
				})
			})

			t.Run("different id", func(t *testing.T) {
				backend := opts.NewBackend(t)

				_, err := backend.WriteCAS(ctx, res)
				require.NoError(t, err)

				id := clone(res.Id)
				id.Name = "different"

				check(t, func(t testingT) {
					_, err := backend.Read(ctx, consistency, id)
					require.ErrorIs(t, err, storage.ErrNotFound)
				})
			})

			t.Run("different uid", func(t *testing.T) {
				backend := opts.NewBackend(t)

				_, err := backend.WriteCAS(ctx, res)
				require.NoError(t, err)

				id := clone(res.Id)
				id.Uid = "b"

				check(t, func(t testingT) {
					_, err := backend.Read(ctx, consistency, id)
					require.ErrorIs(t, err, storage.ErrNotFound)
				})
			})

			t.Run("different GroupVersion", func(t *testing.T) {
				backend := opts.NewBackend(t)

				_, err := backend.WriteCAS(ctx, res)
				require.NoError(t, err)

				id := clone(res.Id)
				id.Type = typeAv2

				check(t, func(t testingT) {
					_, err := backend.Read(ctx, consistency, id)
					require.Error(t, err)

					var e storage.GroupVersionMismatchError
					if errors.As(err, &e) {
						prototest.AssertDeepEqual(t, id.Type, e.RequestedType)
						prototest.AssertDeepEqual(t, res, e.Stored, ignoreVersion)
					} else {
						t.Fatalf("expected storage.GroupVersionMismatchError, got: %T", err)
					}
				})
			})
		})
	}

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
		}

		v1.Version = "some-version"
		_, err := backend.WriteCAS(ctx, v1)
		require.ErrorIs(t, err, storage.ErrCASFailure)

		v1.Version = ""
		v1, err = backend.WriteCAS(ctx, v1)
		require.NoError(t, err)
		require.NotEmpty(t, v1.Version)

		v2, err := backend.WriteCAS(ctx, v1)
		require.NoError(t, err)
		require.NotEmpty(t, v2.Version)
		require.NotEqual(t, v1.Version, v2.Version)

		v3 := clone(v2)
		v3.Version = ""
		_, err = backend.WriteCAS(ctx, v3)
		require.ErrorIs(t, err, storage.ErrCASFailure)

		v3.Version = v1.Version
		_, err = backend.WriteCAS(ctx, v3)
		require.ErrorIs(t, err, storage.ErrCASFailure)
	})

	t.Run("uid immutability", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		v1, err := backend.WriteCAS(ctx, &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeB,
				Tenancy: tenancyDefault,
				Name:    "web",
				Uid:     "a",
			},
		})
		require.NoError(t, err)

		// Uid cannot change.
		v2 := clone(v1)
		v2.Id.Uid = ""
		_, err = backend.WriteCAS(ctx, v2)
		require.Error(t, err)

		v2.Id.Uid = "b"
		_, err = backend.WriteCAS(ctx, v2)
		require.ErrorIs(t, err, storage.ErrWrongUid)

		v2.Id.Uid = v1.Id.Uid
		v2, err = backend.WriteCAS(ctx, v2)
		require.NoError(t, err)

		// Uid can change after original resource is deleted.
		require.NoError(t, backend.DeleteCAS(ctx, v2.Id, v2.Version))

		v3 := clone(v2)
		v3.Id.Uid = "b"
		v3.Version = ""

		_, err = backend.WriteCAS(ctx, v3)
		require.NoError(t, err)
	})
}

func testCASDelete(t *testing.T, opts TestOptions) {
	t.Run("version-based CAS", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		res, err := backend.WriteCAS(ctx, &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeB,
				Tenancy: tenancyDefault,
				Name:    "web",
				Uid:     "a",
			},
		})
		require.NoError(t, err)

		require.ErrorIs(t, backend.DeleteCAS(ctx, res.Id, ""), storage.ErrCASFailure)
		require.ErrorIs(t, backend.DeleteCAS(ctx, res.Id, "some-version"), storage.ErrCASFailure)

		require.NoError(t, backend.DeleteCAS(ctx, res.Id, res.Version))

		eventually(t, func(t testingT) {
			_, err = backend.Read(ctx, storage.EventualConsistency, res.Id)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})

	t.Run("uid must match", func(t *testing.T) {
		backend := opts.NewBackend(t)
		ctx := testContext(t)

		res, err := backend.WriteCAS(ctx, &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    typeB,
				Tenancy: tenancyDefault,
				Name:    "web",
				Uid:     "a",
			},
		})
		require.NoError(t, err)

		id := clone(res.Id)
		id.Uid = "b"
		require.NoError(t, backend.DeleteCAS(ctx, id, res.Version))

		eventually(t, func(t testingT) {
			_, err = backend.Read(ctx, storage.EventualConsistency, res.Id)
			require.NoError(t, err)
		})
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
		ctx := testContext(t)

		consistencyModes := map[storage.ReadConsistency]consistencyChecker{
			storage.EventualConsistency: eventually,
		}
		if opts.SupportsStronglyConsistentList {
			consistencyModes[storage.StrongConsistency] = immediately
		}

		for consistency, check := range consistencyModes {
			t.Run(consistency.String(), func(t *testing.T) {
				for desc, tc := range testCases {
					t.Run(desc, func(t *testing.T) {
						backend := opts.NewBackend(t)
						for _, r := range seedData {
							_, err := backend.WriteCAS(ctx, r)
							require.NoError(t, err)
						}

						check(t, func(t testingT) {
							res, err := backend.List(ctx, consistency, tc.resourceType, tc.tenancy, tc.namePrefix)
							require.NoError(t, err)
							prototest.AssertElementsMatch(t, res, tc.results, ignoreVersion)
						})
					})
				}
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
					_, err := backend.WriteCAS(ctx, r)
					require.NoError(t, err)
				}

				watch, err := backend.WatchList(ctx, tc.resourceType, tc.tenancy, tc.namePrefix)
				require.NoError(t, err)
				t.Cleanup(watch.Close)

				for i := 0; i < len(tc.results); i++ {
					ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					t.Cleanup(cancel)

					event, err := watch.Next(ctx)
					require.NoError(t, err)

					require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, event.Operation)
					prototest.AssertContainsElement(t, tc.results, event.Resource, ignoreVersion)
				}
			})

			t.Run(fmt.Sprintf("%s - following events", desc), func(t *testing.T) {
				backend := opts.NewBackend(t)
				ctx := testContext(t)

				watch, err := backend.WatchList(ctx, tc.resourceType, tc.tenancy, tc.namePrefix)
				require.NoError(t, err)
				t.Cleanup(watch.Close)

				// Write the seed data after the watch has been established.
				for _, r := range seedData {
					_, err := backend.WriteCAS(ctx, r)
					require.NoError(t, err)
				}

				for i := 0; i < len(tc.results); i++ {
					ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					t.Cleanup(cancel)

					event, err := watch.Next(ctx)
					require.NoError(t, err)

					require.Equal(t, pbresource.WatchEvent_OPERATION_UPSERT, event.Operation)
					prototest.AssertContainsElement(t, tc.results, event.Resource, ignoreVersion)

					// Check that Read implements "monotonic reads" with Watch.
					readRes, err := backend.Read(ctx, storage.EventualConsistency, event.Resource.Id)
					require.NoError(t, err)
					prototest.AssertDeepEqual(t, event.Resource, readRes)
				}

				// Delete a random resource to check we get an event.
				del, err := backend.Read(ctx, storage.EventualConsistency, tc.results[rand.Intn(len(tc.results))].Id)
				require.NoError(t, err)
				require.NoError(t, backend.DeleteCAS(ctx, del.Id, del.Version))

				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				t.Cleanup(cancel)

				event, err := watch.Next(ctx)
				require.NoError(t, err)

				require.Equal(t, pbresource.WatchEvent_OPERATION_DELETE, event.Operation)
				prototest.AssertDeepEqual(t, del, event.Resource)

				// Check that Read implements "monotonic reads" with Watch.
				_, err = backend.Read(ctx, storage.EventualConsistency, del.Id)
				require.ErrorIs(t, err, storage.ErrNotFound)
			})
		}
	})
}

func testListByOwner(t *testing.T, opts TestOptions) {
	backend := opts.NewBackend(t)
	ctx := testContext(t)

	owner, err := backend.WriteCAS(ctx, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typeAv1,
			Tenancy: tenancyDefault,
			Name:    "owner",
			Uid:     "a",
		},
	})
	require.NoError(t, err)

	r1, err := backend.WriteCAS(ctx, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typeB,
			Tenancy: tenancyDefault,
			Name:    "r1",
			Uid:     "a",
		},
		Owner: owner.Id,
	})
	require.NoError(t, err)

	r2, err := backend.WriteCAS(ctx, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typeAv2,
			Tenancy: tenancyDefault,
			Name:    "r2",
			Uid:     "a",
		},
		Owner: owner.Id,
	})
	require.NoError(t, err)

	eventually(t, func(t testingT) {
		res, err := backend.ListByOwner(ctx, owner.Id)
		require.NoError(t, err)
		prototest.AssertElementsMatch(t, res, []*pbresource.Resource{r1, r2})
	})

	t.Run("references are anchored to a specific uid", func(t *testing.T) {
		id := clone(owner.Id)
		id.Uid = "different"

		eventually(t, func(t testingT) {
			res, err := backend.ListByOwner(ctx, id)
			require.NoError(t, err)
			require.Empty(t, res)
		})
	})

	t.Run("deleting the owner doesn't remove the references", func(t *testing.T) {
		require.NoError(t, backend.DeleteCAS(ctx, owner.Id, owner.Version))

		eventually(t, func(t testingT) {
			res, err := backend.ListByOwner(ctx, owner.Id)
			require.NoError(t, err)
			prototest.AssertElementsMatch(t, res, []*pbresource.Resource{r1, r2})
		})
	})

	t.Run("deleting the owned resource removes its reference", func(t *testing.T) {
		require.NoError(t, backend.DeleteCAS(ctx, r2.Id, r2.Version))

		eventually(t, func(t testingT) {
			res, err := backend.ListByOwner(ctx, owner.Id)
			require.NoError(t, err)
			prototest.AssertElementsMatch(t, res, []*pbresource.Resource{r1})
		})
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

	ignoreVersion = protocmp.IgnoreFields(&pbresource.Resource{}, "version")
)

func resource(typ *pbresource.Type, ten *pbresource.Tenancy, name string) *pbresource.Resource {
	return &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    typ,
			Tenancy: ten,
			Name:    name,
			Uid:     "a",
		},
	}
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }

type testingT interface {
	require.TestingT
	prototest.TestingT
}

type consistencyChecker func(t *testing.T, fn func(testingT))

func eventually(t *testing.T, fn func(testingT)) {
	t.Helper()
	retry.Run(t, func(r *retry.R) { fn(r) })
}

func immediately(t *testing.T, fn func(testingT)) {
	t.Helper()
	fn(t)
}
