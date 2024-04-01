// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/hashicorp/consul/internal/dnsutil"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MaxNameLength is the maximum length of a resource name.
const MaxNameLength = 63

// DeletionTimestampKey is the key in a resource's metadata that stores the timestamp
// when a resource was marked for deletion. This only applies to resources with finalizers.
const DeletionTimestampKey = "deletionTimestamp"

// FinalizerKey is the key in resource's metadata that stores the whitespace separated
// list of finalizers.
const FinalizerKey = "finalizers"

// ValidateName returns an error a name is not a valid resource name.
// The error will contain reference to what constitutes a valid resource name.
func ValidateName(name string) error {
	if !dnsutil.IsValidLabel(name) || strings.ToLower(name) != name || len(name) > MaxNameLength {
		return fmt.Errorf("a resource name must consist of lower case alphanumeric characters or '-', must start and end with an alphanumeric character and be less than %d characters, got: %q", MaxNameLength+1, name)
	}
	return nil
}

// IsMarkedForDeletion returns true if a resource has been marked for deletion,
// false otherwise.
func IsMarkedForDeletion(res *pbresource.Resource) bool {
	if res.Metadata == nil {
		return false
	}
	_, ok := res.Metadata[DeletionTimestampKey]
	return ok
}

// HasFinalizers returns true if a resource has one or more finalizers, false otherwise.
func HasFinalizers(res *pbresource.Resource) bool {
	return GetFinalizers(res).Cardinality() >= 1
}

// HasFinalizer returns true if a resource has a given finalizers, false otherwise.
func HasFinalizer(res *pbresource.Resource, finalizer string) bool {
	return GetFinalizers(res).Contains(finalizer)
}

// AddFinalizer adds a finalizer to the given resource.
func AddFinalizer(res *pbresource.Resource, finalizer string) {
	finalizerSet := GetFinalizers(res)
	finalizerSet.Add(finalizer)
	if res.Metadata == nil {
		res.Metadata = map[string]string{}
	}
	res.Metadata[FinalizerKey] = strings.Join(finalizerSet.ToSlice(), " ")
}

// RemoveFinalizer removes a finalizer from the given resource.
func RemoveFinalizer(res *pbresource.Resource, finalizer string) {
	finalizerSet := GetFinalizers(res)
	finalizerSet.Remove(finalizer)

	if finalizerSet.Cardinality() == 0 {
		// Remove key if no finalizers to prevent dual representations of
		// the same state.
		_, keyExists := res.Metadata[FinalizerKey]
		if keyExists {
			delete(res.Metadata, FinalizerKey)
		}
	} else {
		// Add/update key
		if res.Metadata == nil {
			res.Metadata = map[string]string{}
		}
		res.Metadata[FinalizerKey] = strings.Join(finalizerSet.ToSlice(), " ")
	}
}

// GetFinalizers returns the set of finalizers for the given resource.
func GetFinalizers(res *pbresource.Resource) mapset.Set[string] {
	if res.Metadata == nil {
		return mapset.NewSet[string]()
	}
	finalizers, ok := res.Metadata[FinalizerKey]
	if !ok {
		return mapset.NewSet[string]()
	}
	return mapset.NewSet[string](strings.Fields(finalizers)...)
}
