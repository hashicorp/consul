package dns

import (
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestServer_ReloadConfig tests that the ReloadConfig method calls the router's ReloadConfig method.
func TestServer_ReloadConfig(t *testing.T) {
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
