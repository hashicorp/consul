package builder

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	t.Helper()
	m := protojson.MarshalOptions{
		Multiline: true,
	}
	gotJSON, err := m.Marshal(pb)
	require.NoError(t, err)
	return string(gotJSON)
}

func goldenValue(t *testing.T, goldenFile string, actual string, update bool) string {
	t.Helper()
	goldenPath := filepath.Join("testdata", goldenFile) + ".golden"

	if update {
		err := os.WriteFile(goldenPath, []byte(actual), 0644)
		require.NoError(t, err)

		return actual
	}

	content, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	return string(content)
}
