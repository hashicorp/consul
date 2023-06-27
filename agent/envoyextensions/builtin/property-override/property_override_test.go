package propertyoverride

import (
	"fmt"
	"strings"
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

// TestConstructor tests raw input to the Constructor function called to initialize
// the property-override extension. This includes implicit validation of the deserialized
// input to the extension.
func TestConstructor(t *testing.T) {
	// These helpers aid in constructing valid raw example input (map[string]any)
	// for the Constructor, with optional overrides for fields under test.
	applyOverrides := func(m map[string]any, overrides map[string]any) map[string]any {
		for k, v := range overrides {
			if v == nil {
				delete(m, k)
			} else {
				m[k] = v
			}
		}
		return m
	}
	makeResourceFilter := func(overrides map[string]any) map[string]any {
		f := map[string]any{
			"ResourceType":     ResourceTypeRoute,
			"TrafficDirection": extensioncommon.TrafficDirectionOutbound,
		}
		return applyOverrides(f, overrides)
	}
	makePatch := func(overrides map[string]any) map[string]any {
		p := map[string]any{
			"ResourceFilter": makeResourceFilter(map[string]any{}),
			"Op":             OpAdd,
			"Path":           "/name",
			"Value":          "foo",
		}
		return applyOverrides(p, overrides)
	}
	makeArguments := func(overrides map[string]any) map[string]any {
		a := map[string]any{
			"Patches": []map[string]any{
				makePatch(map[string]any{}),
			},
			"Debug":     true,
			"ProxyType": api.ServiceKindConnectProxy,
		}
		return applyOverrides(a, overrides)
	}
	type testCase struct {
		extensionName string
		arguments     map[string]any
		expected      propertyOverride
		ok            bool
		errMsg        string
		errFunc       func(*testing.T, error)
	}

	validTestCase := func(o Op, d extensioncommon.TrafficDirection, t ResourceType) testCase {
		var v any = "foo"
		if o != OpAdd {
			v = nil
		}

		// Use a valid field for all resource types.
		path := "/name"
		if t == ResourceTypeClusterLoadAssignment {
			path = "/cluster_name"
		}

		return testCase{
			arguments: makeArguments(map[string]any{
				"Patches": []map[string]any{
					makePatch(map[string]any{
						"ResourceFilter": makeResourceFilter(map[string]any{
							"ResourceType":     t,
							"TrafficDirection": d,
						}),
						"Op":    o,
						"Path":  path,
						"Value": v,
					}),
				},
			}),
			expected: propertyOverride{
				Patches: []Patch{
					{
						ResourceFilter: ResourceFilter{
							ResourceType:     t,
							TrafficDirection: d,
						},
						Op:    o,
						Path:  path,
						Value: v,
					},
				},
				Debug:     true,
				ProxyType: api.ServiceKindConnectProxy,
			},
			ok: true,
		}
	}
	cases := map[string]testCase{
		"with no arguments": {
			arguments: nil,
			ok:        false,
			errMsg:    "at least one patch is required",
		},
		"with an invalid name": {
			arguments:     makeArguments(map[string]any{}),
			extensionName: "bad",
			ok:            false,
			errMsg:        "expected extension name \"builtin/property-override\" but got \"bad\"",
		},
		"empty Patches": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{}}),
			ok:        false,
			errMsg:    "at least one patch is required",
		},
		"patch with no ResourceFilter": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": nil,
				}),
			}}),
			ok:     false,
			errMsg: "field ResourceFilter is required",
		},
		"patch with no ResourceType": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": makeResourceFilter(map[string]any{
						"ResourceType": nil,
					}),
				}),
			}}),
			ok:     false,
			errMsg: "field ResourceType is required",
		},
		"patch with invalid ResourceType": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": makeResourceFilter(map[string]any{
						"ResourceType": "foo",
					}),
				}),
			}}),
			ok:     false,
			errMsg: "invalid ResourceType",
		},
		"patch with no TrafficDirection": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": makeResourceFilter(map[string]any{
						"TrafficDirection": nil,
					}),
				}),
			}}),
			ok:     false,
			errMsg: "field TrafficDirection is required",
		},
		"patch with invalid TrafficDirection": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": makeResourceFilter(map[string]any{
						"TrafficDirection": "foo",
					}),
				}),
			}}),
			ok:     false,
			errMsg: "invalid TrafficDirection",
		},
		"patch with no Op": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"Op": nil,
				}),
			}}),
			ok:     false,
			errMsg: "field Op is required",
		},
		"patch with invalid Op": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"Op": "foo",
				}),
			}}),
			ok:     false,
			errMsg: "invalid Op",
		},
		"patch with invalid Envoy resource Path": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"Path": "/invalid",
				}),
			}}),
			ok:     false,
			errMsg: "no match for field", // this error comes from the patcher dry-run attempt
		},
		"non-Add patch with Value": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"Op":    OpRemove,
					"Value": 1,
				}),
			}}),
			ok:     false,
			errMsg: fmt.Sprintf("field Value is not supported for %s operation", OpRemove),
		},
		"multiple patches includes indexed errors": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"Op":    OpRemove,
					"Value": 0,
				}),
				makePatch(map[string]any{
					"Op":    OpAdd,
					"Value": nil,
				}),
				makePatch(map[string]any{
					"Op":   OpAdd,
					"Path": "/foo",
				}),
			}}),
			ok: false,
			errFunc: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "invalid Patches[0]: field Value is not supported for remove operation")
				require.ErrorContains(t, err, "invalid Patches[1]: non-nil Value is required")
				require.ErrorContains(t, err, "invalid Patches[2]: no match for field 'foo'")
			},
		},
		"multiple patches single error contains correct index": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"Op":    OpAdd,
					"Value": "foo",
				}),
				makePatch(map[string]any{
					"Op":    OpRemove,
					"Value": 1,
				}),
				makePatch(map[string]any{
					"Op":    OpAdd,
					"Value": "bar",
				}),
			}}),
			ok: false,
			errFunc: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "invalid Patches[1]: field Value is not supported for remove operation")
				require.NotContains(t, err.Error(), "invalid Patches[0]")
				require.NotContains(t, err.Error(), "invalid Patches[2]")
			},
		},
		"empty service name": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": makeResourceFilter(map[string]any{
						"Services": []map[string]any{
							{},
						},
					}),
				}),
			}}),
			ok:     false,
			errMsg: "service name is required",
		},
		"non-empty services with invalid traffic direction": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": makeResourceFilter(map[string]any{
						"TrafficDirection": extensioncommon.TrafficDirectionInbound,
						"Services": []map[string]any{
							{"Name:": "foo"},
						},
					}),
				}),
			}}),
			ok:     false,
			errMsg: "patch contains non-empty ResourceFilter.Services but ResourceFilter.TrafficDirection is not \"outbound\"",
		},
		// See decode.HookWeakDecodeFromSlice for more details. In practice, we can end up
		// with a "Patches" field decoded to the single "Patch" value contained in the
		// serialized slice (raised from the containing slice). Using WeakDecode solves
		// for this. Ideally, we would kill that decoding hook entirely, but this test
		// enforces expected behavior until we do. Multi-member slices should be unaffected
		// by WeakDecode as it is a more-permissive version of the default behavior.
		"single value Patches decoded as map construction succeeds": {
			arguments: makeArguments(map[string]any{"Patches": makePatch(map[string]any{}), "ProxyType": nil}),
			expected:  validTestCase(OpAdd, extensioncommon.TrafficDirectionOutbound, ResourceTypeRoute).expected,
			ok:        true,
		},
		// Ensure that embedded api struct used for Services is parsed correctly.
		// See also above comment on decode.HookWeakDecodeFromSlice.
		"single value Services decoded as map construction succeeds": {
			arguments: makeArguments(map[string]any{"Patches": []map[string]any{
				makePatch(map[string]any{
					"ResourceFilter": makeResourceFilter(map[string]any{
						"Services": []map[string]any{
							{"Name": "foo"},
						},
					}),
				}),
			}}),
			expected: propertyOverride{
				Patches: []Patch{
					{
						ResourceFilter: ResourceFilter{
							ResourceType:     ResourceTypeRoute,
							TrafficDirection: extensioncommon.TrafficDirectionOutbound,
							Services: []*ServiceName{
								{CompoundServiceName: api.CompoundServiceName{
									Name:      "foo",
									Namespace: "default",
									Partition: "default",
								}},
							},
						},
						Op:    OpAdd,
						Path:  "/name",
						Value: "foo",
					},
				},
				Debug:     true,
				ProxyType: api.ServiceKindConnectProxy,
			},
			ok: true,
		},
		"invalid ProxyType": {
			arguments: makeArguments(map[string]any{
				"Patches": []map[string]any{
					makePatch(map[string]any{}),
				},
				"ProxyType": "invalid",
			}),
			ok:     false,
			errMsg: "invalid ProxyType",
		},
		"unsupported ProxyType": {
			arguments: makeArguments(map[string]any{
				"Patches": []map[string]any{
					makePatch(map[string]any{}),
				},
				"ProxyType": api.ServiceKindMeshGateway,
			}),
			ok:     false,
			errMsg: "invalid ProxyType",
		},
	}

	for o := range Ops {
		for d := range extensioncommon.TrafficDirections {
			for t := range ResourceTypes {
				cases["valid everything: "+strings.Join([]string{o, d, t}, ",")] =
					validTestCase(Op(o), extensioncommon.TrafficDirection(d), ResourceType(t))
			}
		}
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			extensionName := api.BuiltinPropertyOverrideExtension
			if tc.extensionName != "" {
				extensionName = tc.extensionName
			}

			// Build the wrapping RuntimeConfig struct, which contains the serialized
			// arguments for constructing the property-override extension.
			svc := api.CompoundServiceName{Name: "svc"}
			ext := extensioncommon.RuntimeConfig{
				ServiceName: svc,
				EnvoyExtension: api.EnvoyExtension{
					Name:      extensionName,
					Arguments: tc.arguments,
				},
			}

			// Construct the actual extension
			e, err := Constructor(ext.EnvoyExtension)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, &extensioncommon.BasicEnvoyExtender{Extension: &tc.expected}, e)
			} else {
				require.Error(t, err)
				if tc.errMsg != "" {
					require.ErrorContains(t, err, tc.errMsg)
				}
				if tc.errFunc != nil {
					tc.errFunc(t, err)
				}
			}
		})
	}
}

