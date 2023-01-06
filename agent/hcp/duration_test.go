package hcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_formatDuration(t *testing.T) {
	cases := []struct {
		name     string
		d        time.Duration
		expected string
	}{
		{
			name:     "NegativeDuration",
			d:        time.Second * 5 * -1,
			expected: "-5s",
		},
		{
			name:     "NormalDuration",
			d:        time.Second * 5,
			expected: "5s",
		},
		{
			name:     "MoreThanAMinute",
			d:        time.Minute * 2,
			expected: "120s",
		},
		{
			name:     "MixMinuteAndSecond",
			d:        time.Minute*2 + time.Second*5,
			expected: "125s",
		},
		{
			name:     "MixedSecondAndNanosecond",
			d:        time.Second*5 + time.Nanosecond*200,
			expected: "5.000000200s",
		},
		{
			name:     "MixedSecondAndNanosecondWithTrim",
			d:        time.Second*5 + time.Nanosecond*2000,
			expected: "5.000002s",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, formatDuration(tc.d))
		})
	}

	t.Run("MoreThanMinute", func(t *testing.T) {
		d := time.Second * 120
		expectedVal := "120s"
		require.Equal(t, expectedVal, formatDuration(d))
	})
}
