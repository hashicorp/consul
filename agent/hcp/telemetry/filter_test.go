package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilter(t *testing.T) {
	for name, tc := range map[string]struct {
		filters   []string
		wantMatch bool
		wantErr   string
	}{
		"badFilterRegex": {
			filters: []string{"(*LF)"},
			wantErr: "compilation of filter at index 0 failed",
		},
		"matchFound": {
			filters:   []string{"raft.*"},
			wantMatch: true,
		},
		"matchNotFound": {
			filters:   []string{"mem.heap_size"},
			wantMatch: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := NewFilterList(tc.filters)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			} else {
				m := f.Match("consul.raft.peers")
				require.Equal(t, tc.wantMatch, m)
			}
		})
	}
}
