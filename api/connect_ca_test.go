package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestAPI_ConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		// Explicitly disable Connect to prevent CA being bootstrapped
		c.Connect = map[string]interface{}{
			"enabled": false,
		}
	})
	defer s.Stop()

	s.WaitForSerfCheck(t)

	connect := c.Connect()
	_, _, err := connect.CARoots(nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "Connect must be enabled")
}

func TestAPI_ConnectCARoots_list(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	// This fails occasionally if server doesn't have time to bootstrap CA so
	// retry
	retry.Run(t, func(r *retry.R) {
		connect := c.Connect()
		list, meta, err := connect.CARoots(nil)
		r.Check(err)
		if meta.LastIndex == 0 {
			r.Fatalf("expected roots raft index to be > 0")
		}
		if v := len(list.Roots); v != 1 {
			r.Fatalf("expected 1 root, got %d", v)
		}
		// connect.TestClusterID causes import cycle so hard code it
		if list.TrustDomain != "11111111-2222-3333-4444-555555555555.consul" {
			r.Fatalf("expected fixed trust domain got '%s'", list.TrustDomain)
		}
	})

}

func TestAPI_ConnectCAConfig_get_set(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForSerfCheck(t)
	expected := &ConsulCAProviderConfig{
		IntermediateCertTTL: 365 * 24 * time.Hour,
	}
	expected.LeafCertTTL = 72 * time.Hour
	expected.RootCertTTL = 10 * 365 * 24 * time.Hour

	// This fails occasionally if server doesn't have time to bootstrap CA so
	// retry
	retry.Run(t, func(r *retry.R) {
		connect := c.Connect()

		conf, _, err := connect.CAGetConfig(nil)
		r.Check(err)
		if conf.Provider != "consul" {
			r.Fatalf("expected default provider, got %q", conf.Provider)
		}
		parsed, err := ParseConsulCAConfig(conf.Config)
		r.Check(err)
		require.Equal(r, expected, parsed)

		// Change a config value and update
		conf.Config["PrivateKey"] = ""
		conf.Config["IntermediateCertTTL"] = 300 * 24 * time.Hour
		conf.Config["RootCertTTL"] = 11 * 365 * 24 * time.Hour

		// Pass through some state as if the provider stored it so we can make sure
		// we can read it again.
		conf.Config["test_state"] = map[string]string{"foo": "bar"}

		_, err = connect.CASetConfig(conf, nil)
		r.Check(err)

		updated, _, err := connect.CAGetConfig(nil)
		r.Check(err)
		expected.IntermediateCertTTL = 300 * 24 * time.Hour
		expected.RootCertTTL = 11 * 365 * 24 * time.Hour
		parsed, err = ParseConsulCAConfig(updated.Config)
		r.Check(err)
		require.Equal(r, expected, parsed)
		require.Equal(r, "bar", updated.State["foo"])
	})
}
