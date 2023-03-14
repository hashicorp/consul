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

	// ErrConflict indicates that the attempted write failed because of a version
	// or UID mismatch.
	ErrConflict = errors.New("operation failed because of a Version or Uid mismatch")

	// ErrInconsistent indicates that the attempted write or consistent read could
	// not be achieved because of a consistency or availability issue (e.g. loss of
	// quorum, or when interacting with a Raft follower).
	ErrInconsistent = errors.New("cannot satisfy consistency requirements")
)

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
	// Read makes no guarantees about consistency, and may return stale results.
	// For stronger guarantees, use ReadConsistent.
	Read(ctx context.Context, id *pbresource.ID) (*pbresource.Resource, error)

	// ReadConsistent provides the same functionality as Read, but guarantees
	// single-resource sequential consistency, typically by bypassing any caches
	// and proxying the request directly to the underlying storage system.
	//
	// If a consistent read cannot be achieved (e.g. when interacting with a Raft
	// follower, or quorum is lost) ErrInconsistent will be returned.
	//
	// Use ReadConsistent sparingly, and prefer Read when possible.
	ReadConsistent(ctx context.Context, id *pbresource.ID) (*pbresource.Resource, error)

	// WriteCAS performs an atomic CAS (Check-And-Set) write of a resource based
	// on its version. The given version will be compared to what is stored, and
	// if it does not match, ErrConflict will be returned. To create new resources,
	// pass an empty version string.
	//
	// If a write cannot be performed because of a consistency or availability
	// issue (e.g. when interacting with a Raft follower, or when quorum is lost)
	// ErrInconsistent will be returned.
	//
	// # UIDs
	//
	// UIDs are immutable, so if the given resource's Uid field doesn't match what
	// is stored, ErrConflict will be returned.
	//
	// See Backend docs for more details.
	//
	// # GroupVersion
	//
	// Write does not validate the GroupVersion and allows you to overwrite a
	// resource stored in an older form with a newer, and vice versa.
	//
	// See Backend docs for more details.
	WriteCAS(ctx context.Context, res *pbresource.Resource, version string) (*pbresource.Resource, error)

	// DeleteCAS performs an atomic CAS (Check-And-Set) deletion of a resource
	// based on its version. The given version will be compared to what is stored,
	// and if it does not match, ErrConflict will be returned.
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
	// In order to list resources across multiple tenancy units (e.g. partitions)
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
	// List makes no guarantees about consistency, and may return stale results.
	List(ctx context.Context, resType UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error)

	// WatchList watches resources of the given type, tenancy, and optionally
	// matching the given name prefix. Upsert events for the current state of the
	// world (i.e. existing resources that match the given filters) will be emitted
	// immediately, and will be followed by delta events whenever resources are
	// written or deleted.
	//
	// See List docs for details about Tenancy Wildcard, GroupVersion, and
	// Consistency.
	WatchList(ctx context.Context, resType UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) (Watch, error)

	// OwnerReferences returns the IDs of resources owned by the resource with the
	// given ID. It is typically used to implement cascading deletion.
	//
	// # Consistency
	//
	// OwnerReferences makes no guarantees about consistency, and may return stale
	// results.
	OwnerReferences(ctx context.Context, id *pbresource.ID) ([]*pbresource.ID, error)
}

// Watch represents a watch on a given set of resources. Call Next to get the
// next event (i.e. upsert or deletion).
type Watch interface {
	// Next returns the next event (i.e. upsert or deletion)
	Next(ctx context.Context) (*pbresource.WatchEvent, error)
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
