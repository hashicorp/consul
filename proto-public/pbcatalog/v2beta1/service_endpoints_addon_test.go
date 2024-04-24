// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceEndpoints_GetIdentities(t *testing.T) {
	cases := map[string]struct {
		endpoints     []*Endpoint
		expIdentities []string
	}{
		"no endpoints": {
			endpoints:     nil,
			expIdentities: nil,
		},
		"no identities": {
			endpoints: []*Endpoint{
				{},
				{},
			},
			expIdentities: nil,
		},
		"single identity": {
			endpoints: []*Endpoint{
				{Identity: "foo"},
				{Identity: "foo"},
				{Identity: "foo"},
			},
			expIdentities: []string{"foo"},
		},
		"multiple identities": {
			endpoints: []*Endpoint{
				{Identity: "foo"},
				{Identity: "foo"},
				{Identity: "bar"},
			},
			expIdentities: []string{"bar", "foo"},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			se := &ServiceEndpoints{Endpoints: c.endpoints}
			require.Equal(t, c.expIdentities, se.GetIdentities())
		})
	}
}
