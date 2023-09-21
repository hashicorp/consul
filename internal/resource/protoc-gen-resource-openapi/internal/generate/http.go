// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package generate

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource/protoc-gen-resource-openapi/internal/types"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	consistentParameter = &types.Parameter{
		Name:        "consistent",
		In:          "query",
		Schema:      types.BoolSchema(),
		Description: "When true, the operation will be performed with strong consistency",
	}

	namePrefixParameter = &types.Parameter{
		Name:        "name_prefix",
		In:          "query",
		Description: "The resource name prefix used to filter the result list.",
		Schema:      types.StringSchema(),
	}

	namespaceParameter = &types.Parameter{
		Name:        "namespace",
		In:          "query",
		Schema:      types.StringSchema(),
		Description: "Specifies the Consul namespace of resources to operate on. This parameter takes precedence over the `ns` alias.",
	}

	nsParameter = &types.Parameter{
		Name:        "ns",
		In:          "query",
		Schema:      types.StringSchema(),
		Description: "`ns` is an alias for the `namespace` query param. The `namespace` parameter takes precedence.",
	}

	peerParameter = &types.Parameter{
		Name:        "peer",
		In:          "query",
		Schema:      types.StringSchema(),
		Description: "Specifies the Consul peer of imported resources to operate on.",
	}

	partitionParameter = &types.Parameter{
		Name:        "partition",
		In:          "query",
		Schema:      types.StringSchema(),
		Description: "Specifies the Consul partition of resources to operate on.",
	}

	nameParameter = &types.Parameter{
		Name:        "name",
		In:          "path",
		Schema:      types.StringSchema(),
		Description: "The name of the resource to operate on.",
		Required:    true,
	}

	allParameters = map[string]*types.Parameter{
		"namespace":   namespaceParameter,
		"ns":          nsParameter,
		"peer":        peerParameter,
		"partition":   partitionParameter,
		"name":        nameParameter,
		"name_prefix": namePrefixParameter,
		"consistent":  consistentParameter,
	}

	listParamsFromScope = map[pbresource.Scope]map[string]*types.Parameter{
		pbresource.Scope_SCOPE_UNDEFINED: {
			"peer":        peerParameter,
			"consistent":  consistentParameter,
			"name_prefix": namePrefixParameter,
			"namespace":   namespaceParameter,
			"ns":          nsParameter,
			"partition":   partitionParameter,
		},
		pbresource.Scope_SCOPE_CLUSTER: {
			"peer":        peerParameter,
			"consistent":  consistentParameter,
			"name_prefix": namePrefixParameter,
		},
		pbresource.Scope_SCOPE_PARTITION: {
			"peer":        peerParameter,
			"consistent":  consistentParameter,
			"name_prefix": namePrefixParameter,
			"partition":   partitionParameter,
		},
		pbresource.Scope_SCOPE_NAMESPACE: {
			"peer":        peerParameter,
			"consistent":  consistentParameter,
			"name_prefix": namePrefixParameter,
			"namespace":   namespaceParameter,
			"ns":          nsParameter,
			"partition":   partitionParameter,
		},
	}

	instanceOpParamsFromScope = map[pbresource.Scope]map[string]*types.Parameter{
		pbresource.Scope_SCOPE_UNDEFINED: {
			"peer":      peerParameter,
			"name":      namePrefixParameter,
			"namespace": namespaceParameter,
			"ns":        nsParameter,
			"partition": partitionParameter,
		},
		pbresource.Scope_SCOPE_CLUSTER: {
			"peer": peerParameter,
			"name": namePrefixParameter,
		},
		pbresource.Scope_SCOPE_PARTITION: {
			"peer":      peerParameter,
			"name":      namePrefixParameter,
			"partition": partitionParameter,
		},
		pbresource.Scope_SCOPE_NAMESPACE: {
			"peer":      peerParameter,
			"name":      namePrefixParameter,
			"namespace": namespaceParameter,
			"ns":        nsParameter,
			"partition": partitionParameter,
		},
	}

	readConsistencyParams = map[string]*types.Parameter{
		"consistent": consistentParameter,
	}

	listParamRefsFromScope       map[pbresource.Scope][]*types.Parameter
	instanceOpParamRefsFromScope map[pbresource.Scope][]*types.Parameter
	readConsistencyParamRefs     []*types.Parameter

	securitySchemes = map[string]*types.SecurityScheme{
		"ConsulTokenHeader": {
			Type: "apiKey",
			In:   "header",
			Name: "X-Consul-Token",
		},
		"BearerAuth": {
			Type:   "http",
			Scheme: "bearer",
		},
	}

	security []map[string][]string
)

func createParamRefs(params map[string]*types.Parameter) []*types.Parameter {
	var result []*types.Parameter
	for paramName, param := range params {
		if param.Name == "" {
			param.Name = paramName
		}

		result = append(result, &types.Parameter{Ref: paramRef(paramName)})
	}
	return result
}

func createParamRefsForScopes(scopes map[pbresource.Scope]map[string]*types.Parameter) map[pbresource.Scope][]*types.Parameter {
	result := make(map[pbresource.Scope][]*types.Parameter)
	for scope, params := range scopes {
		result[scope] = createParamRefs(params)
	}
	return result
}

