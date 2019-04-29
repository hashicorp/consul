package xds

import (
	"flag"
	"io/ioutil"
	"path/filepath"
	"testing"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/gogo/protobuf/jsonpb"

	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

// golden reads and optionally writes the expected data to the golden file,
// returning the contents as a string.
func golden(t *testing.T, name, got string) string {
	t.Helper()

	golden := filepath.Join("testdata", name+".golden")
	if *update && got != "" {
		err := ioutil.WriteFile(golden, []byte(got), 0644)
		require.NoError(t, err)
	}

	expected, err := ioutil.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func responseToJSON(t *testing.T, r *envoy.DiscoveryResponse) string {
	t.Helper()
	m := jsonpb.Marshaler{
		Indent: "  ",
	}
	gotJSON, err := m.MarshalToString(r)
	require.NoError(t, err)
	return gotJSON
}
