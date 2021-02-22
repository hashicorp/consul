package xds

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

// goldenEnvoy is a special variant of golden() that silos each named test by
// each supported envoy version
func goldenEnvoy(t *testing.T, name, envoyVersion, newestEnvoyVersion, got string) string {
	require.NotEmpty(t, envoyVersion)

	subname := goldenEnvoyVersionName(t, envoyVersion)
	newestSubname := goldenEnvoyVersionName(t, newestEnvoyVersion)

	return golden(t, name, subname, newestSubname, got)
}

func goldenEnvoyVersionName(t *testing.T, envoyVersion string) string {
	require.NotEmpty(t, envoyVersion)

	// We do version sniffing on the complete version, but only generate
	// golden files ignoring the patch portion
	version := version.Must(version.NewVersion(envoyVersion))
	segments := version.Segments()
	require.Len(t, segments, 3)

	return fmt.Sprintf("envoy-%d-%d-x", segments[0], segments[1])
}

// golden reads and optionally writes the expected data to the golden file,
// returning the contents as a string.
func golden(t *testing.T, name, subname, newestSubname, got string) string {
	t.Helper()

	suffix := ".golden"
	if subname != "" {
		suffix = fmt.Sprintf(".%s.golden", subname)
	}

	golden := filepath.Join("testdata", name+suffix)

	// To trim down PRs, we only create per-version golden files if they differ
	// from the latest version.
	newestExpected := ""
	if newestSubname != "" && subname != newestSubname {
		newestGolden := filepath.Join("testdata", fmt.Sprintf("%s.%s.golden", name, newestSubname))
		expected, err := ioutil.ReadFile(newestGolden)
		require.NoError(t, err)

		if string(expected) == got {
			if *update && got != "" {
				err := os.Remove(golden)
				if err != nil && !os.IsNotExist(err) {
					require.NoError(t, err)
				}
			}
			return string(expected)
		}

		newestExpected = string(expected)
	}

	if *update && got != "" {
		err := ioutil.WriteFile(golden, []byte(got), 0644)
		require.NoError(t, err)
	}

	expected, err := ioutil.ReadFile(golden)
	if newestExpected != "" && os.IsNotExist(err) {
		return newestExpected
	}
	require.NoError(t, err)
	return string(expected)
}

func responseToJSON(t *testing.T, r *envoy.DiscoveryResponse) string {
	return protoToJSON(t, r)
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	t.Helper()
	m := jsonpb.Marshaler{
		Indent: "  ",
	}
	gotJSON, err := m.MarshalToString(pb)
	require.NoError(t, err)
	return gotJSON
}
