package validateupstream

import (
	"io"
	"os"
	"testing"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/hashicorp/consul/agent/xds/xdscommon"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

// TODO make config.json and clusters.json use an http upstream with L7 config entries for more confidence.
func TestValidate(t *testing.T) {
	indexedResources := getConfig(t)
	clusters := getClusters(t)
	err := Validate(indexedResources, clusters, service, "", "")
	require.NoError(t, err)
}

func getConfig(t *testing.T) *xdscommon.IndexedResources {
	file, err := os.Open("testdata/config.json")
	require.NoError(t, err)
	jsonBytes, err := io.ReadAll(file)
	require.NoError(t, err)
	indexedResources, err := ParseConfigDump(jsonBytes)
	require.NoError(t, err)
	return indexedResources
}

func getClusters(t *testing.T) *envoy_admin_v3.Clusters {
	file, err := os.Open("testdata/clusters.json")
	require.NoError(t, err)
	jsonBytes, err := io.ReadAll(file)
	require.NoError(t, err)
	clusters, err := ParseClusters(jsonBytes)
	require.NoError(t, err)
	return clusters
}

var service = api.CompoundServiceName{
	Name: "backend",
}
