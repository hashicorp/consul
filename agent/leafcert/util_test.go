// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package leafcert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestCalculateSoftExpire(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		issued   string
		lifetime time.Duration
		wantMin  string
		wantMax  string
	}{
		{
			name:     "72h just issued",
			now:      "2018-01-01 00:00:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Should jitter between 60% and 90% of the lifetime which is 43.2/64.8
			// hours after issued
			wantMin: "2018-01-02 19:12:00",
			wantMax: "2018-01-03 16:48:00",
		},
		{
			name: "72h in renew range",
			// This time should be inside the renewal range.
			now:      "2018-01-02 20:00:20",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min should be the "now" time
			wantMin: "2018-01-02 20:00:20",
			wantMax: "2018-01-03 16:48:00",
		},
		{
			name: "72h in hard renew",
			// This time should be inside the renewal range.
			now:      "2018-01-03 18:00:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-03 18:00:00",
			wantMax: "2018-01-03 18:00:00",
		},
		{
			name: "72h expired",
			// This time is after expiry
			now:      "2018-01-05 00:00:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 72 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-05 00:00:00",
			wantMax: "2018-01-05 00:00:00",
		},
		{
			name:     "1h just issued",
			now:      "2018-01-01 00:00:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Should jitter between 60% and 90% of the lifetime which is 36/54 mins
			// hours after issued
			wantMin: "2018-01-01 00:36:00",
			wantMax: "2018-01-01 00:54:00",
		},
		{
			name: "1h in renew range",
			// This time should be inside the renewal range.
			now:      "2018-01-01 00:40:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min should be the "now" time
			wantMin: "2018-01-01 00:40:00",
			wantMax: "2018-01-01 00:54:00",
		},
		{
			name: "1h in hard renew",
			// This time should be inside the renewal range.
			now:      "2018-01-01 00:55:00",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 00:55:00",
			wantMax: "2018-01-01 00:55:00",
		},
		{
			name: "1h expired",
			// This time is after expiry
			now:      "2018-01-01 01:01:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Hour,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 01:01:01",
			wantMax: "2018-01-01 01:01:01",
		},
		{
			name: "too short lifetime",
			// This time is after expiry
			now:      "2018-01-01 01:01:01",
			issued:   "2018-01-01 00:00:00",
			lifetime: 1 * time.Minute,
			// Min and max should both be the "now" time
			wantMin: "2018-01-01 01:01:01",
			wantMax: "2018-01-01 01:01:01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse("2006-01-02 15:04:05", tc.now)
			require.NoError(t, err)
			issued, err := time.Parse("2006-01-02 15:04:05", tc.issued)
			require.NoError(t, err)
			wantMin, err := time.Parse("2006-01-02 15:04:05", tc.wantMin)
			require.NoError(t, err)
			wantMax, err := time.Parse("2006-01-02 15:04:05", tc.wantMax)
			require.NoError(t, err)

			min, max := calculateSoftExpiry(now, &structs.IssuedCert{
				ValidAfter:  issued,
				ValidBefore: issued.Add(tc.lifetime),
			})

			require.Equal(t, wantMin, min)
			require.Equal(t, wantMax, max)
		})
	}
}
