package lib

import (
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/require"
)

func TestErrIsEOF(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		name string
		err  error
	}{
		{name: "EOF", err: io.EOF},
		{name: "yamuxStreamClosed", err: yamux.ErrStreamClosed},
		{name: "yamuxSessionShutdown", err: yamux.ErrSessionShutdown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.True(t, IsErrEOF(tt.err))
		})
		t.Run(fmt.Sprintf("Wrapped %s", tt.name), func(t *testing.T) {
			t.Parallel()
			require.True(t, IsErrEOF(fmt.Errorf("test: %w", tt.err)))
		})
		t.Run(fmt.Sprintf("String suffix is %s", tt.name), func(t *testing.T) {
			t.Parallel()
			require.True(t, IsErrEOF(fmt.Errorf("rpc error: %s", tt.err.Error())))
		})
	}
}
