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

const (
	coalesceTimeout       = 200 * time.Millisecond
	rootsWatchID          = "roots"
	leafWatchID           = "leaf"
	intentionsWatchID     = "intentions"
	serviceIDPrefix       = string(structs.UpstreamDestTypeService) + ":"
	preparedQueryIDPrefix = string(structs.UpstreamDestTypePreparedQuery) + ":"
)

// state holds all the state needed to maintain the config for a registered
// connect-proxy service. When a proxy registration is changed, the entire state
// is discarded and a new one created.
type state struct {
	// logger, source and cache are required to be set before calling Watch.
	logger *log.Logger
	source *structs.QuerySource
	cache  *cache.Cache

	// ctx and cancel store the context created during initWatches call
	ctx    context.Context
	cancel func()

	proxyID  string
	address  string
	port     int
	proxyCfg structs.ConnectProxyConfig
	token    string

	ch     chan cache.UpdateEvent
	snapCh chan ConfigSnapshot
	reqCh  chan chan *ConfigSnapshot
}

// newState populates the state struct by copying relevant fields from the
// NodeService and Token. We copy so that we can use them in a separate
// goroutine later without reasoning about races with the NodeService passed
// (especially for embedded fields like maps and slices).
//
// The returned state needs it's required dependencies to be set before Watch
// can be called.
func newState(ns *structs.NodeService, token string) (*state, error) {
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

	return &state{
		proxyID:  ns.ID,
		address:  ns.Address,
		port:     ns.Port,
		proxyCfg: proxyCfg,
		token:    token,
		// 10 is fairly arbitrary here but allow for the 3 mandatory and a
		// reasonable number of upstream watches to all deliver their initial
		// messages in parallel without blocking the cache.Notify loops. It's not a
		// huge deal if we do for a short period so we don't need to be more
		// conservative to handle larger numbers of upstreams correctly but gives
		// some head room for normal operation to be non-blocking in most typical
		// cases.
		ch:     make(chan cache.UpdateEvent, 10),
		snapCh: make(chan ConfigSnapshot, 1),
		reqCh:  make(chan chan *ConfigSnapshot, 1),
	}, nil
}

// Watch initialised watches on all necessary cache data for the current proxy
// registration state and returns a chan to observe updates to the
// ConfigSnapshot that contains all necessary config state. The chan is closed
// when the state is Closed.
func (s *state) Watch() (<-chan ConfigSnapshot, error) {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	err := s.initWatches()
	if err != nil {
		s.cancel()
		return nil, err
	}

	go s.run()

	return s.snapCh, nil
}

// Close discards the state and stops any long-running watches.
func (s *state) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// initWatches sets up the watches needed based on current proxy registration
// state.
func (s *state) initWatches() error {
	// Watch for root changes
	err := s.cache.Notify(s.ctx, cachetype.ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
	}, rootsWatchID, s.ch)
	if err != nil {
		return err
	}

	// Watch the leaf cert
	err = s.cache.Notify(s.ctx, cachetype.ConnectCALeafName, &cachetype.ConnectCALeafRequest{
		Datacenter: s.source.Datacenter,
		Token:      s.token,
		Service:    s.proxyCfg.DestinationServiceName,
	}, leafWatchID, s.ch)
	if err != nil {
		return err
	}

	// Watch for intention updates
	err = s.cache.Notify(s.ctx, cachetype.IntentionMatchName, &structs.IntentionQueryRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: structs.IntentionDefaultNamespace,
					Name:      s.proxyCfg.DestinationServiceName,
				},
			},
		},
	}, intentionsWatchID, s.ch)
	if err != nil {
		return err
	}

	// Watch for updates to service endpoints for all upstreams
	for _, u := range s.proxyCfg.Upstreams {
		dc := s.source.Datacenter
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
		case "": // Treat unset as the default Service type
			err = s.cache.Notify(s.ctx, cachetype.HealthServicesName, &structs.ServiceSpecificRequest{
				Datacenter:   dc,
				QueryOptions: structs.QueryOptions{Token: s.token},
				ServiceName:  u.DestinationName,
				Connect:      true,
			}, u.Identifier(), s.ch)

			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown upstream type: %q", u.DestinationType)
		}
	}
	return nil
}

func (s *state) run() {
	// Close the channel we return from Watch when we stop so consumers can stop
	// watching and clean up their goroutines. It's important we do this here and
	// not in Close since this routine sends on this chan and so might panic if it
	// gets closed from another goroutine.
	defer close(s.snapCh)

	snap := ConfigSnapshot{
		ProxyID:           s.proxyID,
		Address:           s.address,
		Port:              s.port,
		Proxy:             s.proxyCfg,
		UpstreamEndpoints: make(map[string]structs.CheckServiceNodes),
	}
	// This turns out to be really fiddly/painful by just using time.Timer.C
	// directly in the code below since you can't detect when a timer is stopped
	// vs waiting in order to know to reset it. So just use a chan to send
	// ourselves messages.
	sendCh := make(chan struct{})
	var coalesceTimer *time.Timer

	for {
		select {
		case <-s.ctx.Done():
			return
		case u := <-s.ch:
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
			// Allow the next change to trigger a send
			coalesceTimer = nil

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
			if coalesceTimer == nil {
				coalesceTimer = time.AfterFunc(coalesceTimeout, func() {
					// This runs in another goroutine so we can't just do the send
					// directly here as access to snap is racy. Instead, signal the main
					// loop above.
					sendCh <- struct{}{}
				})
			}
		}
	}
}

func (s *state) handleUpdate(u cache.UpdateEvent, snap *ConfigSnapshot) error {
	switch u.CorrelationID {
	case rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for roots response: %T", u.Result)
		}
		snap.Roots = roots
	case leafWatchID:
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for leaf response: %T", u.Result)
		}
		snap.Leaf = leaf
	case intentionsWatchID:
		// Not in snapshot currently, no op
	default:
		// Service discovery result, figure out which type
		switch {
		case strings.HasPrefix(u.CorrelationID, serviceIDPrefix):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for service response: %T", u.Result)
			}
			snap.UpstreamEndpoints[u.CorrelationID] = resp.Nodes

		case strings.HasPrefix(u.CorrelationID, preparedQueryIDPrefix):
			resp, ok := u.Result.(*structs.PreparedQueryExecuteResponse)
			if !ok {
				return fmt.Errorf("invalid type for prepared query response: %T", u.Result)
			}
			snap.UpstreamEndpoints[u.CorrelationID] = resp.Nodes

		default:
			return errors.New("unknown correlation ID")
		}
	}
	return nil
}

// CurrentSnapshot synchronously returns the current ConfigSnapshot if there is
// one ready. If we don't have one yet because not all necessary parts have been
// returned (i.e. both roots and leaf cert), nil is returned.
func (s *state) CurrentSnapshot() *ConfigSnapshot {
	// Make a chan for the response to be sent on
	ch := make(chan *ConfigSnapshot, 1)
	s.reqCh <- ch
	// Wait for the response
	return <-ch
}

// Changed returns whether or not the passed NodeService has had any of the
// fields we care about for config state watching changed or a different token.
func (s *state) Changed(ns *structs.NodeService, token string) bool {
	if ns == nil {
		return true
	}
	return ns.Kind != structs.ServiceKindConnectProxy ||
		s.proxyID != ns.ID ||
		s.address != ns.Address ||
		s.port != ns.Port ||
		!reflect.DeepEqual(s.proxyCfg, ns.Proxy) ||
		s.token != token
}
