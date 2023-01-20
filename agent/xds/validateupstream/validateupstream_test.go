package validateupstream

import (
	"io"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/xds/xdscommon"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

// TODO ClusterLoadAssignment in the config dump is missing the cluster name, causing this validation to fail.
//func TestValidate(t *testing.T) {
//	indexedResources := getConfig(t)
//	err := Validate(indexedResources, service, "", "")
//	require.NoError(t, err)
//}

func getConfig(t *testing.T) *xdscommon.IndexedResources {
	// config.json currently does not have EndpointsConfigDump. But there's currently an error with this case described above.
	file, err := os.Open("testdata/config.json")
	require.NoError(t, err)
	jsonBytes, err := io.ReadAll(file)
	require.NoError(t, err)
	indexedResources, err := ParseConfig(jsonBytes)
	require.NoError(t, err)
	return indexedResources
}

var service = api.CompoundServiceName{
	Name: "s2",
}
