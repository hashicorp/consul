// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestServer_ReloadConfig tests that the ReloadConfig method calls the router's ReloadConfig method.
func TestDNSServer_ReloadConfig(t *testing.T) {
	srv, err := NewServer(Config{
		AgentConfig: &config.RuntimeConfig{
			DNSDomain:    "test-domain",
			DNSAltDomain: "test-alt-domain",
		},
		Logger: testutil.Logger(t),
	})
	srv.Router = NewMockDNSRouter(t)
	require.NoError(t, err)
	cfg := &config.RuntimeConfig{
		DNSARecordLimit:       123,
		DNSEnableTruncate:     true,
		DNSNodeTTL:            123,
		DNSRecursorStrategy:   "test",
		DNSRecursorTimeout:    123,
		DNSUDPAnswerLimit:     123,
		DNSNodeMetaTXT:        true,
		DNSDisableCompression: true,
		DNSSOA: config.RuntimeSOAConfig{
			Expire:  123,
			Refresh: 123,
			Retry:   123,
			Minttl:  123,
		},
	}
	srv.Router.(*MockDNSRouter).On("ReloadConfig", cfg).Return(nil)
	err = srv.ReloadConfig(cfg)
	require.NoError(t, err)
	require.True(t, srv.Router.(*MockDNSRouter).AssertExpectations(t))
}

// TestDNSServer_Lifecycle tests that the server can be started and shutdown.
func TestDNSServer_Lifecycle(t *testing.T) {
	// Arrange
	srv, err := NewServer(Config{
		AgentConfig: &config.RuntimeConfig{
			DNSDomain:    "test-domain",
			DNSAltDomain: "test-alt-domain",
		},
		Logger: testutil.Logger(t),
	})
	defer srv.Shutdown()
	require.NotNil(t, srv.Router)
	require.NoError(t, err)
	require.NotNil(t, srv)

	ch := make(chan bool)
	go func() {
		err = srv.ListenAndServe("udp", "127.0.0.1:8500", func() {
			ch <- true
		})
		require.NoError(t, err)
	}()
	started, ok := <-ch
	require.True(t, ok)
	require.True(t, started)
	require.NotNil(t, srv.Handler)
	require.NotNil(t, srv.Handler.(*Router))
	require.NotNil(t, srv.PacketConn)

	//Shutdown
	srv.Shutdown()
	require.Nil(t, srv.Router)
}