func init() {
	// validation that the protoc generator code can handle all the relevant resource scopes.
	for _, scope := range pbresource.Scope_value {
		_, ok := listParamsFromScope[pbresource.Scope(scope)]
		if !ok {
			panic(fmt.Sprintf("openapi generator needs modification to support a new resource scope: %s (list)", pbresource.Scope_name[scope]))
		}

		_, ok = instanceOpParamsFromScope[pbresource.Scope(scope)]
		if !ok {
			panic(fmt.Sprintf("openapi generator needs modification to support a new resource scope: %s (instance op)", pbresource.Scope_name[scope]))
		}
	}

	listParamRefsFromScope = createParamRefsForScopes(listParamsFromScope)
	instanceOpParamRefsFromScope = createParamRefsForScopes(instanceOpParamsFromScope)
	readConsistencyParamRefs = createParamRefs(readConsistencyParams)

	for securityReq := range securitySchemes {
		security = append(security, map[string][]string{securityReq: {}})
	}
}

func (g *generator) generateTypedResourceSchema(rsc *resourceKind) *types.Schema {
	s := &types.Schema{
		Type:       "object",
		Properties: make(map[string]*types.Schema),
	}

	for name, prop := range g.resourceProperties {
		s.Properties[name] = prop
	}

	// overwrite the "data" field with the schema of the actual type
	s.Properties["data"] = &types.Schema{
		Ref: rsc.dataTypeRef,
	}

	return s
}

func (g *generator) generatePathsForResource(rsc *resourceKind) map[string]types.Path {
	kindSchema := g.generateTypedResourceSchema(rsc)

	paths := make(map[string]types.Path)

	paths[multiPath(rsc)] = types.Path{
		Get: &types.Operation{
			Summary:     listSummary(rsc),
			OperationID: listId(rsc),
			Parameters:  listParamRefsFromScope[rsc.scope],
			Responses: map[string]types.Response{
				"200": {
					Description: "The listing was successful and the body contains the array of results.",
					Content: map[string]types.Content{
						"application/json": {
							Schema: &types.Schema{
								Type:  "array",
								Items: kindSchema,
							},
						},
					},
				},
			},
		},
	}

	paths[singlePath(rsc)] = types.Path{
		Parameters: instanceOpParamRefsFromScope[rsc.scope],
		Get: &types.Operation{
			Summary:     readSummary(rsc),
			OperationID: readId(rsc),
			Parameters:  readConsistencyParamRefs,
			Responses: map[string]types.Response{
				"200": {
					Description: "The read was successful and the body contains the result.",
					Content: map[string]types.Content{
						"application/json": {
							Schema: kindSchema,
						},
					},
				},
			},
		},
		Put: &types.Operation{
			Summary:     writeSummary(rsc),
			OperationID: writeId(rsc),
			RequestBody: &types.RequestBody{
				Description: writeBodyDescription(rsc),
				Content: map[string]types.Content{
					"application/json": {
						Schema: kindSchema,
					},
				},
			},
			Responses: map[string]types.Response{
				"200": {
					Description: "The write was successful and the body contains the result.",
					Content: map[string]types.Content{
						"application/json": {
							Schema: kindSchema,
						},
					},
				},
			},
		},
		Delete: &types.Operation{
			Summary:     deleteSummary(rsc),
			OperationID: deleteId(rsc),
			Responses: map[string]types.Response{
				"200": {
					Description: "The delete was successful and the body contains the result.",
				},
			},
		},
	}

	return paths
}

func multiPath(rsc *resourceKind) string {
	return fmt.Sprintf("/%s/%s/%s", rsc.group, rsc.version, rsc.kind)
}

func listSummary(rsc *resourceKind) string {
	return fmt.Sprintf("List %s.%s.%s resources", rsc.group, rsc.version, rsc.kind)
}

func listId(rsc *resourceKind) string {
	return fmt.Sprintf("list-%s", rsc.kind)
}

func singlePath(rsc *resourceKind) string {
	return fmt.Sprintf("/%s/%s/%s/{name}", rsc.group, rsc.version, rsc.kind)
}

func readSummary(rsc *resourceKind) string {
	return fmt.Sprintf("Read %s.%s.%s resources", rsc.group, rsc.version, rsc.kind)
}

func readId(rsc *resourceKind) string {
	return fmt.Sprintf("read-%s.", rsc.kind)
}

func writeSummary(rsc *resourceKind) string {
	return fmt.Sprintf("Write %s.%s.%s resources", rsc.group, rsc.version, rsc.kind)
}

func writeId(rsc *resourceKind) string {
	return fmt.Sprintf("write-%s", rsc.kind)
}

func writeBodyDescription(rsc *resourceKind) string {
	return fmt.Sprintf("The %s.%s.%s resource to be updated.", rsc.group, rsc.version, rsc.kind)
}

func deleteSummary(rsc *resourceKind) string {
	return fmt.Sprintf("Delete %s.%s.%s resources", rsc.group, rsc.version, rsc.kind)
}

func deleteId(rsc *resourceKind) string {
	return fmt.Sprintf("delete-%s", rsc.kind)
}
