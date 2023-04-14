package test

import (
	"strings"
	"testing"
)

func TestDownload(t *testing.T) {

	t.Run("latestReleases", func(t *testing.T) {
		// should return ~3 entries
		consuls := latestReleases("consul")
		if len(consuls) < 2 || len(consuls) > 4 {
			t.Errorf("weird number of versions?? `%#v`", consuls)
		}
		// should return ~3 entries
		vaults := latestReleases("vault")
		if len(vaults) < 2 || len(vaults) > 4 {
			t.Errorf("weird number of versions?? `%#v`", vaults)
		}
	})

	t.Run("downloadURL", func(t *testing.T) {
		for _, v := range latestReleases("vault") {
			url := downloadURL("vault", v)
			if !strings.HasPrefix(url, "http") || !strings.HasSuffix(url, "zip") {
				t.Errorf("had url: %s", url)
			}
		}
	})

}
