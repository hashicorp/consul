package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilter(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		filters             []string
		expectedRegexString string
		matches             []string
		wantErr             string
		wantMatch           bool
	}{
		"badFilterRegex": {
			filters: []string{"(*LF)"},
			wantErr: "no valid filters",
		},
		"failsWithNoRegex": {
			filters: []string{},
			wantErr: "no valid filters",
		},
		"matchFound": {
			filters:             []string{"raft.*", "mem.*"},
			expectedRegexString: "raft.*|mem.*",
			matches:             []string{"consul.raft.peers", "consul.mem.heap_size"},
			wantMatch:           true,
		},
		"matchNotFound": {
			filters:             []string{"mem.*"},
			matches:             []string{"consul.raft.peers", "consul.txn.apply"},
			expectedRegexString: "mem.*",
			wantMatch:           false,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, err := newFilterList(tc.filters)

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedRegexString, f.isValid.String())
			for _, metric := range tc.matches {
				m := f.Match(metric)
				require.Equal(t, tc.wantMatch, m)
			}
		})
	}
}
