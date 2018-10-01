package proxycfg

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/copystructure"
)

const coallesceTimeout = 200 * time.Millisecond

// State holds all the state needed to maintain the config for a registeres
// connect-proxy service. When a proxy registration is changed, the entire state
// is discarded and a new one created.
type State struct {
	ctx    context.Context
	cancel func()

	proxyID  string
	address  string
	port     int
	proxyCfg structs.ConnectProxyConfig
	token    string

	ch chan cache.UpdateEvent

	snapCh chan ConfigSnapshot
	reqCh  chan chan *ConfigSnapshot

	logger *log.Logger
	source *structs.QuerySource
}

// ConfigSnapshot captures all the resulting config needed for a proxy instance.
// It is meant to be point-in-time coherent and is used to deliver the current
// config state to observers who need it to be pushed in (e.g. XDS server).
type ConfigSnapshot struct {
	ProxyID           string
	Address           string
	Port              int
	Proxy             structs.ConnectProxyConfig
	Roots             *structs.IndexedCARoots
	Leaf              *structs.IssuedCert
	UpstreamEndpoints map[string]structs.CheckServiceNodes

	// Skip intentions for now as we don't push those down yet, just pre-warm them.
}

// Valid returns whether or not the snapshot has all required fields filled yet.
func (s *ConfigSnapshot) Valid() bool {
	return s.Roots != nil && s.Leaf != nil
}

// Clone makes a deep copy of the snapshot we can send to other goroutines
// without worrying that they will racily read or mutate shared maps etc.
func (s *ConfigSnapshot) Clone() (*ConfigSnapshot, error) {
	snapCopy, err := copystructure.Copy(s)
	if err != nil {
		return nil, err
	}
	return snapCopy.(*ConfigSnapshot), nil
}

// NewState populates the state from a NodeService struct. It will start up
// watchers on any required resources for the proxy which are not released until
// Close is called. The NodeService is assumed not to change for the lifetime of
// the State; when it is, this state should be closed and a new one started.
// It's also assumed that it's safe to read from it without coordination in the
// current goroutine. The pointer is NOT preserved and accessed again later from
// another goroutine. This is simpler than diffing states and the performance
// overhead is not a big concern for now.
func NewState(logger *log.Logger, c *cache.Cache, source *structs.QuerySource,
	ns *structs.NodeService, token string) (*State, error) {
	if ns.Kind != structs.ServiceKindConnectProxy {
		return nil, errors.New("not a connect-proxy")
	}

	// Copy the config map
	proxyCfgRaw, err := copystructure.Copy(ns.Proxy)
	if err != nil {
		return nil, err
	}
	proxyCfg, ok := proxyCfgRaw.(structs.ConnectProxyConfig)
	if !ok {
		return nil, errors.New("failed to copy proxy config")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Setup the watchers we need while we have the ns in memory. We don't want to
	// read from it later in another goroutine and have to reason about races with
	// the calling code.

	ch := make(chan cache.UpdateEvent, 10)

	// Watch for root changes
	err = c.Notify(ctx, cachetype.ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter:   source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: token},
	}, "roots", ch)
	if err != nil {
		cancel()
		return nil, err
	}

	// Watch the leaf cert
	err = c.Notify(ctx, cachetype.ConnectCALeafName, &cachetype.ConnectCALeafRequest{
		Datacenter: source.Datacenter,
		Token:      token,
		Service:    ns.Proxy.DestinationServiceName,
	}, "leaf", ch)
	if err != nil {
		cancel()
		return nil, err
	}

	// Watch for intention updates
	err = c.Notify(ctx, cachetype.IntentionMatchName, &structs.IntentionQueryRequest{
		Datacenter:   source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: token},
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: structs.IntentionDefaultNamespace,
					Name:      ns.Service,
				},
			},
		},
	}, "intentions", ch)
	if err != nil {
		cancel()
		return nil, err
	}

	// Watch for updates to service endpoints for all upstreams
	for _, u := range ns.Proxy.Upstreams {
		dc := source.Datacenter
		if u.Datacenter != "" {
			dc = u.Datacenter
		}

		switch u.DestinationType {
		case structs.UpstreamDestTypePreparedQuery:
			// TODO(banks): prepared queries don't support blocking. We need to come
			// up with an alternative to Notify that will poll at a sensible rate.

			// err = c.Notify(ctx, cachetype.PreparedQueryName, &structs.PreparedQueryExecuteRequest{
			//  Datacenter:    dc,
			//  QueryOptions:  structs.QueryOptions{Token: token},
			//  QueryIDOrName: u.DestinationName,
			//  Connect: true,
			// }, u.Identifier(), ch)
		case structs.UpstreamDestTypeService:
			fallthrough
		default:
			err = c.Notify(ctx, cachetype.HealthServicesName, &structs.ServiceSpecificRequest{
				Datacenter:   dc,
				QueryOptions: structs.QueryOptions{Token: token},
				ServiceName:  u.DestinationName,
				Connect:      true,
			}, u.Identifier(), ch)
		}

		if err != nil {
			cancel()
			return nil, err
		}
	}

	s := &State{
		ctx:      ctx,
		cancel:   cancel,
		proxyID:  ns.ID,
		address:  ns.Address,
		port:     ns.Port,
		proxyCfg: proxyCfg,
		token:    token,
		ch:       ch,
		snapCh:   make(chan ConfigSnapshot, 10),
		reqCh:    make(chan chan *ConfigSnapshot, 1),
		logger:   logger,
		source:   source,
	}

	go s.run()
	return s, nil
}

