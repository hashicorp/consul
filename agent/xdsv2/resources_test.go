// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/hashicorp/consul/internal/testing/golden"
	"sort"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"

	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	proxytracker "github.com/hashicorp/consul/internal/mesh/proxy-tracker"
	meshv2beta1 "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/sdk/testutil"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var testTypeUrlToPrettyName = map[string]string{
	xdscommon.ListenerType: "listeners",
	xdscommon.RouteType:    "routes",
	xdscommon.ClusterType:  "clusters",
	xdscommon.EndpointType: "endpoints",
	xdscommon.SecretType:   "secrets",
}

// TestReconcile_SidecarProxyGoldenFileInputs tests the Reconcile() by using
// the golden test output/expected files from the sidecar proxy tests as inputs
// to the XDS controller reconciliation.
// XDS controller reconciles the full ProxyStateTemplate object.  The fields
// that things that it focuses on are leaf certs, endpoints, and trust bundles,
// which is just a subset of the ProxyStateTemplate struct.  Prior to XDS controller
// reconciliation, the sidecar proxy controller will have reconciled the other parts
// of the ProxyStateTemplate.
// Since the XDS controller does act on the ProxyStateTemplate, the tests
// utilize that entire object rather than just the parts that XDS controller
// internals reconciles.  Namely, by using checking the full ProxyStateTemplate
// rather than just endpoints, leaf certs, and trust bundles, the test also ensures
// side effects or change in scope to XDS controller are not introduce mistakenly.
func TestAllResourcesFromIR_XDSGoldenFileInputs(t *testing.T) {
	inputPath := "../../internal/mesh/internal/controllers/xds"

	cases := []string{
		// destinations - please add in alphabetical order
		"destination/l4-single-destination-ip-port-bind-address",
		"destination/l4-single-destination-unix-socket-bind-address",
		"destination/l4-single-implicit-destination-tproxy",
		"destination/l4-multi-destination",
		"destination/l4-multiple-implicit-destinations-tproxy",
		"destination/l4-implicit-and-explicit-destinations-tproxy",
		"destination/mixed-multi-destination",
		"destination/multiport-l4-and-l7-multiple-implicit-destinations-tproxy",
		"destination/multiport-l4-and-l7-single-implicit-destination-tproxy",
		"destination/multiport-l4-and-l7-single-implicit-destination-with-multiple-workloads-tproxy",

		//sources - please add in alphabetical order
		//"source/l4-multiple-workload-addresses-with-specific-ports",
		//"source/l4-multiple-workload-addresses-without-ports",
		//"source/l4-single-workload-address-without-ports",
		//"source/l7-expose-paths",
		//"source/local-and-inbound-connections",
		//"source/multiport-l4-multiple-workload-addresses-with-specific-ports",
		//"source/multiport-l4-multiple-workload-addresses-without-ports",
		//"source/multiport-l4-workload-with-only-mesh-port",
		//"source/multiport-l7-multiple-workload-addresses-with-specific-ports",
		//"source/multiport-l7-multiple-workload-addresses-without-ports",
		//"source/multiport-l7-multiple-workload-addresses-without-ports",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			testFile := fmt.Sprintf("%s.golden", name)
			inputFilePath := fmt.Sprintf("%s/testdata/%s", inputPath, testFile)
			inputValueInput := golden.GetBytesAtFilePath(t, inputFilePath)

			ps := jsonToProxyState(t, inputValueInput)
			generator := NewResourceGenerator(testutil.Logger(t))

			resources, err := generator.AllResourcesFromIR(&proxytracker.ProxyState{ProxyState: ps})
			require.NoError(t, err)

			require.NoError(t, err)

			typeUrls := []string{
				xdscommon.ListenerType,
				xdscommon.RouteType,
				xdscommon.ClusterType,
				xdscommon.EndpointType,
				// TODO(proxystate): add in future
				//xdscommon.SecretType,
			}
			require.Len(t, resources, len(typeUrls))

			for _, typeUrl := range typeUrls {
				prettyName := testTypeUrlToPrettyName[typeUrl]
				t.Run(prettyName, func(t *testing.T) {
					items, ok := resources[typeUrl]
					require.True(t, ok)

					sort.Slice(items, func(i, j int) bool {
						switch typeUrl {
						case xdscommon.ListenerType:
							return items[i].(*envoy_listener_v3.Listener).Name < items[j].(*envoy_listener_v3.Listener).Name
						case xdscommon.RouteType:
							return items[i].(*envoy_route_v3.RouteConfiguration).Name < items[j].(*envoy_route_v3.RouteConfiguration).Name
						case xdscommon.ClusterType:
							return items[i].(*envoy_cluster_v3.Cluster).Name < items[j].(*envoy_cluster_v3.Cluster).Name
						case xdscommon.EndpointType:
							return items[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < items[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
						case xdscommon.SecretType:
							return items[i].(*envoy_tls_v3.Secret).Name < items[j].(*envoy_tls_v3.Secret).Name
						default:
							panic("not possible")
						}
					})

					resp, err := response.CreateResponse(typeUrl, "00000001", "00000001", items)
					require.NoError(t, err)

					gotJSON := protoToJSON(t, resp)

					expectedJSON := golden.Get(t, gotJSON, fmt.Sprintf("%s/%s", prettyName, testFile))
					require.JSONEq(t, expectedJSON, gotJSON)
				})
			}
		})
	}
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	t.Helper()
	m := protojson.MarshalOptions{
		Indent: "  ",
	}
	gotJSON, err := m.Marshal(pb)
	require.NoError(t, err)
	return string(gotJSON)
}

func jsonToProxyState(t *testing.T, json []byte) *meshv2beta1.ProxyState {
	t.Helper()
	um := protojson.UnmarshalOptions{}
	ps := &meshv2beta1.ProxyState{}
	err := um.Unmarshal(json, ps)
	require.NoError(t, err)
	return ps
}
