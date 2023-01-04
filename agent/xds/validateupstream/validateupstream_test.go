package validateupstream

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestDoItTestDoIt(t *testing.T) {
	file, err := os.Open("testdata/config.json")
	require.NoError(t, err)
  jsonBytes, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	err = DoIt(jsonBytes, api.CompoundServiceName{
		Name: "s2",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad cluster")
}
