// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"errors"
	"fmt"
)

var (
	QueryRequired        = errors.New("A non-nil query function was not specified")
	TypeUnspecifiedError = errors.New("the resource type was not specified")
	TypeNotIndexedError  = errors.New("the resource type specified is not indexed")
)

type QueryNotFoundError struct {
	name string
}

func (e QueryNotFoundError) Error() string {
	return fmt.Sprintf("No query with name %q exists", e.name)
}

type IndexNotFoundError struct {
	name string
}

func (e IndexNotFoundError) Error() string {
	return fmt.Sprintf("No index with name %q exists", e.name)
}

type CacheTypeError struct {
	err error
	it  unversionedType
}

func (e CacheTypeError) Error() string {
	return fmt.Sprintf("operation on resource type %s.%s failed: %v", e.it.Group, e.it.Kind, e.err.Error())
}

func (e CacheTypeError) Unwrap() error {
	return e.err
}

type IndexError struct {
	err  error
	name string
}

func (e IndexError) Error() string {
	return fmt.Sprintf("operation on index %q failed: %v", e.name, e.err.Error())
}

func (e IndexError) Unwrap() error {
	return e.err
}

type DuplicateIndexError struct {
	name string
}

func (e DuplicateIndexError) Error() string {
	return fmt.Sprintf("Index with name %q is already defined.", e.name)
}

type DuplicateQueryError struct {
	name string
}

func (e DuplicateQueryError) Error() string {
	return fmt.Sprintf("Query with name %q is already defined.", e.name)
}