// Watch returns a chan of config snapshots
func (s *State) Watch() <-chan ConfigSnapshot {
	return s.snapCh
}

func (s *State) run() {
	snap := ConfigSnapshot{
		ProxyID:           s.proxyID,
		Address:           s.address,
		Port:              s.port,
		Proxy:             s.proxyCfg,
		UpstreamEndpoints: make(map[string]structs.CheckServiceNodes),
	}
	// This turns out to be really fiddly/painful by just using time.Timer.C
	// directly in the code below since you can't detect when a timer is stopped
	// vs waiting in order to now to reset it. So just use a chan for ourselves!
	sendCh := make(chan struct{})
	var coallesceTimer *time.Timer

	for {
		select {
		case <-s.ctx.Done():
			return
		case u, ok := <-s.ch:
			if !ok {
				// Only way this can happen is if the context was cancelled and Notify's
				// goroutine returned and closed this. Bail out.
				return
			}
			if err := s.handleUpdate(u, &snap); err != nil {
				s.logger.Printf("[ERR] %s watch error: %s", u.CorrelationID, err)
				continue
			}

		case <-sendCh:
			// Make a deep copy of snap so we don't mutate any of the embedded structs
			// etc on future updates.
			snapCopy, err := snap.Clone()
			if err != nil {
				s.logger.Printf("[ERR] Failed to copy config snapshot for proxy %s",
					s.proxyID)
				continue
			}
			s.snapCh <- *snapCopy
			// Allow the next send
			coallesceTimer = nil

			// Skip rest of loop - there is nothing to send since nothing changed on
			// this iteration
			continue

		case replyCh := <-s.reqCh:
			if !snap.Valid() {
				// Not valid yet just respond with nil and move on to next task.
				replyCh <- nil
				continue
			}
			// Make a deep copy of snap so we don't mutate any of the embedded structs
			// etc on future updates.
			snapCopy, err := snap.Clone()
			if err != nil {
				s.logger.Printf("[ERR] Failed to copy config snapshot for proxy %s",
					s.proxyID)
				continue
			}
			replyCh <- snapCopy

			// Skip rest of loop - there is nothing to send since nothing changed on
			// this iteration
			continue
		}

		// Check if snap is complete enough to be a valid config to deliver to a
		// proxy yet.
		if snap.Valid() {
			// Don't send it right away, set a short timer that will wait for updates
			// from any of the other cache values and deliver them all together.
			if coallesceTimer == nil {
				coallesceTimer = time.AfterFunc(coallesceTimeout, func() {
					// This runs in another goroutine so we can't just do the send
					// directly here as access to snap is racy so just signal the main
					// loop above.
					sendCh <- struct{}{}
				})
			}
		}
	}
}

func (s *State) handleUpdate(u cache.UpdateEvent, snap *ConfigSnapshot) error {
	switch u.CorrelationID {
	case "roots":
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for roots response: %T", u.Result)
		}
		snap.Roots = roots
	case "leaf":
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for leaf response: %T", u.Result)
		}
		snap.Leaf = leaf
	case "intentions":
		// Not in snapshot currently, no op
	default:
		// Service discovery result, figure out which type
		if strings.HasPrefix(u.CorrelationID, "service:") {
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for service response: %T", u.Result)
			}
			snap.UpstreamEndpoints[u.CorrelationID] = resp.Nodes
		} else if strings.HasPrefix(u.CorrelationID, "prepared_query:") {
			resp, ok := u.Result.(*structs.PreparedQueryExecuteResponse)
			if !ok {
				return fmt.Errorf("invalid type for prepared query response: %T", u.Result)
			}
			snap.UpstreamEndpoints[u.CorrelationID] = resp.Nodes
		} else {
			return errors.New("unknown correlation ID")
		}
	}
	return nil
}

// CurrentSnapshot synchronously returns the current ConfigSnapshot if there is
// one ready. If we don't have one yet because not all necessary parts have been
// returned (i.e. both roots and leaf cert), nil is returned.
func (s *State) CurrentSnapshot() *ConfigSnapshot {
	// Make a chan for the response to be sent on
	ch := make(chan *ConfigSnapshot, 1)
	s.reqCh <- ch
	// Wait for the response
	return <-ch
}

// Close discards the state and stops any long-running watches.
func (s *State) Close() error {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.snapCh != nil {
		close(s.snapCh)
		s.snapCh = nil
	}
	return nil
}

// Changed returns whether or not the passed NodeService has had any of the
// fields we care about for config state watching changed or a different token.
func (s *State) Changed(ns *structs.NodeService, token string) bool {
	return ns.Kind != structs.ServiceKindConnectProxy ||
		s.proxyID != ns.ID ||
		s.address != ns.Address ||
		s.port != ns.Port ||
		!reflect.DeepEqual(s.proxyCfg, ns.Proxy) ||
		s.token != token
}
