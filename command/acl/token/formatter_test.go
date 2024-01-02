// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package token

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
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

func TestFormatToken(t *testing.T) {
	type testCase struct {
		token              api.ACLToken
		overrideGoldenName string
	}

	timeRef := func(in time.Time) *time.Time {
		return &in
	}

	cases := map[string]testCase{
		"basic": {
			token: api.ACLToken{
				AccessorID:  "fbd2447f-7479-4329-ad13-b021d74f86ba",
				SecretID:    "869c6e91-4de9-4dab-b56e-87548435f9c6",
				Description: "test token",
				Local:       false,
				CreateTime:  time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
				Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex: 42,
				ModifyIndex: 100,
			},
		},
		"complex": {
			token: api.ACLToken{
				AccessorID:          "fbd2447f-7479-4329-ad13-b021d74f86ba",
				SecretID:            "869c6e91-4de9-4dab-b56e-87548435f9c6",
				Namespace:           "foo",
				Description:         "test token",
				Local:               false,
				AuthMethod:          "bar",
				AuthMethodNamespace: "baz",
				CreateTime:          time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
				ExpirationTime:      timeRef(time.Date(2020, 5, 22, 19, 52, 31, 0, time.UTC)),
				Hash:                []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex:         5,
				ModifyIndex:         10,
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
				Roles: []*api.ACLLink{
					{
						ID:   "3b0a78fe-b9c3-40de-b8ea-7d4d6674b366",
						Name: "shire",
					},
					{
						ID:   "6c9d1e1d-34bc-4d55-80f3-add0890ad791",
						Name: "west-farthing",
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
					actual, err := formatter.FormatToken(&tcase.token)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)
					if tcase.overrideGoldenName != "" {
						gName = tcase.overrideGoldenName
					}

					expected := golden(t, path.Join("FormatToken", gName), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}

func TestFormatTokenList(t *testing.T) {
	type testCase struct {
		tokens             []*api.ACLTokenListEntry
		overrideGoldenName string
	}

	timeRef := func(in time.Time) *time.Time {
		return &in
	}

	cases := map[string]testCase{
		"basic": {
			tokens: []*api.ACLTokenListEntry{
				{
					AccessorID:  "fbd2447f-7479-4329-ad13-b021d74f86ba",
					SecretID:    "257ade69-748c-4022-bafd-76d27d9143f8",
					Description: "test token",
					Local:       false,
					CreateTime:  time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
					Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
					CreateIndex: 42,
					ModifyIndex: 100,
				},
			},
		},
		"complex": {
			tokens: []*api.ACLTokenListEntry{
				{
					AccessorID:          "fbd2447f-7479-4329-ad13-b021d74f86ba",
					SecretID:            "257ade69-748c-4022-bafd-76d27d9143f8",
					Namespace:           "foo",
					Description:         "test token",
					Local:               false,
					AuthMethod:          "bar",
					AuthMethodNamespace: "baz",
					CreateTime:          time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
					ExpirationTime:      timeRef(time.Date(2020, 5, 22, 19, 52, 31, 0, time.UTC)),
					Hash:                []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
					CreateIndex:         5,
					ModifyIndex:         10,
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
					Roles: []*api.ACLLink{
						{
							ID:   "3b0a78fe-b9c3-40de-b8ea-7d4d6674b366",
							Name: "shire",
						},
						{
							ID:   "6c9d1e1d-34bc-4d55-80f3-add0890ad791",
							Name: "west-farthing",
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
					actual, err := formatter.FormatTokenList(tcase.tokens)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)
					if tcase.overrideGoldenName != "" {
						gName = tcase.overrideGoldenName
					}

					expected := golden(t, path.Join("FormatTokenList", gName), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}

type testCase struct {
	tokenExpanded      api.ACLTokenExpanded
	overrideGoldenName string
}

func timeRef(in time.Time) *time.Time {
	return &in
}

var expandedTokenTestCases = map[string]testCase{
	"basic": {
		tokenExpanded: api.ACLTokenExpanded{
			ExpandedPolicies: []api.ACLPolicy{
				{
					ID:          "beb04680-815b-4d7c-9e33-3d707c24672c",
					Name:        "foo",
					Description: "user policy on token",
					Rules: `service_prefix "" {
  policy = "read"
}`,
				},
				{
					ID:          "18788457-584c-4812-80d3-23d403148a90",
					Name:        "bar",
					Description: "other user policy on token",
					Rules:       `operator = "read"`,
				},
			},
			AgentACLDefaultPolicy: "allow",
			AgentACLDownPolicy:    "deny",
			ResolvedByAgent:       "leader",
			ACLToken: api.ACLToken{
				AccessorID:  "fbd2447f-7479-4329-ad13-b021d74f86ba",
				SecretID:    "869c6e91-4de9-4dab-b56e-87548435f9c6",
				Description: "test token",
				Local:       false,
				CreateTime:  time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
				Hash:        []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex: 42,
				ModifyIndex: 100,
				Policies: []*api.ACLLink{
					{
						ID:   "beb04680-815b-4d7c-9e33-3d707c24672c",
						Name: "foo",
					},
					{
						ID:   "18788457-584c-4812-80d3-23d403148a90",
						Name: "bar",
					},
				},
			},
		},
	},
	"complex": {
		tokenExpanded: api.ACLTokenExpanded{
			ExpandedPolicies: []api.ACLPolicy{
				{
					ID:          "beb04680-815b-4d7c-9e33-3d707c24672c",
					Name:        "hobbiton",
					Description: "user policy on token",
					Rules: `service_prefix "" {
  policy = "read"
}`,
				},
				{
					ID:          "18788457-584c-4812-80d3-23d403148a90",
					Name:        "bywater",
					Description: "other user policy on token",
					Rules:       `operator = "read"`,
				},
				{
					ID:          "6204f4cd-4709-441c-ac1b-cb029e940263",
					Name:        "shire-policy",
					Description: "policy for shire role",
					Rules:       `operator = "write"`,
				},
				{
					ID:          "e86f0d1f-71b1-4690-bdfd-ff8c2cd4ae93",
					Name:        "west-farthing-policy",
					Description: "policy for west-farthing role",
					Rules: `service "foo" {
  policy = "read"
}`,
				},
				{
					ID:          "2b582ff1-4a43-457f-8a2b-30a8265e29a5",
					Name:        "default-policy-1",
					Description: "default policy 1",
					Rules:       `key "foo" { policy = "write" }`,
				},
				{
					ID:          "b55dce64-f2cc-4eb5-8e5f-50e90e63c6ea",
					Name:        "default-policy-2",
					Description: "default policy 2",
					Rules:       `key "bar" { policy = "read" }`,
				},
			},
			ExpandedRoles: []api.ACLRole{
				{
					ID:          "3b0a78fe-b9c3-40de-b8ea-7d4d6674b366",
					Name:        "shire",
					Description: "shire role",
					Policies: []*api.ACLRolePolicyLink{
						{
							ID: "6204f4cd-4709-441c-ac1b-cb029e940263",
						},
					},
					ServiceIdentities: []*api.ACLServiceIdentity{
						{
							ServiceName: "foo",
							Datacenters: []string{"middleearth-southwest"},
						},
					},
				},
				{
					ID:          "6c9d1e1d-34bc-4d55-80f3-add0890ad791",
					Name:        "west-farthing",
					Description: "west-farthing role",
					Policies: []*api.ACLRolePolicyLink{
						{
							ID: "e86f0d1f-71b1-4690-bdfd-ff8c2cd4ae93",
						},
					},
					NodeIdentities: []*api.ACLNodeIdentity{
						{
							NodeName:   "bar",
							Datacenter: "middleearth-southwest",
						},
					},
				},
				{
					ID:          "56033f2b-e1a6-4905-b71d-e011c862bc65",
					Name:        "ns-default",
					Description: "default role",
					Policies: []*api.ACLRolePolicyLink{
						{
							ID: "b55dce64-f2cc-4eb5-8e5f-50e90e63c6ea",
						},
					},
					ServiceIdentities: []*api.ACLServiceIdentity{
						{
							ServiceName: "web",
							Datacenters: []string{"middleearth-northeast"},
						},
					},
					NodeIdentities: []*api.ACLNodeIdentity{
						{
							NodeName:   "db",
							Datacenter: "middleearth-northwest",
						},
					},
				},
			},
			NamespaceDefaultPolicyIDs: []string{"2b582ff1-4a43-457f-8a2b-30a8265e29a5"},
			NamespaceDefaultRoleIDs:   []string{"56033f2b-e1a6-4905-b71d-e011c862bc65"},
			AgentACLDefaultPolicy:     "deny",
			AgentACLDownPolicy:        "extend-cache",
			ResolvedByAgent:           "server-1",
			ACLToken: api.ACLToken{
				AccessorID:          "fbd2447f-7479-4329-ad13-b021d74f86ba",
				SecretID:            "869c6e91-4de9-4dab-b56e-87548435f9c6",
				Namespace:           "foo",
				Description:         "test token",
				Local:               false,
				AuthMethod:          "bar",
				AuthMethodNamespace: "baz",
				CreateTime:          time.Date(2020, 5, 22, 18, 52, 31, 0, time.UTC),
				ExpirationTime:      timeRef(time.Date(2020, 5, 22, 19, 52, 31, 0, time.UTC)),
				Hash:                []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'},
				CreateIndex:         5,
				ModifyIndex:         10,
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
				Roles: []*api.ACLLink{
					{
						ID:   "3b0a78fe-b9c3-40de-b8ea-7d4d6674b366",
						Name: "shire",
					},
					{
						ID:   "6c9d1e1d-34bc-4d55-80f3-add0890ad791",
						Name: "west-farthing",
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

func testFormatTokenExpanded(t *testing.T, dirPath string) {
	formatters := map[string]Formatter{
		"pretty":      newPrettyFormatter(false),
		"pretty-meta": newPrettyFormatter(true),
		// the JSON formatter ignores the showMeta
		"json": newJSONFormatter(false),
	}

	for name, tcase := range expandedTokenTestCases {
		t.Run(name, func(t *testing.T) {
			for fmtName, formatter := range formatters {
				t.Run(fmtName, func(t *testing.T) {
					actual, err := formatter.FormatTokenExpanded(&tcase.tokenExpanded)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)
					if tcase.overrideGoldenName != "" {
						gName = tcase.overrideGoldenName
					}

					expected := golden(t, path.Join(dirPath, gName), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}
