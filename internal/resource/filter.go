// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"fmt"

	"github.com/hashicorp/go-bexpr"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type MetadataFilterableResources interface {
	GetMetadata() map[string]string
}

// FilterResourcesByMetadata will use the provided go-bexpr based filter to
// retain matching items from the provided slice.
//
// The only variables usable in the expressions are the metadata keys prefixed
// by "metadata."
//
// If no filter is provided, then this does nothing and returns the input.
func FilterResourcesByMetadata[T MetadataFilterableResources](resources []T, filter string) ([]T, error) {
	if filter == "" || len(resources) == 0 {
		return resources, nil
	}

	eval, err := createMetadataFilterEvaluator(filter)
	if err != nil {
		return nil, err
	}

	filtered := make([]T, 0, len(resources))
	for _, res := range resources {
		vars := &metadataFilterFieldDetails{
			Meta: res.GetMetadata(),
		}
		match, err := eval.Evaluate(vars)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, res)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	return filtered, nil
}

// FilterMatchesResourceMetadata will use the provided go-bexpr based filter to
// determine if the provided resource matches.
//
// The only variables usable in the expressions are the metadata keys prefixed
// by "metadata."
//
// If no filter is provided, then this returns true.
func FilterMatchesResourceMetadata(res *pbresource.Resource, filter string) (bool, error) {
	if res == nil {
		return false, nil
	} else if filter == "" {
		return true, nil
	}

	eval, err := createMetadataFilterEvaluator(filter)
	if err != nil {
		return false, err
	}

	vars := &metadataFilterFieldDetails{
		Meta: res.Metadata,
	}
	match, err := eval.Evaluate(vars)
	if err != nil {
		return false, err
	}
	return match, nil
}

// ValidateMetadataFilter will validate that the provided filter is going to be
// a valid input to the FilterResourcesByMetadata function.
//
// This is best called from a Validate hook.
func ValidateMetadataFilter(filter string) error {
	if filter == "" {
		return nil
	}

	_, err := createMetadataFilterEvaluator(filter)
	return err
}

func createMetadataFilterEvaluator(filter string) (*bexpr.Evaluator, error) {
	sampleVars := &metadataFilterFieldDetails{
		Meta: make(map[string]string),
	}
	eval, err := bexpr.CreateEvaluatorForType(filter, nil, sampleVars)
	if err != nil {
		return nil, fmt.Errorf("filter %q is invalid: %w", filter, err)
	}
	return eval, nil
}

type metadataFilterFieldDetails struct {
	Meta map[string]string `bexpr:"metadata"`
}
