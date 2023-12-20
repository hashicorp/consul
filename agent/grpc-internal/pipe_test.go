// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"bufio"
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPipeListener_RoundTrip(t *testing.T) {
	lis := NewPipeListener()
	t.Cleanup(func() { _ = lis.Close() })

	go echoServer(lis)

	conn, err := lis.DialContext(context.Background(), "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	input := []byte("Hello World\n")
	_, err = conn.Write(input)
	require.NoError(t, err)

	output := make([]byte, len(input))
	_, err = conn.Read(output)
	require.NoError(t, err)

	require.Equal(t, string(input), string(output))
}

func TestPipeListener_Closed(t *testing.T) {
	lis := NewPipeListener()
	require.NoError(t, lis.Close())

	_, err := lis.Accept()
	require.ErrorIs(t, err, ErrPipeClosed)

	_, err = lis.DialContext(context.Background(), "")
	require.ErrorIs(t, err, ErrPipeClosed)
}

func echoServer(lis net.Listener) {
	handleConn := func(conn net.Conn) {
		defer conn.Close()

		reader := bufio.NewReader(conn)
		for {
			msg, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			if _, err := conn.Write(msg); err != nil {
				return
			}
		}
	}

	for {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		go handleConn(conn)
	}
}
