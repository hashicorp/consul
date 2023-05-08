// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Wildcard can be given as Tenancy fields in List and Watch calls, to enumerate
// resources across multiple partitions, peers, namespaces, etc.
const Wildcard = "*"

var (
	// ErrNotFound indicates that the resource could not be found.
	ErrNotFound = errors.New("resource not found")

	// ErrCASFailure indicates that the attempted write failed because the given
	// version does not match what is currently stored.
	ErrCASFailure = errors.New("CAS operation failed because the given version doesn't match what is stored")

	// ErrWrongUid indicates that the attempted write failed because the resource's
	// Uid doesn't match what is currently stored (e.g. the caller is trying to
	// operate on a deleted resource with the same name).
	ErrWrongUid = errors.New("write failed because the given uid doesn't match what is stored")

	// ErrInconsistent indicates that the attempted write or consistent read could
	// not be achieved because of a consistency or availability issue (e.g. loss of
	// quorum, or when interacting with a Raft follower).
	ErrInconsistent = errors.New("cannot satisfy consistency requirements")

	// ErrWatchClosed is returned by Watch.Next when the watch is closed, e.g. when
	// a snapshot is restored and the watch's events are no longer valid. Consumers
	// should discard any materialized state and start a new watch.
	ErrWatchClosed = errors.New("watch closed")
)

// ReadConsistency is used to specify the required consistency guarantees for
// a read operation.
type ReadConsistency int

const (
	// EventualConsistency provides a weak set of guarantees, but is much cheaper
	// than using StrongConsistency and therefore should be treated as the default.
	//
	// It guarantees [monotonic reads]. That is, a read will always return results
	// that are as up-to-date as an earlier read, provided both happen on the same
	// Consul server. But does not make any such guarantee about writes.
	//
	// In other words, reads won't necessarily reflect earlier writes, even when
	// made against the same server.
	//
	// Operations that don't allow the caller to specify the consistency mode will
	// hold the same guarantees as EventualConsistency, but check the method docs
	// for caveats.
	//
	// [monotonic reads]: https://jepsen.io/consistency/models/monotonic-reads
	EventualConsistency ReadConsistency = iota

	// StrongConsistency provides a very strong set of guarantees but is much more
	// expensive, so should be used sparingly.
	//
	// It guarantees full [linearizability], such that a read will always return
	// the most up-to-date version of a resource, without caveat.
	//
	// [linearizability]: https://jepsen.io/consistency/models/linearizable
	StrongConsistency
)

// String implements the fmt.Stringer interface.
func (c ReadConsistency) String() string {
	switch c {
	case EventualConsistency:
		return "Eventual Consistency"
	case StrongConsistency:
		return "Strong Consistency"
	}
	panic(fmt.Sprintf("unknown ReadConsistency (%d)", c))
}

