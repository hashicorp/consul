// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestAPIServers_WithServiceRunError(t *testing.T) {
	servers := NewAPIServers(hclog.New(nil))

	server1, chErr1 := newAPIServerStub()
	server2, _ := newAPIServerStub()

	t.Run("Start", func(t *testing.T) {
		servers.Start(server1)
		servers.Start(server2)

		select {
		case <-servers.failed:
			t.Fatalf("expected servers to still be running")
		case <-time.After(5 * time.Millisecond):
		}
	})

	err := fmt.Errorf("oops, I broke")

	t.Run("server exit non-nil error", func(t *testing.T) {
		chErr1 <- err

		select {
		case <-servers.failed:
		case <-time.After(time.Second):
			t.Fatalf("expected failed channel to be closed")
		}
	})

	t.Run("shutdown remaining services", func(t *testing.T) {
		servers.Shutdown(context.Background())
		require.Equal(t, err, servers.WaitForShutdown())
	})
}

func newAPIServerStub() (apiServer, chan error) {
	chErr := make(chan error)
	return apiServer{
		Protocol: "http",
		Addr: &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.11"),
			Port: 5505,
		},
		Run: func() error {
			return <-chErr
		},
		Shutdown: func(ctx context.Context) error {
			close(chErr)
			return nil
		},
	}, chErr
}
