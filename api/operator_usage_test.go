// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_OperatorUsage(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	registerService := func(svc *AgentService) {
		reg := &CatalogRegistration{
			Datacenter: "dc1",
			Node:       "foobar",
			Address:    "192.168.10.10",
			Service:    svc,
		}
		if _, err := c.Catalog().Register(reg, nil); err != nil {
			t.Fatal(err)
		}
	}
	registerService(&AgentService{
		ID:      "redis1",
		Service: "redis",
		Port:    8000,
	})
	registerService(&AgentService{
		ID:      "redis2",
		Service: "redis",
		Port:    8001,
	})
	registerService(&AgentService{
		Kind:    ServiceKindConnectProxy,
		ID:      "proxy1",
		Service: "proxy",
		Port:    9000,
		Proxy:   &AgentServiceConnectProxyConfig{DestinationServiceName: "foo"},
	})
	registerService(&AgentService{
		ID:      "web-native",
		Service: "web",
		Port:    8002,
		Connect: &AgentServiceConnect{Native: true},
	})

	usage, _, err := c.Operator().Usage(nil)
	require.NoError(t, err)
	require.Contains(t, usage.Usage, "dc1")
	require.Equal(t, 4, usage.Usage["dc1"].Services)
	require.Equal(t, 5, usage.Usage["dc1"].ServiceInstances)
	require.Equal(t, map[string]int{
		"api-gateway":         0,
		"connect-native":      1,
		"connect-proxy":       1,
		"ingress-gateway":     0,
		"mesh-gateway":        0,
		"terminating-gateway": 0,
	}, usage.Usage["dc1"].ConnectServiceInstances)
	require.Equal(t, 3, usage.Usage["dc1"].BillableServiceInstances)
}
