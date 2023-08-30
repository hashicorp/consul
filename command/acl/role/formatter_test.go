// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package role

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

// golden reads and optionally writes the expected data to the golden file,
// returning the contents as a string.
func golden(t *testing.T, name, got string) string {
	t.Helper()

	golden := filepath.Join("testdata", name+".golden")
	if *update && got != "" {
		err := os.WriteFile(golden, []byte(got), 0644)
		require.NoError(t, err)
	}

	expected, err := os.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

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

					expected := golden(t, path.Join("FormatRole", gName), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}

func TestFormatTokenList(t *testing.T) {
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

					expected := golden(t, path.Join("FormatRoleList", gName), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}
