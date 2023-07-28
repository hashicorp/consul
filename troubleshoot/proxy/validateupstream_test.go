package troubleshoot

import (
	"io"
	"os"
	"testing"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/stretchr/testify/require"
)

// A majority of unit tests for validateupstream.go are in the agent/xds/validateupstream-test package due to internal
// Consul dependencies that shouldn't be imported into the troubleshoot module. The tests that are here don't require
// internal consul packages.

func TestValidateFromJSON(t *testing.T) {
	indexedResources := getConfig(t)
	clusters := getClusters(t)
	messages := Validate(indexedResources, "backend", "", true, clusters)
	require.True(t, messages.Success())
}

// TODO: Manually inspect the config and clusters files and hardcode the list of expected resource names for higher
// confidence in these functions.
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