func Test_patchResourceType(t *testing.T) {
	makeExtension := func(patches ...Patch) *propertyOverride {
		return &propertyOverride{
			Patches: patches,
		}
	}
	makePatchWithPath := func(filter ResourceFilter, p string) Patch {
		return Patch{
			ResourceFilter: filter,
			Op:             OpAdd,
			Path:           p,
			Value:          1,
		}
	}
	makePatch := func(filter ResourceFilter) Patch {
		return makePatchWithPath(filter, "/foo")
	}

	svc1 := ServiceName{
		CompoundServiceName: api.CompoundServiceName{Name: "svc1"},
	}
	svc2 := ServiceName{
		CompoundServiceName: api.CompoundServiceName{Name: "svc2"},
	}

	clusterOutbound := makePatch(ResourceFilter{
		ResourceType:     ResourceTypeCluster,
		TrafficDirection: extensioncommon.TrafficDirectionOutbound,
	})
	clusterInbound := makePatch(ResourceFilter{
		ResourceType:     ResourceTypeCluster,
		TrafficDirection: extensioncommon.TrafficDirectionInbound,
	})
	listenerOutbound := makePatch(ResourceFilter{
		ResourceType:     ResourceTypeListener,
		TrafficDirection: extensioncommon.TrafficDirectionOutbound,
	})
	listenerOutbound2 := makePatchWithPath(ResourceFilter{
		ResourceType:     ResourceTypeListener,
		TrafficDirection: extensioncommon.TrafficDirectionOutbound,
	}, "/bar")
	listenerInbound := makePatch(ResourceFilter{
		ResourceType:     ResourceTypeListener,
		TrafficDirection: extensioncommon.TrafficDirectionInbound,
	})
	routeOutbound := makePatch(ResourceFilter{
		ResourceType:     ResourceTypeRoute,
		TrafficDirection: extensioncommon.TrafficDirectionOutbound,
	})
	routeOutbound2 := makePatchWithPath(ResourceFilter{
		ResourceType:     ResourceTypeRoute,
		TrafficDirection: extensioncommon.TrafficDirectionOutbound,
	}, "/bar")
	routeInbound := makePatch(ResourceFilter{
		ResourceType:     ResourceTypeRoute,
		TrafficDirection: extensioncommon.TrafficDirectionInbound,
	})

	type args struct {
		resourceType ResourceType
		payload      extensioncommon.Payload[proto.Message]
		p            *propertyOverride
	}
	type testCase struct {
		args          args
		expectPatched bool
		wantApplied   []Patch
	}
	cases := map[string]testCase{
		"outbound gets matching patch": {
			args: args{
				resourceType: ResourceTypeCluster,
				payload: extensioncommon.Payload[proto.Message]{
					TrafficDirection: extensioncommon.TrafficDirectionOutbound,
					Message:          &clusterv3.Cluster{},
				},
				p: makeExtension(clusterOutbound),
			},
			expectPatched: true,
			wantApplied:   []Patch{clusterOutbound},
		},
		"inbound gets matching patch": {
			args: args{
				resourceType: ResourceTypeCluster,
				payload: extensioncommon.Payload[proto.Message]{
					TrafficDirection: extensioncommon.TrafficDirectionInbound,
					Message:          &clusterv3.Cluster{},
				},
				p: makeExtension(clusterInbound),
			},
			expectPatched: true,
			wantApplied:   []Patch{clusterInbound},
		},
		"multiple resources same direction only gets matching resource": {
			args: args{
				resourceType: ResourceTypeCluster,
				payload: extensioncommon.Payload[proto.Message]{
					TrafficDirection: extensioncommon.TrafficDirectionOutbound,
					Message:          &clusterv3.Cluster{},
				},
				p: makeExtension(clusterOutbound, listenerOutbound),
			},
			expectPatched: true,
			wantApplied:   []Patch{clusterOutbound},
		},
		"multiple directions same resource only gets matching direction": {
			args: args{
				resourceType: ResourceTypeCluster,
				payload: extensioncommon.Payload[proto.Message]{
					TrafficDirection: extensioncommon.TrafficDirectionOutbound,
					Message:          &clusterv3.Cluster{},
				},
				p: makeExtension(clusterOutbound, clusterInbound),
			},
			expectPatched: true,
			wantApplied:   []Patch{clusterOutbound},
		},
		"multiple directions and resources only gets matching patch": {
			args: args{
				resourceType: ResourceTypeRoute,
				payload: extensioncommon.Payload[proto.Message]{
					TrafficDirection: extensioncommon.TrafficDirectionInbound,
					Message:          &routev3.RouteConfiguration{},
				},
				p: makeExtension(clusterOutbound, clusterInbound, listenerOutbound, listenerInbound, routeOutbound, routeOutbound2, routeInbound),
			},
			expectPatched: true,
			wantApplied:   []Patch{routeInbound},
		},
		"multiple directions and resources multiple matches gets all matching patches": {
			args: args{
				resourceType: ResourceTypeRoute,
				payload: extensioncommon.Payload[proto.Message]{
					TrafficDirection: extensioncommon.TrafficDirectionOutbound,
					Message:          &routev3.RouteConfiguration{},
				},
				p: makeExtension(clusterOutbound, clusterInbound, listenerOutbound, listenerInbound, listenerOutbound2, routeOutbound, routeOutbound2, routeInbound),
			},
			expectPatched: true,
			wantApplied:   []Patch{routeOutbound, routeOutbound2},
		},
		"multiple directions and resources no matches gets no patches": {
			args: args{
				resourceType: ResourceTypeCluster,
				payload: extensioncommon.Payload[proto.Message]{
					TrafficDirection: extensioncommon.TrafficDirectionOutbound,
					Message:          &clusterv3.Cluster{},
				},
				p: makeExtension(clusterInbound, listenerOutbound, listenerInbound, listenerOutbound2, routeInbound, routeOutbound),
			},
			expectPatched: false,
			wantApplied:   nil,
		},
	}

	type resourceTypeServiceMatch struct {
		resourceType ResourceType
		message      proto.Message
	}

	resourceTypeCases := []resourceTypeServiceMatch{
		{
			resourceType: ResourceTypeCluster,
			message:      &clusterv3.Cluster{},
		},
		{
			resourceType: ResourceTypeListener,
			message:      &listenerv3.Listener{},
		},
		{
			resourceType: ResourceTypeRoute,
			message:      &routev3.RouteConfiguration{},
		},
		{
			resourceType: ResourceTypeClusterLoadAssignment,
			message:      &endpointv3.ClusterLoadAssignment{},
		},
	}

	for _, tc := range resourceTypeCases {
		{
			patch := makePatch(ResourceFilter{
				ResourceType:     tc.resourceType,
				TrafficDirection: extensioncommon.TrafficDirectionOutbound,
				Services: []*ServiceName{
					{CompoundServiceName: svc2.CompoundServiceName},
				},
			})

			cases[fmt.Sprintf("%s - no match", tc.resourceType)] = testCase{
				args: args{
					resourceType: tc.resourceType,
					payload: extensioncommon.Payload[proto.Message]{
						TrafficDirection: extensioncommon.TrafficDirectionOutbound,
						ServiceName:      &svc1.CompoundServiceName,
						Message:          tc.message,
						RuntimeConfig: &extensioncommon.RuntimeConfig{
							Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
								svc1.CompoundServiceName: {},
							},
						},
					},
					p: makeExtension(patch),
				},
				expectPatched: false,
				wantApplied:   nil,
			}
		}

		{
			patch := makePatch(ResourceFilter{
				ResourceType:     tc.resourceType,
				TrafficDirection: extensioncommon.TrafficDirectionOutbound,
				Services: []*ServiceName{
					{CompoundServiceName: svc2.CompoundServiceName},
					{CompoundServiceName: svc1.CompoundServiceName},
				},
			})

			cases[fmt.Sprintf("%s - match", tc.resourceType)] = testCase{
				args: args{
					resourceType: tc.resourceType,
					payload: extensioncommon.Payload[proto.Message]{
						TrafficDirection: extensioncommon.TrafficDirectionOutbound,
						ServiceName:      &svc1.CompoundServiceName,
						Message:          tc.message,
						RuntimeConfig: &extensioncommon.RuntimeConfig{
							Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
								svc1.CompoundServiceName: {},
							},
						},
					},
					p: makeExtension(patch),
				},
				expectPatched: true,
				wantApplied:   []Patch{patch},
			}
		}
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			mockPatcher := MockPatcher[proto.Message]{}
			_, patched, err := patchResourceType[proto.Message](tc.args.p, tc.args.resourceType, tc.args.payload, &mockPatcher)

			require.NoError(t, err, "unexpected error from mock")
			require.Equal(t, tc.expectPatched, patched)
			require.Equal(t, tc.wantApplied, mockPatcher.appliedPatches)
		})
	}
}

type MockPatcher[K proto.Message] struct {
	appliedPatches []Patch
}

//nolint:unparam
func (m *MockPatcher[K]) applyPatch(k K, p Patch, _ bool) (result K, e error) {
	m.appliedPatches = append(m.appliedPatches, p)
	return k, nil
}

func TestCanApply(t *testing.T) {
	cases := map[string]struct {
		ext      *propertyOverride
		conf     *extensioncommon.RuntimeConfig
		canApply bool
	}{
		"valid proxy type": {
			ext: &propertyOverride{
				ProxyType: api.ServiceKindConnectProxy,
			},
			conf: &extensioncommon.RuntimeConfig{
				Kind: api.ServiceKindConnectProxy,
			},
			canApply: true,
		},
		"invalid proxy type": {
			ext: &propertyOverride{
				ProxyType: api.ServiceKindConnectProxy,
			},
			conf: &extensioncommon.RuntimeConfig{
				Kind: api.ServiceKindMeshGateway,
			},
			canApply: false,
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			require.Equal(t, tc.canApply, tc.ext.CanApply(tc.conf))
		})
	}
}
