// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxy

import (
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/connect/proxy"
)

func TestCommandConfigWatcher(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := []struct {
		Name    string
		Flags   []string
		Test    func(*testing.T, *proxy.Config)
		WantErr string
	}{
		{
			Name:  "-service flag only",
			Flags: []string{"-service", "web"},
			Test: func(t *testing.T, cfg *proxy.Config) {
				require.Equal(t, 0, cfg.PublicListener.BindPort)
				require.Len(t, cfg.Upstreams, 0)
			},
		},

		{
			Name: "-service flag with upstreams",
			Flags: []string{
				"-service", "web",
				"-upstream", "db:1234",
				"-upstream", "db2:2345",
			},
			Test: func(t *testing.T, cfg *proxy.Config) {
				require.Equal(t, 0, cfg.PublicListener.BindPort)
				require.Len(t, cfg.Upstreams, 2)
				require.Equal(t, 1234, cfg.Upstreams[0].LocalBindPort)
				require.Equal(t, 2345, cfg.Upstreams[1].LocalBindPort)
			},
		},

		{
			Name:  "-service flag with -service-addr",
			Flags: []string{"-service", "web"},
			Test: func(t *testing.T, cfg *proxy.Config) {
				// -service-addr has no affect since -listen isn't set
				require.Equal(t, 0, cfg.PublicListener.BindPort)
				require.Len(t, cfg.Upstreams, 0)
			},
		},

		{
			Name: "-service, -service-addr, -listen",
			Flags: []string{
				"-service", "web",
				"-service-addr", "127.0.0.1:1234",
				"-listen", ":4567",
			},
			Test: func(t *testing.T, cfg *proxy.Config) {
				require.Len(t, cfg.Upstreams, 0)

				require.Equal(t, "", cfg.PublicListener.BindAddress)
				require.Equal(t, 4567, cfg.PublicListener.BindPort)
				require.Equal(t, "127.0.0.1:1234", cfg.PublicListener.LocalServiceAddress)
			},
		},

		{
			Name: "-sidecar-for, no sidecar",
			Flags: []string{
				"-sidecar-for", "no-sidecar",
			},
			WantErr: "No sidecar proxy registered",
		},

		{
			Name: "-sidecar-for, multiple sidecars",
			Flags: []string{
				"-sidecar-for", "two-sidecars",
			},
			// Order is non-deterministic so don't assert the list of proxy IDs here
			WantErr: `More than one sidecar proxy registered for two-sidecars.
    Start proxy with -proxy-id and one of the following IDs: `,
		},

		{
			Name: "-sidecar-for, non-existent",
			Flags: []string{
				"-sidecar-for", "foo",
			},
			WantErr: "No sidecar proxy registered",
		},

		{
			Name: "-sidecar-for, one sidecar",
			Flags: []string{
				"-sidecar-for", "one-sidecar",
			},
			Test: func(t *testing.T, cfg *proxy.Config) {
				// Sanity check we got the right instance.
				require.Equal(t, 9999, cfg.PublicListener.BindPort)
			},
		},

		{
			Name: "-sidecar-for, one sidecar case-insensitive",
			Flags: []string{
				"-sidecar-for", "One-SideCar",
			},
			Test: func(t *testing.T, cfg *proxy.Config) {
				// Sanity check we got the right instance.
				require.Equal(t, 9999, cfg.PublicListener.BindPort)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			// Register a few services with 0, 1 and 2 sidecars
			a := agent.NewTestAgent(t, `
			services {
				name = "no-sidecar"
				port = 1111
			}
			services {
				name = "one-sidecar"
				port = 2222
				connect {
					sidecar_service {
						port = 9999
					}
				}
			}
			services {
				name = "two-sidecars"
				port = 3333
				connect {
					sidecar_service {}
				}
			}
			services {
				kind = "connect-proxy"
				name = "other-sidecar-for-two-sidecars"
				port = 4444
				proxy {
					destination_service_id = "two-sidecars"
					destination_service_name = "two-sidecars"
				}
			}
			`)
			defer a.Shutdown()
			client := a.Client()

			ui := cli.NewMockUi()
			c := New(ui, make(chan struct{}))
			c.testNoStart = true

			// Run the command
			code := c.Run(append([]string{
				"-http-addr=" + a.HTTPAddr(),
			}, tc.Flags...))
			if tc.WantErr == "" {
				require.Equal(t, 0, code, ui.ErrorWriter.String())
			} else {
				require.Equal(t, 1, code, ui.ErrorWriter.String())
				require.Contains(t, ui.ErrorWriter.String(), tc.WantErr)
				return
			}

			// Get the configuration watcher
			cw, err := c.configWatcher(client)
			require.NoError(t, err)
			if tc.Test != nil {
				tc.Test(t, testConfig(t, cw))
			}
		})
	}
}

func testConfig(t *testing.T, cw proxy.ConfigWatcher) *proxy.Config {
	t.Helper()

	select {
	case cfg := <-cw.Watch():
		return cfg

	case <-time.After(1 * time.Second):
		t.Fatal("no configuration loaded")
		return nil // satisfy compiler
	}
}

func TestFlagUpstreams_ConnectTimeout(t *testing.T) {
	testCases := []struct {
		name           string
		upstreamFlag   string
		configOverride map[string]interface{}
		expectedMs     int
	}{
		{
			name:         "default timeout",
			upstreamFlag: "db:8080",
			expectedMs:   10000, // 10s default
		},
		{
			name:         "custom timeout via config",
			upstreamFlag: "db:8080",
			configOverride: map[string]interface{}{
				"connect_timeout_ms": 5000,
			},
			expectedMs: 5000,
		},
		{
			name:         "zero timeout",
			upstreamFlag: "cache:6379",
			configOverride: map[string]interface{}{
				"connect_timeout_ms": 0,
			},
			expectedMs: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the upstream flag
			var upstreams FlagUpstreams
			err := upstreams.Set(tc.upstreamFlag)
			require.NoError(t, err)

			// Get the parsed upstream
			var upstream *proxy.UpstreamConfig
			for _, u := range upstreams {
				upstream = &u
				break
			}
			require.NotNil(t, upstream)

			// Override config if provided
			if tc.configOverride != nil {
				upstream.Config = tc.configOverride
			}

			// Test the timeout
			timeout := upstream.ConnectTimeout()
			expectedDuration := time.Duration(tc.expectedMs) * time.Millisecond
			require.Equal(t, expectedDuration, timeout)
		})
	}
}

func TestCatalogCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil, nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
