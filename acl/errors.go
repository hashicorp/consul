package acl

import (
	"errors"
	"fmt"
	"strings"
)

// These error constants define the standard ACL error types. The values
// must not be changed since the error values are sent via RPC calls
// from older clients and may not have the correct type.
const (
	errNotFound         = "ACL not found"
	errRootDenied       = "Cannot resolve root ACL"
	errDisabled         = "ACL support disabled"
	errPermissionDenied = "Permission denied"
	errInvalidParent    = "Invalid Parent"
)

var (
	// ErrNotFound indicates there is no matching ACL.
	ErrNotFound = errors.New(errNotFound)

	// ErrRootDenied is returned when attempting to resolve a root ACL.
	ErrRootDenied = errors.New(errRootDenied)

	// ErrDisabled is returned when ACL changes are not permitted since
	// they are disabled.
	ErrDisabled = errors.New(errDisabled)

	// ErrPermissionDenied is returned when an ACL based rejection
	// happens.
	ErrPermissionDenied = PermissionDeniedError{}

	// ErrInvalidParent is returned when a remotely resolve ACL
	// token claims to have a non-root parent
	ErrInvalidParent = errors.New(errInvalidParent)
)

// IsErrNotFound checks if the given error message is comparable to
// ErrNotFound.
func IsErrNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNotFound)
}

// IsErrRootDenied checks if the given error message is comparable to
// ErrRootDenied.
func IsErrRootDenied(err error) bool {
	return err != nil && strings.Contains(err.Error(), errRootDenied)
}

// IsErrDisabled checks if the given error message is comparable to
// ErrDisabled.
func IsErrDisabled(err error) bool {
	return err != nil && strings.Contains(err.Error(), errDisabled)
}

// IsErrPermissionDenied checks if the given error message is comparable
// to ErrPermissionDenied.
func IsErrPermissionDenied(err error) bool {
	return err != nil && strings.Contains(err.Error(), errPermissionDenied)
}

// Arguably this should be some sort of union type.
// The usage of Cause and the rest of the fields is entirely disjoint.
//
type PermissionDeniedError struct {
	Cause string

	// Accessor contains information on the accessor used e.g. "token <GUID>"
	Accessor string
	// Resource (e.g. Service)
	Resource string
	// Access leve (e.g. Read)
	AccessLevel string
	// e.g. "sidecar-proxy-1"
	ResourceID ResourceDescriptor
}

// Initially we may not have attribution information; that will become more complete as we work this change through
// There are generally three classes of errors
// 1) Named entities without a context
// 2) Unnamed entities with a context
// 3) Completely context free checks (global permissions)
// 4) Errors that only have a cause and bad
func (e PermissionDeniedError) Error() string {
	var message strings.Builder
	message.WriteString(errPermissionDenied)

	// This is used where we
	if e.Cause != "" {
		fmt.Fprintf(&message, ": %s", e.Cause)
		return message.String()
	}
	if e.Resource == "" {
		return message.String()
	}

	if e.Accessor == "" {
		message.WriteString(": provided accessor")
	} else {
		fmt.Fprintf(&message, ": accessor '%s'", e.Accessor)
	}

	fmt.Fprintf(&message, " lacks permission '%s:%s'", e.Resource, e.AccessLevel)

	if e.ResourceID.Name != "" {
		fmt.Fprintf(&message, " %s", e.ResourceID.ToString())
	}
	return message.String()
}

func PermissionDenied(msg string, args ...interface{}) PermissionDeniedError {
	cause := fmt.Sprintf(msg, args...)
	return PermissionDeniedError{Cause: cause}
}

// TODO Extract information from Authorizer
func PermissionDeniedByACL(_ Authorizer, context *AuthorizerContext, resource string, accessLevel string, resourceID string) PermissionDeniedError {
	desc := NewResourceDescriptor(resourceID, context)
	return PermissionDeniedError{Accessor: "", Resource: resource, AccessLevel: accessLevel, ResourceID: desc}
}

func PermissionDeniedByACLUnnamed(_ Authorizer, context *AuthorizerContext, accessLevel string, permission string) PermissionDeniedError {
	desc := NewResourceDescriptor("", context)
	return PermissionDeniedError{Accessor: "", Resource: accessLevel, AccessLevel: permission, ResourceID: desc}
}
