package test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testVersions = `[{"version":"1.1.0"},{"version":"1.2.0"},{"version":"1.3.0"},{"version":"1.4.0"},{"version":"1.5.0"}]`
const testDownload = `{ "builds": [{"arch": "amd64", "os": "linux", "url": "goodurl" }, {"arch": "amd64", "os": "freebsd", "url": "goodurl" }, {"arch": "risc-v", "os": "linux", "url": "badurl"}, {"arch": "arm64", "os": "openbsd", "url": "badurl" }]}`

func TestDownload(t *testing.T) {

	// calls API
	t.Run("latestReleases", func(t *testing.T) {
		consuls := latestReleases("consul")
		if len(consuls) != 3 {
			t.Errorf("weird number of versions?? `%v`", consuls)
		}
		vaults := latestReleases("vault")
		if len(vaults) != 3 {
			t.Errorf("weird number of versions?? `%v`", vaults)
		}
	})

	// calls API
	t.Run("downloadURL", func(t *testing.T) {
		for _, v := range latestReleases("vault") {
			v := v
			url := downloadURL("vault", v)
			if !(strings.HasPrefix(url, "http") && strings.HasSuffix(url, "zip")) && url != "goodurl" {
				t.Errorf("had url: %s", url)
			}
		}
	})

	// Table drivin mocked coverage to verify behavior
	//
	defer func(lt, ar func(string, string) []byte) {
		lastTwenty = lt
		aboutRelease = ar
	}(lastTwenty, aboutRelease)
	// comment these 2 out to run against external (actual) API
	lastTwenty = func(string, string) []byte { return []byte(testVersions) }
	//aboutRelease = func(string, string) []byte { return []byte(testDownload) }

	t.Run("latestReleases-table-test", func(t *testing.T) {
		tests := []struct {
			name, json string
			expected   []string
		}{
			{name: "base", json: testVersions, expected: []string{"1.1.0", "1.2.0", "1.3.0"}},
			{name: "multiple-1.1s", json: `[{"version":"1.1.0"},{"version":"1.1.1"},{"version":"1.2.0"},{"version":"1.2.1"},{"version":"1.3.0"}]`, expected: []string{"1.1.0", "1.2.0", "1.3.0"}},
			{name: "duplicates", json: `[{"version":"1.1.0"},{"version":"1.1.0"},{"version":"1.2.0"},{"version":"1.2.0"},{"version":"1.3.0"}]`, expected: []string{"1.1.0", "1.2.0", "1.3.0"}},
			{name: "no-third", json: `[{"version":"1.1.0"},{"version":"1.1.1"},{"version":"1.2.0"}]`, expected: []string{"1.1.0", "1.2.0"}},
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				lastTwenty = func(string, string) []byte { return []byte(tc.json) }
				result := latestReleases("dummy")
				require.EqualValues(t, tc.expected, result)
			})
		}
	})
}
