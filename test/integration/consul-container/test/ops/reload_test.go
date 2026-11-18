// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ops

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}

func debugConfig(t retry.TestingTB, n node) map[string]interface{} {
	self, err := n.Client().Agent().Self()
	require.NoError(t, err)
	return self["DebugConfig"]
}

func reload(t *testing.T, n node) {
	t.Helper()
	out, code := n.CLI(t, "reload")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "Configuration reload triggered")
}

const fileService = `service {
  name = "from-file"
  id = "from-file"
  port = %d
  tags = ["file"]
  check {
    name = "ttl"
    ttl = "1h"
    status = "passing"
  }
}`

// TestReload_Services verifies that `consul reload` makes an agent follow its
// configuration files: services and checks defined in files appear, change and
// disappear without a restart, and services registered through the API are
// left alone.
func TestReload_Services(t *testing.T) {
	e := newEnv(t, "", "")
	agent := e.client

	err := agent.Client().Agent().ServiceRegister(&api.AgentServiceRegistration{ID: "from-api", Name: "from-api", Port: 7000})
	require.NoError(t, err)

	servicePort := func(r retry.TestingTB, id string) (int, bool) {
		services, err := agent.Client().Agent().Services()
		require.NoError(r, err)
		svc, ok := services[id]
		if !ok {
			return 0, false
		}
		return svc.Port, true
	}

	t.Run("a service defined in a file appears after a reload", func(t *testing.T) {
		agent.WriteConfig(t, "reload-service.hcl", fmtService(9000))
		_, ok := servicePort(t, "from-file")
		require.False(t, ok, "the file is only read on reload")

		reload(t, agent)

		eventually(t, func(r *retry.R) {
			port, ok := servicePort(r, "from-file")
			require.True(r, ok)
			require.Equal(r, 9000, port)

			svcs, _, err := e.server.Client().Catalog().Service("from-file", "file", nil)
			require.NoError(r, err)
			require.Len(r, svcs, 1)

			checks, err := agent.Client().Agent().Checks()
			require.NoError(r, err)
			var found bool
			for _, c := range checks {
				if c.ServiceID == "from-file" {
					found = true
					require.Equal(r, api.HealthPassing, c.Status)
				}
			}
			require.True(r, found, "the check of the service is registered with it")
		})
	})

	t.Run("a changed definition is picked up", func(t *testing.T) {
		agent.WriteConfig(t, "reload-service.hcl", fmtService(9001))
		reload(t, agent)

		eventually(t, func(r *retry.R) {
			port, ok := servicePort(r, "from-file")
			require.True(r, ok)
			require.Equal(r, 9001, port)
		})
	})

	t.Run("a service registered through the API survives a reload", func(t *testing.T) {
		port, ok := servicePort(t, "from-api")
		require.True(t, ok)
		require.Equal(t, 7000, port)
	})

	t.Run("a removed file removes its service", func(t *testing.T) {
		agent.RemoveConfig(t, "reload-service.hcl")
		reload(t, agent)

		eventually(t, func(r *retry.R) {
			_, ok := servicePort(r, "from-file")
			require.False(r, ok)

			svcs, _, err := e.server.Client().Catalog().Service("from-file", "", nil)
			require.NoError(r, err)
			require.Empty(r, svcs)
		})
		_, ok := servicePort(t, "from-api")
		require.True(t, ok)
	})
}

// TestReload_Limits verifies that connection and request limits are applied by
// a reload, without restarting the agent or dropping its state.
func TestReload_Limits(t *testing.T) {
	e := newEnv(t, "", "")
	srv := e.server

	t.Run("a lower connection limit is enforced after a reload", func(t *testing.T) {
		srv.WriteConfig(t, "reload-limits.hcl", `limits { http_max_conns_per_client = 5 }`)
		reload(t, srv)

		require.EqualValues(t, 5, debugConfig(t, srv)["HTTPMaxConnsPerClient"])

		accepted, refused := utils.FillHTTPConns(t, srv.HTTPAddr(), 30)
		defer utils.CloseHTTPConns(accepted)
		require.Equal(t, http.StatusTooManyRequests, refused)
		require.LessOrEqual(t, len(accepted), 5)
	})

	t.Run("a higher limit is enforced after another reload", func(t *testing.T) {
		srv.WriteConfig(t, "reload-limits.hcl", `limits { http_max_conns_per_client = 50 }`)
		reload(t, srv)

		require.EqualValues(t, 50, debugConfig(t, srv)["HTTPMaxConnsPerClient"])

		accepted, refused := utils.FillHTTPConns(t, srv.HTTPAddr(), 30)
		defer utils.CloseHTTPConns(accepted)
		require.Zero(t, refused, "30 connections fit under the new limit")
		require.Len(t, accepted, 30)
	})

	t.Run("request limits are applied after a reload", func(t *testing.T) {
		before := debugConfig(t, srv)
		require.NotEqual(t, 100, before["RequestLimitsReadRate"], "reads are unlimited by default")

		srv.WriteConfig(t, "reload-limits.hcl", `limits {
  request_limits {
    mode = "permissive"
    read_rate = 100
    write_rate = 50
  }
}`)
		reload(t, srv)

		after := debugConfig(t, srv)
		require.EqualValues(t, 100, after["RequestLimitsReadRate"])
		require.EqualValues(t, 50, after["RequestLimitsWriteRate"])
		require.NotEqual(t, before["RequestLimitsMode"], after["RequestLimitsMode"])
	})

	t.Run("removing the file returns to the defaults", func(t *testing.T) {
		srv.RemoveConfig(t, "reload-limits.hcl")
		reload(t, srv)

		after := debugConfig(t, srv)
		require.EqualValues(t, 200, after["HTTPMaxConnsPerClient"])
		require.NotEqual(t, 100, after["RequestLimitsReadRate"])
		require.EqualValues(t, 0, after["RequestLimitsMode"], "request limits are disabled by default")
	})
}

// TestReload_InvalidConfiguration verifies that a reload that cannot be
// applied is refused and leaves the running configuration as it was.
func TestReload_InvalidConfiguration(t *testing.T) {
	e := newEnv(t, "", "")
	srv := e.server

	srv.WriteConfig(t, "reload-good.hcl", `limits { http_max_conns_per_client = 40 }`)
	reload(t, srv)
	require.EqualValues(t, 40, debugConfig(t, srv)["HTTPMaxConnsPerClient"])

	t.Run("a file that does not parse is refused", func(t *testing.T) {
		srv.WriteConfig(t, "reload-bad.hcl", `service {
  name = "broken"
  port = "not-a-number"
}`)

		out, code := srv.CLI(t, "reload")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "Error reloading")
		require.Contains(t, out, "reload-bad.hcl")

		require.EqualValues(t, 40, debugConfig(t, srv)["HTTPMaxConnsPerClient"], "the running configuration is kept")
		_, err := srv.Client().Status().Leader()
		require.NoError(t, err, "the agent keeps serving")
	})

	t.Run("a later valid configuration is applied", func(t *testing.T) {
		srv.RemoveConfig(t, "reload-bad.hcl")
		srv.WriteConfig(t, "reload-good.hcl", `limits { http_max_conns_per_client = 60 }`)
		reload(t, srv)

		require.EqualValues(t, 60, debugConfig(t, srv)["HTTPMaxConnsPerClient"])
	})
}

func fmtService(port int) string {
	return fmt.Sprintf(fileService, port)
}
