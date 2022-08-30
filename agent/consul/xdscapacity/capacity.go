package xdscapacity

import (
	"context"
	"math"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

var StatsGauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"xds", "server", "idealSessionsMax"},
		Help: "The ideal maximum number of xDS sessions per server.",
	},
}

// errorMargin is amount to which we allow a server to be over-occupied,
// expressed as a percentage (between 0 and 1).
//
// We allow 10% more than the ideal number of sessions per server.
const errorMargin = 0.1

// Controller determines the ideal number of xDS sessions for the server to
// handle and enforces it using the given SessionLimiter.
//
// We aim for a roughly even spread of sessions between servers in the cluster
// and, to that end, limit the number of sessions each server can handle to:
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
}

// Config contains the dependencies for Controller.
type Config struct {
	Logger         hclog.Logger
	GetStore       func() Store
	SessionLimiter SessionLimiter
}

// SessionLimiter is used to enforce the session limit to achieve the ideal
// spread of xDS sessions between servers.
type SessionLimiter interface {
	SetMaxSessions(maxSessions uint32)
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
func (a *Controller) Run(ctx context.Context) {
	defer close(a.doneCh)

	ws, numProxies, err := a.countProxies()
	if err != nil {
		a.cfg.Logger.Error("failed to count proxy services", "error", err)
	}

	var numServers, prevMaxSessions uint32
	update := func() {
		if numServers == 0 && numProxies == 0 {
			return
		}
		maxSessions := uint32(math.Ceil((float64(numProxies) / float64(numServers)) * (1 + errorMargin)))
		if prevMaxSessions == maxSessions {
			return
		}
		a.cfg.Logger.Debug(
			"updating max sessions",
			"max_sessions", maxSessions,
			"num_servers", numServers,
			"num_proxies", numProxies,
		)
		metrics.SetGauge([]string{"xds", "server", "idealSessionsMax"}, float32(maxSessions))
		a.cfg.SessionLimiter.SetMaxSessions(maxSessions)
		prevMaxSessions = maxSessions
	}

	for {
		select {
		case s := <-a.serverCh:
			numServers = s
			update()
		case <-ws.WatchCh(ctx):
			var count uint32
			ws, count, err = a.countProxies()
			if err == nil {
				numProxies = count
				update()
			} else {
				a.cfg.Logger.Error("failed to count proxy services", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// SetServerCount updates the number of healthy servers that is used when
// determining capacity. It is called by the autopilot delegate.
func (a *Controller) SetServerCount(count uint32) {
	select {
	case a.serverCh <- count:
	case <-a.doneCh:
	}
}

func (a *Controller) countProxies() (memdb.WatchSet, uint32, error) {
	store := a.cfg.GetStore()

	ws := memdb.NewWatchSet()
	ws.Add(store.AbandonCh())

	var count uint32
	_, csns, err := store.ServiceDump(
		ws,
		"",
		false,
		structs.WildcardEnterpriseMetaInPartition(acl.WildcardName),
		structs.DefaultPeerKeyword,
	)
	if err != nil {
		return ws, 0, err
	}
	for _, csn := range csns {
		if csn.Service.Kind.IsProxy() {
			count++
		}
	}
	return ws, count, nil
}

type Store interface {
	watch.StateStore

	ServiceDump(ws memdb.WatchSet, kind structs.ServiceKind, useKind bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
	PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error)
}
