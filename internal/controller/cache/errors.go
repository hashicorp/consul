package cache

import (
	"errors"
	"fmt"
)

var (
	TypeUnspecifiedError      = errors.New("the resource type was not specified")
	TypeNotIndexedError       = errors.New("the resource type specified is not indexed")
	MissingRequiredIndexError = errors.New("resource is missing required index")
)

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
	return fmt.Sprintf("operation on indexed %q failed: %v", e.name, e.err.Error())
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
