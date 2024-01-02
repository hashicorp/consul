// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdscapacity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestController(t *testing.T) {
	const index = 123

	store := state.NewStateStore(nil)

	// This loop generates:
	//
	//	4 (service kind) * 5 (service) * 5 * (node) = 100 proxy services. And 25 non-proxy services.
	for _, kind := range []structs.ServiceKind{
		// These will be included in the count.
		structs.ServiceKindConnectProxy,
		structs.ServiceKindIngressGateway,
		structs.ServiceKindTerminatingGateway,
		structs.ServiceKindMeshGateway,

		// This one will not.
		structs.ServiceKindTypical,
	} {
		for i := 0; i < 5; i++ {
			serviceName := fmt.Sprintf("%s-%d", kind, i)

			for j := 0; j < 5; j++ {
				nodeName := fmt.Sprintf("%s-node-%d", serviceName, j)

				require.NoError(t, store.EnsureRegistration(index, &structs.RegisterRequest{
					Node: nodeName,
					Service: &structs.NodeService{
						ID:      serviceName,
						Service: serviceName,
						Kind:    kind,
					},
				}))
			}
		}
	}

	limiter := newTestLimiter()

	sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
	cfg := metrics.DefaultConfig("consul")
	cfg.EnableHostname = false
	metrics.NewGlobal(cfg, sink)

	t.Cleanup(func() {
		sink := &metrics.BlackholeSink{}
		metrics.NewGlobal(cfg, sink)
	})

	adj := NewController(Config{
		Logger:         testutil.Logger(t),
		GetStore:       func() Store { return store },
		SessionLimiter: limiter,
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go adj.Run(ctx)

	// Keen readers will notice the numbers here are off by one. This is due to
	// floating point math (because we multiply by 1.1).
	testutil.RunStep(t, "load split between 2 servers", func(t *testing.T) {
		adj.SetServerCount(2)
		require.Equal(t, 56, limiter.receive(t))
	})

	testutil.RunStep(t, "all load on 1 server", func(t *testing.T) {
		adj.SetServerCount(1)
		require.Equal(t, 111, limiter.receive(t))
	})

	testutil.RunStep(t, "delete proxy service", func(t *testing.T) {
		require.NoError(t, store.DeleteService(index+1, "ingress-gateway-0-node-0", "ingress-gateway-0", acl.DefaultEnterpriseMeta(), structs.DefaultPeerKeyword))
		require.Equal(t, 109, limiter.receive(t))
	})

	testutil.RunStep(t, "check we're emitting gauge", func(t *testing.T) {
		data := sink.Data()
		require.Len(t, data, 1)

		gauge, ok := data[0].Gauges["consul.xds.server.idealStreamsMax"]
		require.True(t, ok)
		require.Equal(t, float32(109), gauge.Value)
	})
}

func newTestLimiter() *testLimiter {
	return &testLimiter{ch: make(chan uint32, 1)}
}

type testLimiter struct{ ch chan uint32 }

func (tl *testLimiter) SetMaxSessions(max uint32) { tl.ch <- max }

func (tl *testLimiter) receive(t *testing.T) int {
	select {
	case v := <-tl.ch:
		return int(v)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for SetMaxSessions")
	}
	panic("this should never be reached")
}

func (tl *testLimiter) SetDrainRateLimit(rateLimit rate.Limit) {}

func TestCalcRateLimit(t *testing.T) {
	for in, out := range map[uint32]rate.Limit{
		0:          rate.Limit(1),
		1:          rate.Limit(1),
		512:        rate.Limit(1),
		768:        rate.Limit(2),
		1024:       rate.Limit(3),
		2816:       rate.Limit(10),
		1000000000: rate.Limit(10),
	} {
		require.Equalf(t, out, calcRateLimit(in), "calcRateLimit(%d)", in)
	}
}
