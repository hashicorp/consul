// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestEqualStatus(t *testing.T) {
	generation := ulid.Make().String()

	for idx, tc := range []struct {
		a, b  map[string]*pbresource.Status
		equal bool
	}{
		{nil, nil, true},
		{nil, map[string]*pbresource.Status{}, true},
		{
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			true,
		},
		{
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_FALSE,
							Reason:  "Bar",
							Message: "Foo is false because of Bar",
						},
					},
				},
			},
			false,
		},
		{
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			map[string]*pbresource.Status{
				"consul.io/some-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
				"consul.io/other-controller": {
					ObservedGeneration: generation,
					Conditions: []*pbresource.Condition{
						{
							Type:    "Foo",
							State:   pbresource.Condition_STATE_TRUE,
							Reason:  "Bar",
							Message: "Foo is true because of Bar",
						},
					},
				},
			},
			false,
		},
	} {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			require.Equal(t, tc.equal, EqualStatus(tc.a, tc.b))
			require.Equal(t, tc.equal, EqualStatus(tc.b, tc.a))
		})
	}
}