// Backend provides the low-level storage substrate for resources. It can be
// implemented using internal (i.e. Raft+MemDB) or external (e.g. DynamoDB)
// storage systems.
//
// Refer to the method comments for details of the behaviors and invariants
// provided, which are also verified by the conformance test suite in the
// internal/storage/conformance package.
//
// Cross-cutting concerns:
//
// # UIDs
//
// Users identify resources with a name of their choice (e.g. service "billing")
// but internally, we add our own identifier in the Uid field to disambiguate
// references when resources are deleted and re-created with the same name.
//
// # GroupVersion
//
// In order to support automatic translation between schema versions, we only
// store a single version of a resource, and treat types with the same Group
// and Kind, but different GroupVersions, as equivalent.
//
// # Read-Modify-Write Patterns
//
// All writes at the storage backend level are CAS (Compare-And-Swap) operations
// where the caller must provide the resource in its entirety, with the current
// version string.
//
// Non-CAS writes should be implemented at a higher level (i.e. in the Resource
// Service) by reading the resource, applying the user's requested modifications,
// and writing it back. This allows us to ensure we're correctly carrying over
// the resource's Status and Uid, without requiring support for partial update
// or "patch" operations from external storage systems.
//
// In cases where there are concurrent interleaving writes made to a resource,
// it's likely that a CAS operation will fail, so callers may need to put their
// Read-Modify-Write cycle in a retry loop.
type Backend interface {
	// Read a resource using its ID.
	//
	// # UIDs
	//
	// If id.Uid is empty, Read will ignore it and return whatever resource is
	// stored with the given name. This is the desired behavior for user-initiated
	// reads.
	//
	// If id.Uid is non-empty, Read will only return a resource if its Uid matches,
	// otherwise it'll return ErrNotFound. This is the desired behaviour for reads
	// initiated by controllers, which tend to operate on a specific lifetime of a
	// resource.
	//
	// See Backend docs for more details.
	//
	// # GroupVersion
	//
	// If id.Type.GroupVersion doesn't match what is stored, Read will return a
	// GroupVersionMismatchError which contains a pointer to the stored resource.
	//
	// See Backend docs for more details.
	//
	// # Consistency
	//
	// Read supports both EventualConsistency and StrongConsistency.
	Read(ctx context.Context, consistency ReadConsistency, id *pbresource.ID) (*pbresource.Resource, error)

	// WriteCAS performs an atomic CAS (Compare-And-Swap) write of a resource based
	// on its version. The given version will be compared to what is stored, and if
	// it does not match, ErrCASFailure will be returned. To create new resources,
	// set version to an empty string.
	//
	// If a write cannot be performed because of a consistency or availability
	// issue (e.g. when interacting with a Raft follower, or when quorum is lost)
	// ErrInconsistent will be returned.
	//
	// # UIDs
	//
	// UIDs are immutable, so if the given resource's Uid field doesn't match what
	// is stored, ErrWrongUid will be returned.
	//
	// See Backend docs for more details.
	//
	// # GroupVersion
	//
	// Write does not validate the GroupVersion and allows you to overwrite a
	// resource stored in an older form with a newer, and vice versa.
	//
	// See Backend docs for more details.
	WriteCAS(ctx context.Context, res *pbresource.Resource) (*pbresource.Resource, error)

	// DeleteCAS performs an atomic CAS (Compare-And-Swap) deletion of a resource
	// based on its version. The given version will be compared to what is stored,
	// and if it does not match, ErrCASFailure will be returned.
	//
	// If the resource does not exist (i.e. has already been deleted) no error will
	// be returned.
	//
	// If a deletion cannot be performed because of a consistency or availability
	// issue (e.g. when interacting with a Raft follower, or when quorum is lost)
	// ErrInconsistent will be returned.
	//
	// # UIDs
	//
	// If the given id's Uid does not match what is stored, the deletion will be a
	// no-op (i.e. it is considered to be a different resource).
	//
	// See Backend docs for more details.
	//
	// # GroupVersion
	//
	// Delete does not check or refer to the GroupVersion. Resources of the same
	// Group and Kind are considered equivalent, so requests to delete a resource
	// using a new GroupVersion will delete a resource even if it's stored with an
	// old GroupVersion.
	//
	// See Backend docs for more details.
	DeleteCAS(ctx context.Context, id *pbresource.ID, version string) error

	// List resources of the given type, tenancy, and optionally matching the given
	// name prefix.
	//
	// # Tenancy Wildcard
	//
	// In order to list resources across multiple tenancy units (e.g. namespaces)
	// pass the Wildcard sentinel value in tenancy fields.
	//
	// # GroupVersion
	//
	// The resType argument contains only the Group and Kind, to reflect the fact
	// that resources may be stored in a mix of old and new forms. As such, it's
	// the caller's responsibility to check the resource's GroupVersion and
	// translate or filter accordingly.
	//
	// # Consistency
	//
	// Generally, List only supports EventualConsistency. However, for backward
	// compatability with our v1 APIs, the Raft backend supports StrongConsistency
	// for list operations.
	//
	// When the v1 APIs finally goes away, so will this consistency parameter, so
	// it should not be depended on outside of the backward compatability layer.
	List(ctx context.Context, consistency ReadConsistency, resType UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error)

	// WatchList watches resources of the given type, tenancy, and optionally
	// matching the given name prefix. Upsert events for the current state of the
	// world (i.e. existing resources that match the given filters) will be emitted
	// immediately, and will be followed by delta events whenever resources are
	// written or deleted.
	//
	// # Consistency
	//
	// WatchList makes no guarantees about event timeliness (e.g. an event for a
	// write may not be received immediately), but it does guarantee that events
	// will be emitted in the correct order.
	//
	// There's also a guarantee of [monotonic reads] between Read and WatchList,
	// such that Read will never return data that is older than the most recent
	// event you received. Note: this guarantee holds at the (in-process) storage
	// backend level, only. Controllers and other users of the Resource Service API
	// must remain connected to the same Consul server process to avoid receiving
	// events about writes that they then cannot read. In other words, it is *not*
	// linearizable.
	//
	// There's a similar guarantee between WatchList and ListByOwner, see the
	// ListByOwner docs for more information.
	//
	// See List docs for details about Tenancy Wildcard and GroupVersion.
	//
	// [monotonic reads]: https://jepsen.io/consistency/models/monotonic-reads
	WatchList(ctx context.Context, resType UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) (Watch, error)

	// ListByOwner returns resources owned by the resource with the given ID. It
	// is typically used to implement cascading deletion.
	//
	// # Consistency
	//
	// ListByOwner may return stale results, but guarantees [monotonic reads]
	// with events received from WatchList. In practice, this means that if you
	// learn that a resource has been deleted through a watch event, the results
	// you receive from ListByOwner will represent all references that existed
	// at the time the owner was deleted. It doesn't make any guarantees about
	// references that are created *after* the owner was deleted, though, so you
	// must either prevent that from happening (e.g. by performing a consistent
	// read of the owner in the write-path, which has its own ordering/correctness
	// challenges), or by calling ListByOwner after the expected window of
	// inconsistency (e.g. deferring cascading deletion, or doing a second pass
	// an hour later).
	//
	// [montonic reads]: https://jepsen.io/consistency/models/monotonic-reads
	ListByOwner(ctx context.Context, id *pbresource.ID) ([]*pbresource.Resource, error)
}

// Watch represents a watch on a given set of resources. Call Next to get the
// next event (i.e. upsert or deletion) and Close when you're done watching.
type Watch interface {
	// Next returns the next event (i.e. upsert or deletion)
	Next(ctx context.Context) (*pbresource.WatchEvent, error)

	// Close the watch and free its associated resources.
	Close()
}

// UnversionedType represents a pbresource.Type as it is stored without the
// GroupVersion.
type UnversionedType struct {
	Group string
	Kind  string
}

// UnversionedTypeFrom creates an UnversionedType from the given *pbresource.Type.
func UnversionedTypeFrom(t *pbresource.Type) UnversionedType {
	return UnversionedType{
		Group: t.Group,
		Kind:  t.Kind,
	}
}

// GroupVersionMismatchError is returned when a resource is stored as a type
// with a different GroupVersion than was requested.
type GroupVersionMismatchError struct {
	// RequestedType is the type that was requested.
	RequestedType *pbresource.Type

	// Stored is the resource as it is stored.
	Stored *pbresource.Resource
}

// Error implements the error interface.
func (e GroupVersionMismatchError) Error() string {
	return fmt.Sprintf(
		"resource was requested with GroupVersion=%q, but stored with GroupVersion=%q",
		e.RequestedType.GroupVersion,
		e.Stored.Id.Type.GroupVersion,
	)
}
