// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func makeExtAuthzEnvoyExtension(svc string, opts ...string) []structs.EnvoyExtension {
	target := map[string]any{"URI": "127.0.0.1:9191"}
	insertOptions := map[string]any{}
	required := false
	ent := false
	serviceKey := "GrpcService"
	if svc == "http" {
		serviceKey = "HttpService"
	}

	configMap := map[string]any{
		serviceKey: map[string]any{
			"Target": target,
		},
	}

	for _, opt := range opts {
		if k, v, valid := strings.Cut(opt, "="); valid {
			switch k {
			case "required":
				required = true
			case "enterprise":
				ent = true
			case "dest":
				if v == "upstream" {
					svcMap := map[string]any{"Name": "db"}
					if ent {
						svcMap["Namespace"] = "bar"
						svcMap["Partition"] = "zip"
					}
					target = map[string]any{"Service": svcMap}
				}
			case "insert":
				if location, filterName, found := strings.Cut(v, ":"); found {
					insertOptions = map[string]any{
						"Location":   location,
						"FilterName": filterName,
					}
				}
			case "config-type":
				if v == "full" {
					target["Timeout"] = "2s"
					configMap = map[string]any{
						serviceKey: map[string]any{
							"Target":     target,
							"PathPrefix": "/ext_authz",
							"Authority":  "test-authority",
							"AuthorizationRequest": map[string]any{
								"AllowedHeaders": makeStringMatcherSlice("Exact", "allow-header", 2),
								"HeadersToAdd": []map[string]any{
									{"Key": "add-header-1", "Value": "foo"},
									{"Key": "add-header-2", "Value": "bar"},
								},
							},
							"AuthorizationResponse": map[string]any{
								"AllowedUpstreamHeaders":         makeStringMatcherSlice("Contains", "upstream-header", 2),
								"AllowedUpstreamHeadersToAppend": makeStringMatcherSlice("Exact", "add-upstream", 2),
								"AllowedClientHeaders":           makeStringMatcherSlice("Prefix", "client-header", 2),
								"AllowedClientHeadersOnSuccess":  makeStringMatcherSlice("SafeRegex", "client-ok-header", 2),
								"DynamicMetadataFromHeaders":     makeStringMatcherSlice("Suffix", "dynamic-header", 2),
							},
							"InitialMetadata": []map[string]any{
								{"Key": "init-metadata-1", "Value": "value 1"},
								{"Key": "init-metadata-2", "Value": "value 2"},
							},
						},
						"BootstrapMetadataLabelsKey": "test-labels-key",
						"ClearRouteCache":            true,
						"IncludePeerCertificate":     true,
						"MetadataContextNamespaces":  []string{"test-ns-1", "test-ns-2"},
						"StatusOnError":              417,
						"StatPrefix":                 "ext_authz_stats",
						"WithRequestBody": map[string]any{
							"MaxRequestBytes":     10000,
							"AllowPartialMessage": true,
							"PackAsBytes":         true,
						},
					}
				}
			}
		}
	}
	return []structs.EnvoyExtension{
		{
			Name:     api.BuiltinExtAuthzExtension,
			Required: required,
			Arguments: map[string]any{
				"InsertOptions": insertOptions,
				"Config":        configMap,
			},
		},
	}
}

func makeStringMatcherSlice(match, name string, count int) []map[string]any {
	var s []map[string]any
	for i := 0; i < count; i++ {
		s = append(s, map[string]any{
			match:        fmt.Sprintf("%s-%d", name, i+1),
			"IgnoreCase": match != "SafeRegex" && i == 1,
		})
	}
	return s
}
