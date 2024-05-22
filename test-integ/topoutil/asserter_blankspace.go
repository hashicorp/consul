// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

// CheckBlankspaceNameViaHTTP calls a copy of blankspace and asserts it arrived
// on the correct instance using HTTP1 or HTTP2.
func (a *Asserter) CheckBlankspaceNameViaHTTP(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	useHTTP2 bool,
	path string,
	clusterName string,
	sid topology.ID,
) {
	t.Helper()

	a.checkBlankspaceNameViaHTTPWithCallback(t, workload, us, useHTTP2, path, 1, func(_ *retry.R) {}, func(r *retry.R, remoteName string) {
		require.Equal(r, fmt.Sprintf("%s::%s", clusterName, sid.String()), remoteName)
	}, func(r *retry.R) {})
}

// CheckBlankspaceNameTrafficSplitViaHTTP is like CheckBlankspaceNameViaHTTP
// but it is verifying a relative traffic split.
func (a *Asserter) CheckBlankspaceNameTrafficSplitViaHTTP(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	useHTTP2 bool,
	path string,
	expect map[string]int,
) {
	t.Helper()

	got := make(map[string]int)
	a.checkBlankspaceNameViaHTTPWithCallback(t, workload, us, useHTTP2, path, 100, func(_ *retry.R) {
		got = make(map[string]int)
	}, func(_ *retry.R, name string) {
		got[name]++
	}, func(r *retry.R) {
		assertTrafficSplitFor100Requests(r, got, expect)
	})
}

func (a *Asserter) checkBlankspaceNameViaHTTPWithCallback(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	useHTTP2 bool,
	path string,
	count int,
	resetFn func(r *retry.R),
	attemptFn func(r *retry.R, remoteName string),
	checkFn func(r *retry.R),
) {
	t.Helper()

	var (
		node         = workload.Node
		internalPort = workload.Port
		addr         = fmt.Sprintf("%s:%d", node.LocalAddress(), internalPort)
		client       = a.mustGetHTTPClient(t, node.Cluster)
	)

	if useHTTP2 {
		// We can't use the forward proxy for http2, so use the exposed port on localhost instead.
		exposedPort := node.ExposedPort(internalPort)
		require.True(t, exposedPort > 0)

		addr = fmt.Sprintf("%s:%d", "127.0.0.1", exposedPort)

		// This will clear the proxy field on the transport.
		client = EnableHTTP2(client)
	}

	actualURL := fmt.Sprintf("http://localhost:%d/%s", us.LocalPort, path)

	multiassert(t, count, resetFn, func(r *retry.R) {
		name, err := GetBlankspaceNameViaHTTP(context.Background(), client, addr, actualURL)
		require.NoError(r, err)
		attemptFn(r, name)
	}, func(r *retry.R) {
		checkFn(r)
	})
}

// CheckBlankspaceNameViaTCP calls a copy of blankspace and asserts it arrived
// on the correct instance using plain tcp sockets.
func (a *Asserter) CheckBlankspaceNameViaTCP(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	clusterName string,
	sid topology.ID,
) {
	t.Helper()

	a.checkBlankspaceNameViaTCPWithCallback(t, workload, us, 1, func(_ *retry.R) {}, func(r *retry.R, remoteName string) {
		require.Equal(r, fmt.Sprintf("%s::%s", clusterName, sid.String()), remoteName)
	}, func(r *retry.R) {})
}

// CheckBlankspaceNameTrafficSplitViaTCP is like CheckBlankspaceNameViaTCP
// but it is verifying a relative traffic split.
func (a *Asserter) CheckBlankspaceNameTrafficSplitViaTCP(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	expect map[string]int,
) {
	t.Helper()

	got := make(map[string]int)
	a.checkBlankspaceNameViaTCPWithCallback(t, workload, us, 100, func(_ *retry.R) {
		got = make(map[string]int)
	}, func(_ *retry.R, name string) {
		got[name]++
	}, func(r *retry.R) {
		assertTrafficSplitFor100Requests(r, got, expect)
	})
}

func (a *Asserter) checkBlankspaceNameViaTCPWithCallback(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	count int,
	resetFn func(r *retry.R),
	attemptFn func(r *retry.R, remoteName string),
	checkFn func(r *retry.R),
) {
	t.Helper()

	port := us.LocalPort
	require.True(t, port > 0)

	node := workload.Node

	// We can't use the forward proxy for TCP yet, so use the exposed port on localhost instead.
	exposedPort := node.ExposedPort(port)
	require.True(t, exposedPort > 0)

	addr := fmt.Sprintf("%s:%d", "127.0.0.1", exposedPort)

	multiassert(t, count, resetFn, func(r *retry.R) {
		name, err := GetBlankspaceNameViaTCP(context.Background(), addr)
		require.NoError(r, err)
		attemptFn(r, name)
	}, func(r *retry.R) {
		checkFn(r)
	})
}

