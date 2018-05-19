package proxy

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/connect/proxy"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestCommandConfigWatcher(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name  string
		Flags []string
		Test  func(*testing.T, *proxy.Config)
	}{
		{
			"-service flag only",
			[]string{"-service", "web"},
			func(t *testing.T, cfg *proxy.Config) {
				require.Equal(t, 0, cfg.PublicListener.BindPort)
				require.Len(t, cfg.Upstreams, 0)
			},
		},

		{
			"-service flag with upstreams",
			[]string{
				"-service", "web",
				"-upstream", "db:1234",
				"-upstream", "db2:2345",
			},
			func(t *testing.T, cfg *proxy.Config) {
				require.Equal(t, 0, cfg.PublicListener.BindPort)
				require.Len(t, cfg.Upstreams, 2)
				require.Equal(t, 1234, cfg.Upstreams[0].LocalBindPort)
				require.Equal(t, 2345, cfg.Upstreams[1].LocalBindPort)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			a := agent.NewTestAgent(t.Name(), ``)
			defer a.Shutdown()
			client := a.Client()

			ui := cli.NewMockUi()
			c := New(ui, make(chan struct{}))
			c.testNoStart = true

			// Run and purposely fail the command
			code := c.Run(append([]string{
				"-http-addr=" + a.HTTPAddr(),
			}, tc.Flags...))
			require.Equal(0, code, ui.ErrorWriter.String())

			// Get the configuration watcher
			cw, err := c.configWatcher(client)
			require.NoError(err)
			tc.Test(t, testConfig(t, cw))
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
