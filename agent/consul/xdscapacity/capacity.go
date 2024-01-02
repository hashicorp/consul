// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdscapacity

import (
	"context"
	"math"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/retry"
)

var StatsGauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"xds", "server", "idealStreamsMax"},
		Help: "The maximum number of xDS streams per server, chosen to achieve a roughly even spread of load across servers.",
	},
}

// errorMargin is amount to which we allow a server to be over-occupied,
// expressed as a percentage (between 0 and 1).
//
// We allow 10% more than the ideal number of streams per server.
const errorMargin = 0.1

// Controller determines the ideal number of xDS streams for the server to
// handle and enforces it using the given SessionLimiter.
//
// We aim for a roughly even spread of streams between servers in the cluster
// and, to that end, limit the number of streams each server can handle to:
//
//	(<number of proxies> / <number of healthy servers>) + <error margin>
//
// Controller receives changes to the number of healthy servers from the
// autopilot delegate. It queries the state store's catalog tables to discover
// the number of registered proxy (sidecar and gateway) services.
type Controller struct {
	cfg Config

	serverCh chan uint32
	doneCh   chan struct{}

	prevMaxSessions uint32
	prevRateLimit   rate.Limit
}

// Config contains the dependencies for Controller.
type Config struct {
	Logger         hclog.Logger
	GetStore       func() Store
	SessionLimiter SessionLimiter
}

// SessionLimiter is used to enforce the session limit to achieve the ideal
// spread of xDS streams between servers.
type SessionLimiter interface {
	SetMaxSessions(maxSessions uint32)
	SetDrainRateLimit(rateLimit rate.Limit)
}

// NewController creates a new capacity controller with the given config.
//
// Call Run to start the control-loop.
func NewController(cfg Config) *Controller {
	return &Controller{
		cfg:      cfg,
		serverCh: make(chan uint32),
		doneCh:   make(chan struct{}),
	}
}

// Run the control-loop until the given context is canceled or reaches its
// deadline.
func (c *Controller) Run(ctx context.Context) {
	defer close(c.doneCh)

	watchCh, numProxies, err := c.countProxies(ctx)
	if err != nil {
		return
	}

	var numServers uint32
	for {
		select {
		case s := <-c.serverCh:
			numServers = s
			c.updateMaxSessions(numServers, numProxies)
		case <-watchCh:
			watchCh, numProxies, err = c.countProxies(ctx)
			if err != nil {
				return
			}
			c.updateDrainRateLimit(numProxies)
			c.updateMaxSessions(numServers, numProxies)
		case <-ctx.Done():
			return
		}
	}
}

// SetServerCount updates the number of healthy servers that is used when
// determining capacity. It is called by the autopilot delegate.
func (c *Controller) SetServerCount(count uint32) {
	select {
	case c.serverCh <- count:
	case <-c.doneCh:
	}
}

func (c *Controller) updateDrainRateLimit(numProxies uint32) {
	rateLimit := calcRateLimit(numProxies)
	if rateLimit == c.prevRateLimit {
		return
	}

	c.cfg.Logger.Debug("updating drain rate limit", "rate_limit", rateLimit)
	c.cfg.SessionLimiter.SetDrainRateLimit(rateLimit)
	c.prevRateLimit = rateLimit
}

// We dynamically scale the rate at which excess sessions will be drained
// according to the number of proxies in the catalog.
//
// The numbers here are pretty arbitrary (change them if you find better ones!)
// but the logic is:
//
//	0-512 proxies: drain 1 per second
//	513-2815 proxies: linearly scaled by 1/s for every additional 256 proxies
//	2816+ proxies: drain 10 per second
func calcRateLimit(numProxies uint32) rate.Limit {
	perSecond := math.Floor((float64(numProxies) - 256) / 256)

	if perSecond < 1 {
		return 1
	}

	if perSecond > 10 {
		return 10
	}

	return rate.Limit(perSecond)
}

func (c *Controller) updateMaxSessions(numServers, numProxies uint32) {
	if numServers == 0 || numProxies == 0 {
		return
	}

	maxSessions := uint32(math.Ceil((float64(numProxies) / float64(numServers)) * (1 + errorMargin)))
	if maxSessions == c.prevMaxSessions {
		return
	}

	c.cfg.Logger.Debug(
		"updating max sessions",
		"max_sessions", maxSessions,
		"num_servers", numServers,
		"num_proxies", numProxies,
	)
	metrics.SetGauge([]string{"xds", "server", "idealStreamsMax"}, float32(maxSessions))
	c.cfg.SessionLimiter.SetMaxSessions(maxSessions)
	c.prevMaxSessions = maxSessions
}

// countProxies counts the number of registered proxy services, retrying on
// error until the given context is cancelled.
func (c *Controller) countProxies(ctx context.Context) (<-chan error, uint32, error) {
	retryWaiter := &retry.Waiter{
		MinFailures: 1,
		MinWait:     1 * time.Second,
		MaxWait:     1 * time.Minute,
	}

	for {
		store := c.cfg.GetStore()

		ws := memdb.NewWatchSet()
		ws.Add(store.AbandonCh())

		var count uint32
		_, usage, err := store.ServiceUsage(ws)

		// Query failed? Wait for a while, and then go to the top of the loop to
		// retry (unless the context is cancelled).
		if err != nil {
			if err := retryWaiter.Wait(ctx); err != nil {
				return nil, 0, err
			}
			continue
		}

		for kind, kindCount := range usage.ConnectServiceInstances {
			if structs.ServiceKind(kind).IsProxy() {
				count += uint32(kindCount)
			}
		}
		return ws.WatchCh(ctx), count, nil
	}
}

type Store interface {
	AbandonCh() <-chan struct{}
	ServiceUsage(ws memdb.WatchSet) (uint64, structs.ServiceUsage, error)
}
