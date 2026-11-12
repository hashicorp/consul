// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"fmt"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

func executedIPs(resp *api.PreparedQueryExecuteResponse) []string {
	var ips []string
	for _, n := range resp.Nodes {
		addr := n.Service.Address
		if addr == "" {
			addr = n.Node.Address
		}
		ips = append(ips, addr)
	}
	return ips
}

// TestPreparedQuery covers the prepared query lifecycle and execution through
// both the HTTP API and the .query DNS interface.
func TestPreparedQuery(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	c := e.client

	// svc-1: passing, primary, env=prod   svc-2: passing, secondary, env=staging
	// svc-3: warning, primary, env=prod   svc-4: critical, primary, env=prod
	for i, inst := range []struct {
		tag, env, status string
	}{
		{"primary", "prod", api.HealthPassing},
		{"secondary", "staging", api.HealthPassing},
		{"primary", "prod", api.HealthWarning},
		{"primary", "prod", api.HealthCritical},
	} {
		id := fmt.Sprintf("pq-svc-%d", i+1)
		register(t, c, &api.AgentServiceRegistration{
			ID:      id,
			Name:    "pqsvc",
			Address: fmt.Sprintf("10.7.0.%d", i+1),
			Port:    8000 + i,
			Tags:    []string{inst.tag},
			Meta:    map[string]string{"env": inst.env},
			Check:   &api.AgentServiceCheck{TTL: "10m", Status: inst.status},
		})
		require.NoError(t, c.Agent().UpdateTTL("service:"+id, "", inst.status))
	}
	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		entries, _, err := c.Health().Service("pqsvc", "", false, nil)
		require.NoError(r, err)
		require.Len(r, entries, 4)
	})

	pq := c.PreparedQuery()

	t.Run("named query honours only_passing and is resolvable via DNS", func(t *testing.T) {
		id, _, err := pq.Create(&api.PreparedQueryDefinition{
			Name:    "pq-passing",
			Service: api.ServiceQuery{Service: "pqsvc", OnlyPassing: true},
			DNS:     api.QueryDNSOptions{TTL: "13s"},
		}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, id)

		byName, _, err := pq.Execute("pq-passing", nil)
		require.NoError(t, err)
		require.Equal(t, "pqsvc", byName.Service)
		require.ElementsMatch(t, []string{"10.7.0.1", "10.7.0.2"}, executedIPs(byName))

		byID, _, err := pq.Execute(id, nil)
		require.NoError(t, err)
		require.ElementsMatch(t, executedIPs(byName), executedIPs(byID))

		e.query(t, "pq-passing.query.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.ElementsMatch(r, []string{"10.7.0.1", "10.7.0.2"}, dnsIPs(resp))
			require.Equal(r, uint32(13), resp.Answer[0].Header().Ttl)
		})
	})

	t.Run("without only_passing warnings are included but critical never is", func(t *testing.T) {
		_, _, err := pq.Create(&api.PreparedQueryDefinition{
			Name:    "pq-any",
			Service: api.ServiceQuery{Service: "pqsvc"},
		}, nil)
		require.NoError(t, err)

		resp, _, err := pq.Execute("pq-any", nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"10.7.0.1", "10.7.0.2", "10.7.0.3"}, executedIPs(resp))
	})

	t.Run("tag and meta filters", func(t *testing.T) {
		_, _, err := pq.Create(&api.PreparedQueryDefinition{
			Name:    "pq-primary-only",
			Service: api.ServiceQuery{Service: "pqsvc", Tags: []string{"primary"}, OnlyPassing: true},
		}, nil)
		require.NoError(t, err)
		_, _, err = pq.Create(&api.PreparedQueryDefinition{
			Name:    "pq-not-secondary",
			Service: api.ServiceQuery{Service: "pqsvc", Tags: []string{"!secondary"}, OnlyPassing: true},
		}, nil)
		require.NoError(t, err)
		_, _, err = pq.Create(&api.PreparedQueryDefinition{
			Name:    "pq-staging",
			Service: api.ServiceQuery{Service: "pqsvc", ServiceMeta: map[string]string{"env": "staging"}},
		}, nil)
		require.NoError(t, err)

		for name, want := range map[string][]string{
			"pq-primary-only":  {"10.7.0.1"},
			"pq-not-secondary": {"10.7.0.1"},
			"pq-staging":       {"10.7.0.2"},
		} {
			resp, _, err := pq.Execute(name, nil)
			require.NoError(t, err, name)
			require.ElementsMatch(t, want, executedIPs(resp), name)
		}
	})

	t.Run("template queries match by name prefix", func(t *testing.T) {
		_, _, err := pq.Create(&api.PreparedQueryDefinition{
			Name:     "geo-",
			Template: api.QueryTemplate{Type: "name_prefix_match", Regexp: "^geo-(.*?)$"},
			Service:  api.ServiceQuery{Service: "${match(1)}", OnlyPassing: true},
		}, nil)
		require.NoError(t, err)

		resp, _, err := pq.Execute("geo-pqsvc", nil)
		require.NoError(t, err)
		require.Equal(t, "pqsvc", resp.Service)
		require.ElementsMatch(t, []string{"10.7.0.1", "10.7.0.2"}, executedIPs(resp))

		e.query(t, "geo-pqsvc.query.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.ElementsMatch(r, []string{"10.7.0.1", "10.7.0.2"}, dnsIPs(resp))
		})
	})

	t.Run("get list update delete", func(t *testing.T) {
		id, _, err := pq.Create(&api.PreparedQueryDefinition{
			Name:    "pq-crud",
			Service: api.ServiceQuery{Service: "pqsvc", OnlyPassing: true},
		}, nil)
		require.NoError(t, err)

		got, _, err := pq.Get(id, nil)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "pq-crud", got[0].Name)

		all, _, err := pq.List(nil)
		require.NoError(t, err)
		var names []string
		for _, def := range all {
			names = append(names, def.Name)
		}
		require.Contains(t, names, "pq-crud")

		def := got[0]
		def.Service.Tags = []string{"secondary"}
		_, err = pq.Update(def, nil)
		require.NoError(t, err)
		resp, _, err := pq.Execute("pq-crud", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"10.7.0.2"}, executedIPs(resp))

		_, err = pq.Delete(id, nil)
		require.NoError(t, err)
		_, _, err = pq.Execute("pq-crud", nil)
		require.Error(t, err)
		e.query(t, "pq-crud.query.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, dns.RcodeNameError, resp.Rcode)
		})
	})
}