// CheckBlankspaceNameViaGRPC calls a copy of blankspace and asserts it arrived
// on the correct instance using gRPC.
func (a *Asserter) CheckBlankspaceNameViaGRPC(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	clusterName string,
	sid topology.ID,
) {
	t.Helper()

	a.checkBlankspaceNameViaGRPCWithCallback(t, workload, us, 1, func(_ *retry.R) {}, func(r *retry.R, remoteName string) {
		require.Equal(r, fmt.Sprintf("%s::%s", clusterName, sid.String()), remoteName)
	}, func(_ *retry.R) {})
}

// CheckBlankspaceNameTrafficSplitViaGRPC is like CheckBlankspaceNameViaGRPC
// but it is verifying a relative traffic split.
func (a *Asserter) CheckBlankspaceNameTrafficSplitViaGRPC(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	expect map[string]int,
) {
	t.Helper()

	got := make(map[string]int)
	a.checkBlankspaceNameViaGRPCWithCallback(t, workload, us, 100, func(_ *retry.R) {
		got = make(map[string]int)
	}, func(_ *retry.R, name string) {
		got[name]++
	}, func(r *retry.R) {
		assertTrafficSplitFor100Requests(r, got, expect)
	})
}

func (a *Asserter) checkBlankspaceNameViaGRPCWithCallback(
	t *testing.T,
	workload *topology.Workload,
	us *topology.Upstream,
	count int,
	resetFn func(r *retry.R),
	attemptFn func(r *retry.R, remoteName string),
	checkFn func(r *retry.R),
) {
	t.Helper()

	port := us.LocalPort
	require.True(t, port > 0)

	node := workload.Node

	// We can't use the forward proxy for gRPC yet, so use the exposed port on localhost instead.
	exposedPort := node.ExposedPort(port)
	require.True(t, exposedPort > 0)

	addr := fmt.Sprintf("%s:%d", "127.0.0.1", exposedPort)

	multiassert(t, count, resetFn, func(r *retry.R) {
		name, err := GetBlankspaceNameViaGRPC(context.Background(), addr)
		require.NoError(r, err)
		attemptFn(r, name)
	}, func(r *retry.R) {
		checkFn(r)
	})
}

// assertTrafficSplitFor100Requests compares the counts of 100 requests that
// did reach an observed set of upstreams (nameCounts) against the expected
// counts of those same services is the same within a fixed difference of 2.
func assertTrafficSplitFor100Requests(t require.TestingT, nameCounts map[string]int, expect map[string]int) {
	const (
		numRequests  = 100
		allowedDelta = 2
	)
	require.Equal(t, numRequests, sumMapValues(nameCounts), "measured traffic was not %d requests", numRequests)
	require.Equal(t, numRequests, sumMapValues(expect), "expected traffic was not %d requests", numRequests)
	assertTrafficSplit(t, nameCounts, expect, allowedDelta)
}

func sumMapValues(m map[string]int) int {
	sum := 0
	for _, v := range m {
		sum += v
	}
	return sum
}

// assertTrafficSplit compares the counts of requests that did reach an
// observed set of upstreams (nameCounts) against the expected counts of
// those same services is the same within the provided allowedDelta value.
//
// When doing random traffic splits it'll never be perfect so we need the
// wiggle room to avoid having a flaky test.
func assertTrafficSplit(t require.TestingT, nameCounts map[string]int, expect map[string]int, allowedDelta int) {
	require.Len(t, nameCounts, len(expect))
	for name, expectCount := range expect {
		gotCount, ok := nameCounts[name]
		require.True(t, ok)
		if len(expect) == 1 {
			require.Equal(t, expectCount, gotCount)
		} else {
			require.InDelta(t, expectCount, gotCount, float64(allowedDelta),
				"expected %q side of split to have %d requests not %d (e=%d)",
				name, expectCount, gotCount, allowedDelta,
			)
		}
	}
}

// multiassert will retry in bulk calling attemptFn count times and following
// that with one last call to checkFn.
//
// It's primary use at the time it was written was to execute a set of requests
// repeatedly to witness where the requests went, and then at the end doing a
// verification of traffic splits (a bit like MAP/REDUCE).
func multiassert(t *testing.T, count int, resetFn, attemptFn, checkFn func(r *retry.R)) {
	retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
		resetFn(r)
		for i := 0; i < count; i++ {
			attemptFn(r)
		}
		checkFn(r)
	})
}
