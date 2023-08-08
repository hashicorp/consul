// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	ErrMissing                  = NewConstError("missing required field")
	ErrEmpty                    = NewConstError("cannot be empty")
	ErrReferenceTenancyNotEqual = NewConstError("resource tenancy and reference tenancy differ")
)

// ConstError is more or less equivalent to the stdlib errors.errorstring. However, having
// our own exported type allows us to more accurately compare error values in tests.
//
//   - go-cmp will not compared unexported fields by default.
//   - cmp.AllowUnexported(<type>) requires a concrete struct type and due to the stdlib not
//     exporting the errorstring type there doesn't seem to be a way to get at the type.
//   - cmpopts.EquateErrors has issues with protobuf types within other error structs.
//
// Due to these factors the easiest thing to do is to create a custom comparer for
// the ConstError type and use it where necessary.
type ConstError struct {
	message string
}

func NewConstError(msg string) ConstError {
	return ConstError{message: msg}
}

func (e ConstError) Error() string {
	return e.message
}

type ErrDataParse struct {
	TypeName string
	Wrapped  error
}

func NewErrDataParse(msg protoreflect.ProtoMessage, err error) ErrDataParse {
	return ErrDataParse{
		TypeName: string(msg.ProtoReflect().Descriptor().FullName()),
		Wrapped:  err,
	}
}

func (err ErrDataParse) Error() string {
	return fmt.Sprintf("error parsing resource data as type %q: %s", err.TypeName, err.Wrapped.Error())
}

func (err ErrDataParse) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidField struct {
	Name    string
	Wrapped error
}

func (err ErrInvalidField) Error() string {
	return fmt.Sprintf("invalid %q field: %v", err.Name, err.Wrapped)
}

func (err ErrInvalidField) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidListElement struct {
	Name    string
	Index   int
	Wrapped error
}

func (err ErrInvalidListElement) Error() string {
	return fmt.Sprintf("invalid element at index %d of list %q: %v", err.Index, err.Name, err.Wrapped)
}

func (err ErrInvalidListElement) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidMapValue struct {
	Map     string
	Key     string
	Wrapped error
}

func (err ErrInvalidMapValue) Error() string {
	return fmt.Sprintf("invalid value of key %q within %s: %v", err.Key, err.Map, err.Wrapped)
}

func (err ErrInvalidMapValue) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidMapKey struct {
	Map     string
	Key     string
	Wrapped error
}

func (err ErrInvalidMapKey) Error() string {
	return fmt.Sprintf("map %s contains an invalid key - %q: %v", err.Map, err.Key, err.Wrapped)
}

func (err ErrInvalidMapKey) Unwrap() error {
	return err.Wrapped
}

type ErrOwnerTypeInvalid struct {
	ResourceType *pbresource.Type
	OwnerType    *pbresource.Type
}

func (err ErrOwnerTypeInvalid) Error() string {
	return fmt.Sprintf(
		"resources of type %s cannot be owned by resources with type %s",
		ToGVK(err.ResourceType),
		ToGVK(err.OwnerType),
	)
}

type ErrOwnerTenantInvalid struct {
	ResourceType    *pbresource.Type
	ResourceTenancy *pbresource.Tenancy
	OwnerTenancy    *pbresource.Tenancy
}

func (err ErrOwnerTenantInvalid) Error() string {
	return fmt.Sprintf(
		"resource in partition %s, namespace %s and peer %s cannot be owned by a resource in partition %s, namespace %s and peer %s",
		err.ResourceTenancy.Partition, err.ResourceTenancy.Namespace, err.ResourceTenancy.PeerName,
		err.OwnerTenancy.Partition, err.OwnerTenancy.Namespace, err.OwnerTenancy.PeerName,
	)
}

type ErrInvalidReferenceType struct {
	AllowedType *pbresource.Type
}

func (err ErrInvalidReferenceType) Error() string {
	return fmt.Sprintf("reference must have type %s", ToGVK(err.AllowedType))
}
