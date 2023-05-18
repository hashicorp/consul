// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	ErrMissing                  = errors.New("missing required field")
	ErrEmpty                    = errors.New("cannot be empty")
	ErrReferenceTenancyNotEqual = errors.New("resource tenancy and reference tenancy differ")
)

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

type ErrOwnerInvalid struct {
	ResourceType *pbresource.Type
	OwnerType    *pbresource.Type
}

func (err ErrOwnerInvalid) Error() string {
	return fmt.Sprintf(
		"resources of type %s cannot be owned by resources with type %s",
		ToGVK(err.ResourceType),
		ToGVK(err.OwnerType),
	)
}

type ErrInvalidReferenceType struct {
	AllowedType *pbresource.Type
}

func (err ErrInvalidReferenceType) Error() string {
	return fmt.Sprintf("reference must have type %s", ToGVK(err.AllowedType))
}
