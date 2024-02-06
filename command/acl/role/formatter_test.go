// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package role

import (
	"fmt"
	"path"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/internal/testing/golden"
	"github.com/stretchr/testify/require"
)

func TestFormatRole(t *testing.T) {
	type testCase struct {
		role               api.ACLRole
		overrideGoldenName string
	}

	cases := map[string]testCase{
		"basic": {
			role: api.ACLRole{
				ID:          "bd6c9fb0-2d1a-4b96-acaf-669f5d7e7852",
				Name:        "basic",
				Description: "test role",
				Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex: 42,
				ModifyIndex: 100,
			},
		},
		"complex": {
			role: api.ACLRole{
				ID:          "c29c4ee4-bca6-474e-be37-7d9606f9582a",
				Name:        "complex",
				Namespace:   "foo",
				Description: "test role complex",
				Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex: 5,
				ModifyIndex: 10,
				Policies: []*api.ACLLink{
					{
						ID:   "beb04680-815b-4d7c-9e33-3d707c24672c",
						Name: "hobbiton",
					},
					{
						ID:   "18788457-584c-4812-80d3-23d403148a90",
						Name: "bywater",
					},
				},
				ServiceIdentities: []*api.ACLServiceIdentity{
					{
						ServiceName: "gardener",
						Datacenters: []string{"middleearth-northwest"},
					},
				},
				NodeIdentities: []*api.ACLNodeIdentity{
					{
						NodeName:   "bagend",
						Datacenter: "middleearth-northwest",
					},
				},
				TemplatedPolicies: []*api.ACLTemplatedPolicy{
					{
						TemplateName:      api.ACLTemplatedPolicyServiceName,
						TemplateVariables: &api.ACLTemplatedPolicyVariables{Name: "gardener"},
						Datacenters:       []string{"middleearth-northwest", "somewhere-east"},
					},
					{TemplateName: api.ACLTemplatedPolicyNodeName, TemplateVariables: &api.ACLTemplatedPolicyVariables{Name: "bagend"}},
				},
			},
		},
	}

	formatters := map[string]Formatter{
		"pretty":      newPrettyFormatter(false),
		"pretty-meta": newPrettyFormatter(true),
		// the JSON formatter ignores the showMeta
		"json": newJSONFormatter(false),
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			for fmtName, formatter := range formatters {
				t.Run(fmtName, func(t *testing.T) {
					actual, err := formatter.FormatRole(&tcase.role)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)
					if tcase.overrideGoldenName != "" {
						gName = tcase.overrideGoldenName
					}

					expected := golden.Get(t, actual, path.Join("FormatRole", gName))
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}

func TestFormatRoleList(t *testing.T) {
	type testCase struct {
		roles              []*api.ACLRole
		overrideGoldenName string
	}

	cases := map[string]testCase{
		"basic": {
			roles: []*api.ACLRole{
				{
					ID:          "bd6c9fb0-2d1a-4b96-acaf-669f5d7e7852",
					Name:        "basic",
					Description: "test role",
					Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
					CreateIndex: 42,
					ModifyIndex: 100,
				},
			},
		},
		"complex": {
			roles: []*api.ACLRole{
				{
					ID:          "c29c4ee4-bca6-474e-be37-7d9606f9582a",
					Name:        "complex",
					Namespace:   "foo",
					Description: "test role complex",
					Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
					CreateIndex: 5,
					ModifyIndex: 10,
					Policies: []*api.ACLLink{
						{
							ID:   "beb04680-815b-4d7c-9e33-3d707c24672c",
							Name: "hobbiton",
						},
						{
							ID:   "18788457-584c-4812-80d3-23d403148a90",
							Name: "bywater",
						},
					},
					ServiceIdentities: []*api.ACLServiceIdentity{
						{
							ServiceName: "gardener",
							Datacenters: []string{"middleearth-northwest"},
						},
					},
					NodeIdentities: []*api.ACLNodeIdentity{
						{
							NodeName:   "bagend",
							Datacenter: "middleearth-northwest",
						},
					},
					TemplatedPolicies: []*api.ACLTemplatedPolicy{
						{TemplateName: api.ACLTemplatedPolicyServiceName, TemplateVariables: &api.ACLTemplatedPolicyVariables{Name: "gardener"}},
						{TemplateName: api.ACLTemplatedPolicyNodeName, TemplateVariables: &api.ACLTemplatedPolicyVariables{Name: "bagend"}},
					},
				},
			},
		},
	}

	formatters := map[string]Formatter{
		"pretty":      newPrettyFormatter(false),
		"pretty-meta": newPrettyFormatter(true),
		// the JSON formatter ignores the showMeta
		"json": newJSONFormatter(false),
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			for fmtName, formatter := range formatters {
				t.Run(fmtName, func(t *testing.T) {
					actual, err := formatter.FormatRoleList(tcase.roles)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)
					if tcase.overrideGoldenName != "" {
						gName = tcase.overrideGoldenName
					}

					expected := golden.Get(t, actual, path.Join("FormatRoleList", gName))
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}
