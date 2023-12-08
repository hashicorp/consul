// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package templatedpolicy

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

// golden reads from the golden file returning the contents as a string.
func golden(t *testing.T, name string) string {
	t.Helper()

	golden := filepath.Join("testdata", name+".golden")
	expected, err := os.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func testFormatTemplatedPolicy(t *testing.T, dirPath string) {
	type testCase struct {
		templatedPolicy api.ACLTemplatedPolicyResponse
	}

	cases := map[string]testCase{
		"node-templated-policy": {
			templatedPolicy: api.ACLTemplatedPolicyResponse{
				TemplateName: api.ACLTemplatedPolicyNodeName,
				Schema:       structs.ACLTemplatedPolicyNodeSchema,
				Template:     structs.ACLTemplatedPolicyNode,
				Description:  structs.ACLTemplatedPolicyNodeDescription,
			},
		},
		"dns-templated-policy": {
			templatedPolicy: api.ACLTemplatedPolicyResponse{
				TemplateName: api.ACLTemplatedPolicyDNSName,
				Schema:       structs.ACLTemplatedPolicyNoRequiredVariablesSchema,
				Template:     structs.ACLTemplatedPolicyDNS,
				Description:  structs.ACLTemplatedPolicyDNSDescription,
			},
		},
		"service-templated-policy": {
			templatedPolicy: api.ACLTemplatedPolicyResponse{
				TemplateName: api.ACLTemplatedPolicyServiceName,
				Schema:       structs.ACLTemplatedPolicyServiceSchema,
				Template:     structs.ACLTemplatedPolicyService,
				Description:  structs.ACLTemplatedPolicyServiceDescription,
			},
		},
		"nomad-server-templated-policy": {
			templatedPolicy: api.ACLTemplatedPolicyResponse{
				TemplateName: api.ACLTemplatedPolicyNomadServerName,
				Schema:       structs.ACLTemplatedPolicyNoRequiredVariablesSchema,
				Template:     structs.ACLTemplatedPolicyNomadServer,
				Description:  structs.ACLTemplatedPolicyNomadServerDescription,
			},
		},
		"nomad-client-templated-policy": {
			templatedPolicy: api.ACLTemplatedPolicyResponse{
				TemplateName: api.ACLTemplatedPolicyNomadClientName,
				Schema:       structs.ACLTemplatedPolicyNoRequiredVariablesSchema,
				Template:     structs.ACLTemplatedPolicyNomadClient,
				Description:  structs.ACLTemplatedPolicyNomadClientDescription,
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
					actual, err := formatter.FormatTemplatedPolicy(tcase.templatedPolicy)
					require.NoError(t, err)

					gName := fmt.Sprintf("%s.%s", name, fmtName)

					expected := golden(t, path.Join(dirPath, gName))
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}

func testFormatTemplatedPolicyList(t *testing.T, dirPath string) {
	// we don't consider the showMeta field for policy list
	formatters := map[string]Formatter{
		"pretty": newPrettyFormatter(false),
		"json":   newJSONFormatter(false),
	}

	policies := map[string]api.ACLTemplatedPolicyResponse{
		"builtin/node": {
			TemplateName: api.ACLTemplatedPolicyNodeName,
			Schema:       structs.ACLTemplatedPolicyNodeSchema,
			Template:     structs.ACLTemplatedPolicyNode,
			Description:  structs.ACLTemplatedPolicyNodeDescription,
		},
		"builtin/dns": {
			TemplateName: api.ACLTemplatedPolicyDNSName,
			Schema:       structs.ACLTemplatedPolicyNoRequiredVariablesSchema,
			Template:     structs.ACLTemplatedPolicyDNS,
			Description:  structs.ACLTemplatedPolicyDNSDescription,
		},
		"builtin/service": {
			TemplateName: api.ACLTemplatedPolicyServiceName,
			Schema:       structs.ACLTemplatedPolicyServiceSchema,
			Template:     structs.ACLTemplatedPolicyService,
			Description:  structs.ACLTemplatedPolicyServiceDescription,
		},
	}

	for fmtName, formatter := range formatters {
		t.Run(fmtName, func(t *testing.T) {
			actual, err := formatter.FormatTemplatedPolicyList(policies)
			require.NoError(t, err)

			gName := fmt.Sprintf("list.%s", fmtName)

			expected := golden(t, path.Join(dirPath, gName))
			require.Equal(t, expected, actual)
		})
	}
}
