package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewWatcher(t *testing.T) {
	w, err := New(func(event *WatcherEvent) error {
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, w)
}
