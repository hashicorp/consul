package validateupstream

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/xds/xdscommon"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	indexedResources := getConfig(t)
	err := Validate(indexedResources, service)
	require.NoError(t, err)
}

func getConfig(t *testing.T) *xdscommon.IndexedResources {
	file, err := os.Open("testdata/config.json")
	require.NoError(t, err)
	jsonBytes, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	indexedResources, err := ParseConfig(jsonBytes)
	require.NoError(t, err)
	return indexedResources
}

var service = api.CompoundServiceName{
		Name: "s2",
	}
