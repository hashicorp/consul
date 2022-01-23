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

type PermissionDeniedError struct {
	Resource    Resource
	AccessLevel AccessLevel
	AccessorID  string
	ResourceID  string

	// TODO: remove Cause, use other fields
	Cause string
}

func (e PermissionDeniedError) Error() string {
	if e.Cause != "" {
		return errPermissionDenied + ": " + e.Cause
	}
	if e.Resource != "" {
		return fmt.Sprintf("%s: token %s is missing %s:%s for %s",
			errPermissionDenied, e.AccessorID, e.Resource, e.AccessLevel, e.ResourceID)
	}
	return errPermissionDenied
}

func PermissionDenied(msg string, args ...interface{}) PermissionDeniedError {
	cause := fmt.Sprintf(msg, args...)
	return PermissionDeniedError{Cause: cause}
}
