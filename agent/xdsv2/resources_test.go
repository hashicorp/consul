// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"os"
	"path/filepath"
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

func TestResources_ImplicitDestinations(t *testing.T) {

	cases := map[string]struct {
	}{
		"l4-single-implicit-destination-tproxy": {},
	}

	for name := range cases {
		goldenValueInput := goldenValueJSON(t, name, "input")

		proxyTemplate := jsonToProxyTemplate(t, goldenValueInput)
		generator := NewResourceGenerator(testutil.Logger(t))

		resources, err := generator.AllResourcesFromIR(&proxytracker.ProxyState{ProxyState: proxyTemplate.ProxyState})
		require.NoError(t, err)

		verifyClusterResourcesToGolden(t, resources, name)
		verifyListenerResourcesToGolden(t, resources, name)

	}
}

func verifyClusterResourcesToGolden(t *testing.T, resources map[string][]proto.Message, testName string) {
	clusters := resources[xdscommon.ClusterType]

	// The order of clusters returned via CDS isn't relevant, so it's safe
	// to sort these for the purposes of test comparisons.
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].(*envoy_cluster_v3.Cluster).Name < clusters[j].(*envoy_cluster_v3.Cluster).Name
	})

	resp, err := response.CreateResponse(xdscommon.ClusterType, "00000001", "00000001", clusters)
	require.NoError(t, err)
	gotJSON := protoToJSON(t, resp)

	expectedJSON := goldenValue(t, filepath.Join("clusters", testName), "output")
	require.JSONEq(t, expectedJSON, gotJSON)
}

func verifyListenerResourcesToGolden(t *testing.T, resources map[string][]proto.Message, testName string) {
	listeners := resources[xdscommon.ListenerType]

	// The order of clusters returned via CDS isn't relevant, so it's safe
	// to sort these for the purposes of test comparisons.
	sort.Slice(listeners, func(i, j int) bool {
		return listeners[i].(*envoy_listener_v3.Listener).Name < listeners[j].(*envoy_listener_v3.Listener).Name
	})

	resp, err := response.CreateResponse(xdscommon.ListenerType, "00000001", "00000001", listeners)
	require.NoError(t, err)
	gotJSON := protoToJSON(t, resp)

	expectedJSON := goldenValue(t, filepath.Join("listeners", testName), "output")
	require.JSONEq(t, expectedJSON, gotJSON)
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

func jsonToProxyTemplate(t *testing.T, json []byte) *meshv2beta1.ProxyStateTemplate {
	t.Helper()
	um := protojson.UnmarshalOptions{}
	proxyTemplate := &meshv2beta1.ProxyStateTemplate{}
	err := um.Unmarshal(json, proxyTemplate)
	require.NoError(t, err)
	return proxyTemplate
}

func goldenValueJSON(t *testing.T, goldenFile, inputOutput string) []byte {
	t.Helper()
	goldenPath := filepath.Join("testdata", inputOutput, goldenFile) + ".golden"

	content, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	return content
}

func goldenValue(t *testing.T, goldenFile, inputOutput string) string {
	t.Helper()
	return string(goldenValueJSON(t, goldenFile, inputOutput))
}
