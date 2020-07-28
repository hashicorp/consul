package proto

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/types"
	"github.com/mitchellh/mapstructure"

	"github.com/stretchr/testify/require"
)

type pbTSWrapper struct {
	Timestamp *types.Timestamp
}

type timeTSWrapper struct {
	Timestamp time.Time
}

func TestHookPBTimestampToTime(t *testing.T) {
	in := pbTSWrapper{
		Timestamp: &types.Timestamp{
			Seconds: 1000,
			Nanos:   42,
		},
	}

	expected := timeTSWrapper{
		Timestamp: time.Unix(1000, 42),
	}

	var actual timeTSWrapper
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: HookPBTimestampToTime,
		Result:     &actual,
	})
	require.NoError(t, err)
	require.NoError(t, decoder.Decode(in))

	require.Equal(t, expected, actual)
}

func TestHookTimeToPBTimestamp(t *testing.T) {
	in := timeTSWrapper{
		Timestamp: time.Unix(999999, 123456),
	}

	expected := pbTSWrapper{
		Timestamp: &types.Timestamp{
			Seconds: 999999,
			Nanos:   123456,
		},
	}

	var actual pbTSWrapper
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: HookTimeToPBTimestamp,
		Result:     &actual,
	})
	require.NoError(t, err)
	require.NoError(t, decoder.Decode(in))

	require.Equal(t, expected, actual)
}
