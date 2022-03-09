package lib

import (
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/yamux"

	"github.com/stretchr/testify/require"
)

func TestErrIsEOF(t *testing.T) {
	var tests = []struct {
		name string
		err  error
	}{
		{name: "EOF", err: io.EOF},
		{name: "Wrapped EOF", err: fmt.Errorf("test: %w", io.EOF)},
		{name: "yamuxStreamClosed", err: yamux.ErrStreamClosed},
		{name: "yamuxSessionShutdown", err: yamux.ErrSessionShutdown},
		{name: "ServerError(___: EOF)", err: rpc.ServerError(fmt.Sprintf("rpc error: %s", io.EOF.Error()))},
		{name: "Wrapped ServerError(___: EOF)", err: fmt.Errorf("rpc error: %w", rpc.ServerError(fmt.Sprintf("rpc error: %s", io.EOF.Error())))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, IsErrEOF(tt.err))
		})
	}
}
