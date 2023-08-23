package version

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		err := os.WriteFile(golden, []byte(got), 0644)
		require.NoError(t, err)
	}

	expected, err := os.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func TestFormat(t *testing.T) {
	buildDate, _ := time.Parse(time.RFC3339, "2022-06-01T13:18:45Z")
	info := VersionInfo{
		HumanVersion: "1.99.3-beta1",
		Version:      "1.99.3",
		Prerelease:   "beta1",
		Revision:     "5e5dbedd47a5f875b60e241c5555a9caab595246",
		BuildDate:    buildDate,
		RPC: RPCVersionInfo{
			Default: 2,
			Min:     1,
			Max:     3,
		},
	}

	formatters := map[string]Formatter{
		"pretty": newPrettyFormatter(),
		// the JSON formatter ignores the showMeta
		"json": newJSONFormatter(),
	}

	for fmtName, formatter := range formatters {
		t.Run(fmtName, func(t *testing.T) {
			actual, err := formatter.Format(&info)
			require.NoError(t, err)

			gName := fmt.Sprintf("%s", fmtName)

			expected := golden(t, gName, actual)
			require.Equal(t, expected, actual)
		})
	}
}
