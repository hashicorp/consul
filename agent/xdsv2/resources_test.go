// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"
	"sort"
	"testing"

	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/stretchr/testify/suite"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	"github.com/hashicorp/consul/proto-public/pbresource"

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

type resourceTestSuite struct {
	suite.Suite
	tenancies []*pbresource.Tenancy
}

var testTypeUrlToPrettyName = map[string]string{
	xdscommon.ListenerType: "listeners",
	xdscommon.RouteType:    "routes",
	xdscommon.ClusterType:  "clusters",
	xdscommon.EndpointType: "endpoints",
	xdscommon.SecretType:   "secrets",
}

func TestResources(t *testing.T) {
	suite.Run(t, new(resourceTestSuite))
}

func (suite *resourceTestSuite) SetupTest() {
	suite.tenancies = resourcetest.TestTenancies()
}

// TestAllResourcesFromIR_XDSGoldenFileInputs tests the AllResourcesFromIR() by
// using the golden test output/expected files from the XDS controller tests as
// inputs to the XDSV2 resources generation.
func (suite *resourceTestSuite) TestAllResourcesFromIR_XDSGoldenFileInputs() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
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
			"source/l7-expose-paths",
			"source/local-and-inbound-connections",
			"source/multiple-workload-addresses-with-specific-ports",
			"source/multiple-workload-addresses-without-ports",
			"source/multiport-l4-multiple-workload-addresses-with-specific-ports",
			"source/multiport-l4-multiple-workload-addresses-without-ports",
			"source/multiport-l4-workload-with-only-mesh-port",
			"source/multiport-l7-multiple-workload-addresses-with-specific-ports",
			"source/multiport-l7-multiple-workload-addresses-without-ports",
			"source/single-workload-address-without-ports",
		}

		for _, name := range cases {
			suite.Run(name, func() {
				// Arrange - paths to input and output golden files.
				testFile := fmt.Sprintf("%s-%s-%s.golden", name, tenancy.Partition, tenancy.Namespace)
				inputFilePath := fmt.Sprintf("%s/testdata/%s", inputPath, testFile)
				inputValueInput := golden.GetBytesAtFilePath(suite.T(), inputFilePath)

				// Act.
				ps := jsonToProxyState(suite.T(), inputValueInput)
				generator := NewResourceGenerator(testutil.Logger(suite.T()))
				resources, err := generator.AllResourcesFromIR(&proxytracker.ProxyState{ProxyState: ps})
				require.NoError(suite.T(), err)

				// Assert.
				// Assert all resources were generated.
				typeUrls := []string{
					xdscommon.ListenerType,
					xdscommon.RouteType,
					xdscommon.ClusterType,
					xdscommon.EndpointType,
					// TODO(proxystate): add in future
					//xdscommon.SecretType,
				}
				require.Len(suite.T(), resources, len(typeUrls))

				// Assert each resource type has actual XDS matching expected XDS.
				for _, typeUrl := range typeUrls {
					prettyName := testTypeUrlToPrettyName[typeUrl]
					suite.T().Run(prettyName, func(t *testing.T) {
						items, ok := resources[typeUrl]
						require.True(t, ok)

						// sort resources so they don't show up as flakey tests as
						// ordering in JSON is not guaranteed.
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

						// Compare actual to expected.
						resp, err := response.CreateResponse(typeUrl, "00000001", "00000001", items)
						require.NoError(t, err)
						gotJSON := protoToJSON(t, resp)

						expectedJSON := golden.Get(t, gotJSON, fmt.Sprintf("%s/%s", prettyName, testFile))
						require.JSONEq(t, expectedJSON, gotJSON)
					})
				}
			})
		}
	})
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

func (suite *resourceTestSuite) runTestCaseWithTenancies(testCase func(tenancy *pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			testCase(tenancy)
		})
	}
}

func (suite *resourceTestSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}
