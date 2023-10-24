// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/stretchr/testify/require"
)

func (a *Asserter) CheckBlankspaceNameViaHTTP(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	useHTTP2 bool,
	path string,
	clusterName string,
	sid topology.ServiceID,
) {
	t.Helper()

	a.checkBlankspaceNameViaHTTPWithCallback(t, service, upstream, useHTTP2, path, 1, func(r *retry.R, remoteName string) {
		require.Equal(r, fmt.Sprintf("%s::%s", clusterName, sid.String()), remoteName)
	}, func(r *retry.R) {})
}

func (a *Asserter) CheckBlankspaceNameTrafficSplitViaHTTP(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	useHTTP2 bool,
	path string,
	expect map[string]int,
	epsilon int,
) {
	got := make(map[string]int)
	a.checkBlankspaceNameViaHTTPWithCallback(t, service, upstream, useHTTP2, path, 100, func(_ *retry.R, name string) {
		got[name]++
	}, func(r *retry.R) {
		assertTrafficSplit(r, got, expect, epsilon)
	})
}

func (a *Asserter) checkBlankspaceNameViaHTTPWithCallback(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	useHTTP2 bool,
	path string,
	count int,
	attemptFn func(r *retry.R, remoteName string),
	checkFn func(r *retry.R),
) {
	t.Helper()

	var (
		node   = service.Node
		addr   = fmt.Sprintf("%s:%d", node.LocalAddress(), service.PortOrDefault(upstream.PortName))
		client = a.MustGetHTTPClient(t, node.Cluster)
	)

	if useHTTP2 {
		client = EnableHTTP2(client)
	}

	var actualURL string
	if upstream.Implied {
		actualURL = fmt.Sprintf("http://%s--%s--%s.virtual.consul:%d/%s",
			upstream.ID.Name,
			upstream.ID.Namespace,
			upstream.ID.Partition,
			upstream.VirtualPort,
			path,
		)
	} else {
		actualURL = fmt.Sprintf("http://localhost:%d/%s", upstream.LocalPort, path)
	}

	multiassert(t, count, func(r *retry.R) {
		name, err := GetBlankspaceNameViaHTTP(context.Background(), client, addr, actualURL)
		require.NoError(r, err)
		attemptFn(r, name)
	}, func(r *retry.R) {
		checkFn(r)
	})
}

func (a *Asserter) CheckBlankspaceNameViaTCP(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	clusterName string,
	sid topology.ServiceID,
) {
	t.Helper()

	a.checkBlankspaceNameViaTCPWithCallback(t, service, upstream, 1, func(r *retry.R, remoteName string) {
		require.Equal(r, fmt.Sprintf("%s::%s", clusterName, sid.String()), remoteName)
	}, func(r *retry.R) {})
}

func (a *Asserter) CheckBlankspaceNameTrafficSplitViaTCP(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	expect map[string]int,
	epsilon int,
) {
	got := make(map[string]int)
	a.checkBlankspaceNameViaTCPWithCallback(t, service, upstream, 100, func(_ *retry.R, name string) {
		got[name]++
	}, func(r *retry.R) {
		assertTrafficSplit(r, got, expect, epsilon)
	})
}

func (a *Asserter) checkBlankspaceNameViaTCPWithCallback(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	count int,
	attemptFn func(r *retry.R, remoteName string),
	checkFn func(r *retry.R),
) {
	t.Helper()

	require.False(t, upstream.Implied, "helper does not support tproxy yet")
	port := upstream.LocalPort
	require.True(t, port > 0)

	node := service.Node

	// We can't use the forward proxy for TCP yet, so use the exposed port on localhost instead.
	exposedPort := node.ExposedPort(port)
	require.True(t, exposedPort > 0)

	addr := fmt.Sprintf("%s:%d", "127.0.0.1", exposedPort)

	multiassert(t, count, func(r *retry.R) {
		name, err := GetBlankspaceNameViaTCP(context.Background(), addr)
		require.NoError(r, err)
		attemptFn(r, name)
	}, func(r *retry.R) {
		checkFn(r)
	})
}

func (a *Asserter) CheckBlankspaceNameViaGRPC(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	clusterName string,
	sid topology.ServiceID,
) {
	t.Helper()

	a.checkBlankspaceNameViaGRPCWithCallback(t, service, upstream, 1, func(r *retry.R, remoteName string) {
		require.Equal(r, fmt.Sprintf("%s::%s", clusterName, sid.String()), remoteName)
	}, func(r *retry.R) {})
}

func (a *Asserter) CheckBlankspaceNameTrafficSplitViaGRPC(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	expect map[string]int,
	epsilon int,
) {
	got := make(map[string]int)
	a.checkBlankspaceNameViaGRPCWithCallback(t, service, upstream, 100, func(_ *retry.R, name string) {
		got[name]++
	}, func(r *retry.R) {
		assertTrafficSplit(r, got, expect, epsilon)
	})
}

func (a *Asserter) checkBlankspaceNameViaGRPCWithCallback(
	t *testing.T,
	service *topology.Service,
	upstream *topology.Upstream,
	count int,
	attemptFn func(r *retry.R, remoteName string),
	checkFn func(r *retry.R),
) {
	t.Helper()

	require.False(t, upstream.Implied, "helper does not support tproxy yet")
	port := upstream.LocalPort
	require.True(t, port > 0)

	node := service.Node

	// We can't use the forward proxy for gRPC yet, so use the exposed port on localhost instead.
	exposedPort := node.ExposedPort(port)
	require.True(t, exposedPort > 0)

	addr := fmt.Sprintf("%s:%d", "127.0.0.1", exposedPort)

	multiassert(t, count, func(r *retry.R) {
		name, err := GetBlankspaceNameViaGRPC(context.Background(), addr)
		require.NoError(r, err)
		attemptFn(r, name)
	}, func(r *retry.R) {
		checkFn(r)
	})
}

func assertTrafficSplit(t require.TestingT, nameCounts map[string]int, expect map[string]int, epsilon int) {
	require.Len(t, nameCounts, len(expect))
	for name, expectCount := range expect {
		gotCount, ok := nameCounts[name]
		require.True(t, ok)
		if len(expect) == 1 {
			require.Equal(t, expectCount, gotCount)
		} else {
			require.InEpsilon(t, expectCount, gotCount, float64(epsilon),
				"expected %q side of split to have %d requests not %d (e=%d)",
				name, expectCount, gotCount, epsilon,
			)
		}
	}
}

func multiassert(t *testing.T, count int, attemptFn, checkFn func(r *retry.R)) {
	retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
		for i := 0; i < count; i++ {
			attemptFn(r)
		}
		checkFn(r)
	})
}
