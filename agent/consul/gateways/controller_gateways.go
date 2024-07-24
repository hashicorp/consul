// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package gateways

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

var (
	errServiceDoesNotExist = errors.New("service does not exist")
	errInvalidProtocol     = errors.New("route protocol does not match targeted service protocol")
)

// Updater is a thin wrapper around a set of callbacks used for updating
// and deleting config entries via raft operations.
type Updater struct {
	UpdateWithStatus func(entry structs.ControlledConfigEntry) error
	Update           func(entry structs.ConfigEntry) error
	Delete           func(entry structs.ConfigEntry) error
}

// apiGatewayReconciler is the monolithic reconciler used for reconciling
// all of our routes and gateways into bound gateway state.
type apiGatewayReconciler struct {
	fsm        *fsm.FSM
	logger     hclog.Logger
	updater    *Updater
	controller controller.Controller
}

// Reconcile is the main reconciliation function for the gateway reconciler, it
// delegates each reconciliation request to functions designated for a
// particular type of config entry.
func (r *apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	// We do this in a single threaded way to avoid race conditions around setting
	// shared state. In our current out-of-repo code, this is handled via a global
	// lock on our shared store, but this makes it so we don't have to deal with lock
	// contention, and instead just work with a single control loop.
	switch req.Kind {
	case structs.APIGateway:
		return reconcileEntry(r.fsm.State(), r.logger, ctx, req, r.reconcileGateway, r.cleanupGateway)
	case structs.BoundAPIGateway:
		return reconcileEntry(r.fsm.State(), r.logger, ctx, req, r.reconcileBoundGateway, r.cleanupBoundGateway)
	case structs.HTTPRoute:
		return reconcileEntry(r.fsm.State(), r.logger, ctx, req, r.reconcileHTTPRoute, r.cleanupRoute)
	case structs.TCPRoute:
		return reconcileEntry(r.fsm.State(), r.logger, ctx, req, r.reconcileTCPRoute, r.cleanupRoute)
	case structs.InlineCertificate, structs.FileSystemCertificate:
		return r.enqueueCertificateReferencedGateways(r.fsm.State(), ctx, req)
	case structs.JWTProvider:
		return r.enqueueJWTProviderReferencedGatewaysAndHTTPRoutes(r.fsm.State(), ctx, req)
	default:
		return nil
	}
}

// reconcileEntry converts the controller request into a config entry that we then pass
// along to either a cleanup function if the entry no longer exists (it's been deleted),
// or a reconciler if the entry has been updated or created.
func reconcileEntry[T structs.ControlledConfigEntry](store *state.Store, logger hclog.Logger, ctx context.Context, req controller.Request, reconciler func(ctx context.Context, req controller.Request, store *state.Store, entry T) error, cleaner func(ctx context.Context, req controller.Request, store *state.Store) error) error {
	_, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, req.Meta)
	if err != nil {
		requestLogger(logger, req).Warn("error fetching config entry for reconciliation request", "error", err)
		return err
	}

	if entry == nil {
		return cleaner(ctx, req, store)
	}

	return reconciler(ctx, req, store, entry.(T))
}

// enqueueCertificateReferencedGateways retrieves all gateway objects, filters to those referencing
// the provided certificate, and enqueues the gateways for reconciliation
func (r *apiGatewayReconciler) enqueueCertificateReferencedGateways(store *state.Store, _ context.Context, req controller.Request) error {
	logger := certificateRequestLogger(r.logger, req)

	logger.Trace("certificate changed, enqueueing dependent gateways")
	defer logger.Trace("finished enqueuing gateways")

	_, entries, err := store.ConfigEntriesByKind(nil, structs.APIGateway, wildcardMeta())
	if err != nil {
		logger.Warn("error retrieving api gateways", "error", err)
		return err
	}

	requests := []controller.Request{}

	for _, entry := range entries {
		gateway := entry.(*structs.APIGatewayConfigEntry)
		for _, listener := range gateway.Listeners {
			for _, certificate := range listener.TLS.Certificates {
				if certificate.IsSame(&structs.ResourceReference{
					Kind:           req.Kind,
					Name:           req.Name,
					EnterpriseMeta: *req.Meta,
				}) {
					requests = append(requests, controller.Request{
						Kind: structs.APIGateway,
						Name: gateway.Name,
						Meta: &gateway.EnterpriseMeta,
					})
				}
			}
		}
	}

	r.controller.Enqueue(requests...)

	return nil
}

// cleanupBoundGateway retrieves all routes from the store and removes the gateway from any
// routes that are bound to it, updating their status appropriately
func (r *apiGatewayReconciler) cleanupBoundGateway(_ context.Context, req controller.Request, store *state.Store) error {
	logger := gatewayRequestLogger(r.logger, req)

	logger.Trace("cleaning up bound gateway")
	defer logger.Trace("finished cleaning up bound gateway")

	routes, err := retrieveAllRoutesFromStore(store)
	if err != nil {
		logger.Warn("error retrieving routes", "error", err)
		return err
	}

	resource := requestToResourceRef(req)
	resource.Kind = structs.APIGateway

	for _, modifiedRoute := range removeGateway(resource, routes...) {
		routeLogger := routeLogger(logger, modifiedRoute)
		routeLogger.Trace("persisting route status")
		if err := r.updater.Update(modifiedRoute); err != nil {
			routeLogger.Warn("error removing gateway from route", "error", err)
			return err
		}
	}

	return nil
}

// reconcileBoundGateway mainly handles orphaned bound gateways at startup, it just checks
// to make sure there's still an existing gateway, and if not, it deletes the bound gateway
func (r *apiGatewayReconciler) reconcileBoundGateway(_ context.Context, req controller.Request, store *state.Store, bound *structs.BoundAPIGatewayConfigEntry) error {
	logger := gatewayRequestLogger(r.logger, req)

	logger.Trace("reconciling bound gateway")
	defer logger.Trace("finished reconciling bound gateway")

	_, gateway, err := store.ConfigEntry(nil, structs.APIGateway, req.Name, req.Meta)
	if err != nil {
		logger.Warn("error retrieving api gateway", "error", err)
		return err
	}

	if gateway == nil {
		// delete the bound gateway
		logger.Trace("deleting bound api gateway")
		if err := r.updater.Delete(bound); err != nil {
			logger.Warn("error deleting bound api gateway", "error", err)
			return err
		}
	}

	return nil
}

// cleanupGateway deletes the associated bound gateway state with the config entry, route
// cleanup occurs when the bound gateway is re-reconciled or on the next reconciliation
// pass for the route.
func (r *apiGatewayReconciler) cleanupGateway(_ context.Context, req controller.Request, store *state.Store) error {
	logger := gatewayRequestLogger(r.logger, req)

	logger.Trace("cleaning up deleted gateway")
	defer logger.Trace("finished cleaning up deleted gateway")

	_, bound, err := store.ConfigEntry(nil, structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		logger.Warn("error retrieving bound api gateway", "error", err)
		return err
	}

	logger.Trace("deleting bound api gateway")
	if err := r.updater.Delete(bound); err != nil {
		logger.Warn("error deleting bound api gateway", "error", err)
		return err
	}

	return nil
}

// reconcileGateway attempts to initialize or fetch the associated bound
// gateway state, fetch all route references, validate the existence of any
// referenced certificates, and then update the bound gateway with certificate
// references and add or remove any routes that reference or previously
// referenced this gateway. It then persists any status updates for the gateway,
// the modified routes, and updates the bound gateway.
func (r *apiGatewayReconciler) reconcileGateway(_ context.Context, req controller.Request, store *state.Store, gateway *structs.APIGatewayConfigEntry) error {
	logger := gatewayRequestLogger(r.logger, req)

	logger.Trace("started reconciling gateway")
	defer logger.Trace("finished reconciling gateway")

	updater := structs.NewStatusUpdater(gateway)
	// we clear out the initial status conditions since we're doing a full update
	// of this gateway's status
	updater.ClearConditions()

	routes, err := retrieveAllRoutesFromStore(store)
	if err != nil {
		logger.Warn("error retrieving routes", "error", err)
		return err
	}

	// construct the tuple we'll be working on to update state
	_, bound, err := store.ConfigEntry(nil, structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		logger.Warn("error retrieving bound api gateway", "error", err)
		return err
	}

	_, jwtProvidersConfigEntries, err := store.ConfigEntriesByKind(nil, structs.JWTProvider, wildcardMeta())
	if err != nil {
		return err
	}

	jwtProviders := make(map[string]*structs.JWTProviderConfigEntry, len(jwtProvidersConfigEntries))
	for _, provider := range jwtProvidersConfigEntries {
		jwtProviders[provider.GetName()] = provider.(*structs.JWTProviderConfigEntry)
	}

	meta := newGatewayMeta(gateway, bound, jwtProviders)

	certificateErrors, err := meta.checkCertificates(store)
	if err != nil {
		logger.Warn("error checking gateway certificates", "error", err)
		return err
	}

	jwtErrors, err := meta.checkJWTProviders()
	if err != nil {
		logger.Warn("error checking gateway JWT Providers", "error", err)
		return err
	}

	// set each listener as having resolved refs, then overwrite that status condition
	// if there are any certificate errors
	meta.eachListener(func(_ *structs.APIGatewayListener, bound *structs.BoundAPIGatewayListener) error {
		listenerRef := structs.ResourceReference{
			Kind:           structs.APIGateway,
			Name:           meta.BoundGateway.Name,
			SectionName:    bound.Name,
			EnterpriseMeta: meta.BoundGateway.EnterpriseMeta,
		}
		updater.SetCondition(resolvedRefs(listenerRef))
		return nil
	})

	for ref, err := range certificateErrors {
		updater.SetCondition(invalidCertificate(ref, err))
	}

	for ref, err := range jwtErrors {
		updater.SetCondition(invalidJWTProvider(ref, err))
	}

	if len(certificateErrors) > 0 {
		updater.SetCondition(invalidCertificates())
	}

	if len(jwtErrors) > 0 {
		updater.SetCondition(invalidJWTProviders())
	}

	if len(certificateErrors) == 0 && len(jwtErrors) == 0 {
		updater.SetCondition(gatewayAccepted())
	}

	// now we bind all of the routes we can
	updatedRoutes := []structs.ControlledConfigEntry{}
	for _, route := range routes {
		routeUpdater := structs.NewStatusUpdater(route)
		_, boundRefs, bindErrors := bindRoutesToGateways(route, meta)

		// unset the old gateway binding in case it's stale
		for _, parent := range route.GetParents() {
			if parent.Kind == gateway.Kind && parent.Name == gateway.Name && parent.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
				routeUpdater.RemoveCondition(routeBound(parent))
			}
		}

		// set the status for parents that have bound successfully
		for _, ref := range boundRefs {
			routeUpdater.SetCondition(routeBound(ref))
		}

		// set the status for any parents that have errored trying to
		// bind
		for ref, err := range bindErrors {
			routeUpdater.SetCondition(routeUnbound(ref, err))
		}

		// if we've updated any statuses, then store them as needing
		// to be updated
		if entry, updated := routeUpdater.UpdateEntry(); updated {
			updatedRoutes = append(updatedRoutes, entry)
		}
	}

	// first set any gateway conflict statuses
	meta.setConflicts(updater)

	// now check if we need to update the gateway status
	if modifiedGateway, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
		logger.Trace("persisting gateway status")
		if err := r.updater.UpdateWithStatus(modifiedGateway); err != nil {
			logger.Warn("error persisting gateway status", "error", err)
			return err
		}
	}

	// next update route statuses
	for _, modifiedRoute := range updatedRoutes {
		routeLogger := routeLogger(logger, modifiedRoute)
		routeLogger.Trace("persisting route status")
		if err := r.updater.UpdateWithStatus(modifiedRoute); err != nil {
			routeLogger.Warn("error persisting route status", "error", err)
			return err
		}
	}

	// now update the bound state if it changed
	if bound == nil || !bound.(*structs.BoundAPIGatewayConfigEntry).IsSame(meta.BoundGateway) {
		logger.Trace("persisting bound api gateway")
		if err := r.updater.Update(meta.BoundGateway); err != nil {
			logger.Warn("error persisting bound api gateway", "error", err)
			return err
		}
	}

	return nil
}

// cleanupRoute fetches all gateways and removes any existing reference to
// the route we're reconciling from them.
func (r *apiGatewayReconciler) cleanupRoute(_ context.Context, req controller.Request, store *state.Store) error {
	logger := routeRequestLogger(r.logger, req)

	logger.Trace("cleaning up route")
	defer logger.Trace("finished cleaning up route")

	meta, err := getAllGatewayMeta(store)
	if err != nil {
		logger.Warn("error retrieving gateways", "error", err)
		return err
	}

	for _, modifiedGateway := range removeRoute(requestToResourceRef(req), meta...) {
		gatewayLogger := gatewayLogger(logger, modifiedGateway.BoundGateway)
		gatewayLogger.Trace("persisting bound gateway state")
		if err := r.updater.Update(modifiedGateway.BoundGateway); err != nil {
			gatewayLogger.Warn("error updating bound api gateway", "error", err)
			return err
		}
	}

	r.controller.RemoveTrigger(req)

	return nil
}

// reconcileRoute attempts to validate a route against its referenced service
// discovery chain, it also fetches all gateways, and attempts to either remove
// the route being reconciled from gateways containing either stale references
// when this route no longer references them, or add the route to gateways that
// it now references. It then updates any necessary route statuses, checks for
// gateways that now have route conflicts, and updates all statuses and states
// as necessary.
func (r *apiGatewayReconciler) reconcileRoute(_ context.Context, req controller.Request, store *state.Store, route structs.BoundRoute) error {
	logger := routeRequestLogger(r.logger, req)

	logger.Trace("reconciling route")
	defer logger.Trace("finished reconciling route")

	meta, err := getAllGatewayMeta(store)
	if err != nil {
		logger.Warn("error retrieving gateways", "error", err)
		return err
	}

	updater := structs.NewStatusUpdater(route)
	// we clear out the initial status conditions since we're doing a full update
	// of this route's status
	updater.ClearConditions()

	ws := memdb.NewWatchSet()
	ws.Add(store.AbandonCh())

	finalize := func(modifiedGateways []*structs.BoundAPIGatewayConfigEntry) error {
		// first update any gateway statuses that are now in conflict
		for _, gateway := range meta {
			modifiedGateway, shouldUpdate := gateway.checkConflicts()
			if shouldUpdate {
				gatewayLogger := gatewayLogger(logger, modifiedGateway)
				gatewayLogger.Trace("persisting gateway status")
				if err := r.updater.UpdateWithStatus(modifiedGateway); err != nil {
					gatewayLogger.Warn("error persisting gateway", "error", err)
					return err
				}
			}
		}

		// next update the route status
		if modifiedRoute, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
			r.logger.Trace("persisting route status")
			if err := r.updater.UpdateWithStatus(modifiedRoute); err != nil {
				r.logger.Warn("error persisting route", "error", err)
				return err
			}
		}

		// now update all of the bound gateways that have been modified
		for _, bound := range modifiedGateways {
			gatewayLogger := gatewayLogger(logger, bound)
			gatewayLogger.Trace("persisting bound api gateway")
			if err := r.updater.Update(bound); err != nil {
				gatewayLogger.Warn("error persisting bound api gateway", "error", err)
				return err
			}
		}

		return nil
	}

	var triggerOnce sync.Once
	for _, service := range route.GetServiceNames() {
		_, chainSet, err := store.ReadDiscoveryChainConfigEntries(ws, service.Name, pointerTo(service.EnterpriseMeta))
		if err != nil {
			logger.Warn("error reading discovery chain", "error", err)
			return err
		}

		// trigger a watch since we now need to check when the discovery chain gets updated
		triggerOnce.Do(func() {
			r.controller.AddTrigger(req, ws.WatchCtx)
		})

		// make sure that we can actually compile a discovery chain based on this route
		// the main check is to make sure that all of the protocols align
		chain, err := discoverychain.Compile(discoverychain.CompileRequest{
			ServiceName:           service.Name,
			EvaluateInNamespace:   service.NamespaceOrDefault(),
			EvaluateInPartition:   service.PartitionOrDefault(),
			EvaluateInDatacenter:  "dc1",           // just mock out a fake dc since we're just checking for compilation errors
			EvaluateInTrustDomain: "consul.domain", // just mock out a fake trust domain since we're just checking for compilation errors
			Entries:               chainSet,
		})
		if err != nil {
			updater.SetCondition(routeInvalidDiscoveryChain(err))
			continue
		}

		if chain.Protocol != string(route.GetProtocol()) {
			updater.SetCondition(routeInvalidDiscoveryChain(errInvalidProtocol))
			continue
		}

		updater.SetCondition(routeAccepted())
	}

	// if we have no upstream targets, then set the route as invalid
	// this should already happen in the validation check on write, but
	// we'll do it here too just in case
	if len(route.GetServiceNames()) == 0 {
		updater.SetCondition(routeNoUpstreams())
	}

	// the route is valid, attempt to bind it to all gateways
	r.logger.Trace("binding routes to gateway")
	modifiedGateways, boundRefs, bindErrors := bindRoutesToGateways(route, meta...)

	// set the status of the references that are bound
	for _, ref := range boundRefs {
		updater.SetCondition(routeBound(ref))
	}

	// set any binding errors
	for ref, err := range bindErrors {
		updater.SetCondition(routeUnbound(ref, err))
	}

	// set any refs that haven't been bound or explicitly errored
PARENT_LOOP:
	for _, ref := range route.GetParents() {
		for _, boundRef := range boundRefs {
			if ref.IsSame(&boundRef) {
				continue PARENT_LOOP
			}
		}
		if _, ok := bindErrors[ref]; ok {
			continue PARENT_LOOP
		}
		updater.SetCondition(gatewayNotFound(ref))
	}

	return finalize(modifiedGateways)
}

// reconcileHTTPRoute is a thin wrapper around recnocileRoute for a HTTPRoutes
func (r *apiGatewayReconciler) reconcileHTTPRoute(ctx context.Context, req controller.Request, store *state.Store, route *structs.HTTPRouteConfigEntry) error {
	return r.reconcileRoute(ctx, req, store, route)
}

// reconcileTCPRoute is a thin wrapper around recnocileRoute for a TCPRoutes
func (r *apiGatewayReconciler) reconcileTCPRoute(ctx context.Context, req controller.Request, store *state.Store, route *structs.TCPRouteConfigEntry) error {
	return r.reconcileRoute(ctx, req, store, route)
}

// NewAPIGatewayController initializes a controller that reconciles all APIGateway objects
func NewAPIGatewayController(fsm *fsm.FSM, publisher state.EventPublisher, updater *Updater, logger hclog.Logger) controller.Controller {
	reconciler := &apiGatewayReconciler{
		fsm:     fsm,
		logger:  logger,
		updater: updater,
	}
	reconciler.controller = controller.New(publisher, reconciler).
		WithLogger(logger.With("controller", "apiGatewayController"))
	return reconciler.controller.Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicAPIGateway,
			Subject: stream.SubjectWildcard,
		},
	).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicHTTPRoute,
			Subject: stream.SubjectWildcard,
		},
	).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicTCPRoute,
			Subject: stream.SubjectWildcard,
		},
	).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicBoundAPIGateway,
			Subject: stream.SubjectWildcard,
		},
	).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicInlineCertificate,
			Subject: stream.SubjectWildcard,
		},
	).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicJWTProvider,
			Subject: stream.SubjectWildcard,
		})
}

// gatewayMeta embeds both a BoundAPIGateway and its corresponding APIGateway.
// This is used for binding routes to a gateway, because the binding logic
// requires correlation between fields on a gateway and a route, while persisting
// the state onto the corresponding subfields of a BoundAPIGateway. For example,
// when binding we need to validate that a route's protocol (e.g. http)
// matches the protocol of the listener it wants to bind to.
type gatewayMeta struct {
	// BoundGateway is the bound-api-gateway config entry for a given gateway.
	BoundGateway *structs.BoundAPIGatewayConfigEntry
	// Gateway is the api-gateway config entry for the gateway.
	Gateway *structs.APIGatewayConfigEntry
	// listeners is a map of gateway listeners by name for fast access
	// the map values are pointers so that we can update them directly
	// and have the changes propagate back to the container gateways.
	listeners map[string]*structs.APIGatewayListener
	// boundListeners is a map of gateway listeners by name for fast access
	// the map values are pointers so that we can update them directly
	// and have the changes propagate back to the container gateways.
	boundListeners map[string]*structs.BoundAPIGatewayListener
	// jwtProviders holds the list of all the JWT Providers in a given partition
	// we expect this list to be relatively small so we're okay with holding them all
	// in memory
	jwtProviders map[string]*structs.JWTProviderConfigEntry
}

// getAllGatewayMeta returns a pre-constructed list of all valid gateway and state
// tuples based on the state coming from the store. Any gateway that does not have
// a corresponding bound-api-gateway config entry will be filtered out.
func getAllGatewayMeta(store *state.Store) ([]*gatewayMeta, error) {
	_, gateways, err := store.ConfigEntriesByKind(nil, structs.APIGateway, wildcardMeta())
	if err != nil {
		return nil, err
	}
	_, boundGateways, err := store.ConfigEntriesByKind(nil, structs.BoundAPIGateway, wildcardMeta())
	if err != nil {
		return nil, err
	}

	_, jwtProvidersConfigEntries, err := store.ConfigEntriesByKind(nil, structs.JWTProvider, wildcardMeta())
	if err != nil {
		return nil, err
	}

	jwtProviders := make(map[string]*structs.JWTProviderConfigEntry, len(jwtProvidersConfigEntries))
	for _, provider := range jwtProvidersConfigEntries {
		jwtProviders[provider.GetName()] = provider.(*structs.JWTProviderConfigEntry)
	}

	meta := make([]*gatewayMeta, 0, len(boundGateways))
	for _, b := range boundGateways {
		bound := b.(*structs.BoundAPIGatewayConfigEntry)
		bound = bound.DeepCopy()
		for _, g := range gateways {
			gateway := g.(*structs.APIGatewayConfigEntry)
			if bound.IsInitializedForGateway(gateway) {
				meta = append(meta, (&gatewayMeta{
					BoundGateway: bound,
					Gateway:      gateway,
					jwtProviders: jwtProviders,
				}).initialize())
				break
			}
		}
	}
	return meta, nil
}

// updateRouteBinding takes a BoundRoute and modifies the listeners on the
// BoundAPIGateway config entry in GatewayMeta to reflect the binding of the
// route to the gateway.
//
// The return values correspond to:
// 1. whether the underlying BoundAPIGateway was actually modified
// 2. what references from the BoundRoute actually bound to the Gateway successfully
// 3. any errors that occurred while attempting to bind a particular reference to the Gateway
func (g *gatewayMeta) updateRouteBinding(route structs.BoundRoute) (bool, []structs.ResourceReference, map[structs.ResourceReference]error) {
	errors := make(map[structs.ResourceReference]error)

	boundRefs := []structs.ResourceReference{}
	listenerUnbound := make(map[string]bool, len(g.boundListeners))
	listenerBound := make(map[string]bool, len(g.boundListeners))

	routeRef := structs.ResourceReference{
		Kind:           route.GetKind(),
		Name:           route.GetName(),
		EnterpriseMeta: *route.GetEnterpriseMeta(),
	}

	// first attempt to unbind all of the routes from the listeners in case they're
	// stale
	g.eachListener(func(listener *structs.APIGatewayListener, bound *structs.BoundAPIGatewayListener) error {
		listenerUnbound[listener.Name] = bound.UnbindRoute(routeRef)
		return nil
	})

	if g.BoundGateway.Services == nil {
		g.BoundGateway.Services = make(structs.ServiceRouteReferences)
	}

	// now try and bind all of the route's current refs
	for _, ref := range route.GetParents() {
		if !g.shouldBindRoute(ref) {
			continue
		}

		if len(g.boundListeners) == 0 {
			errors[ref] = fmt.Errorf("route cannot bind because gateway has no listeners")
			continue
		}

		// try to bind to all listeners
		refDidBind := false
		g.eachListener(func(listener *structs.APIGatewayListener, bound *structs.BoundAPIGatewayListener) error {
			didBind, err := g.bindRoute(listener, bound, route, ref)
			if err != nil {
				errors[ref] = err
			}

			isValidJWT := true
			if httpRoute, ok := route.(*structs.HTTPRouteConfigEntry); ok {
				var jwtErrors map[structs.ResourceReference]error
				isValidJWT, jwtErrors = g.validateJWTForRoute(httpRoute)
				for ref, err := range jwtErrors {
					errors[ref] = err
				}
			}

			if didBind && isValidJWT {
				refDidBind = true
				listenerBound[listener.Name] = true
			}
			return nil
		})

		// double check that the wildcard ref actually bound to something
		if !refDidBind && errors[ref] == nil {
			errors[ref] = fmt.Errorf("failed to bind route %s to gateway %s with listener '%s'", route.GetName(), g.Gateway.Name, ref.SectionName)
		}

		if refDidBind {
			for _, serviceName := range route.GetServiceNames() {
				g.BoundGateway.Services.AddService(structs.NewServiceName(serviceName.Name, &serviceName.EnterpriseMeta), routeRef)
			}
			boundRefs = append(boundRefs, ref)
		}
	}

	didUpdate := false
	for name, didUnbind := range listenerUnbound {
		didBind := listenerBound[name]
		if didBind != didUnbind {
			didUpdate = true
			break
		}
	}

	return didUpdate, boundRefs, errors
}

// shouldBindRoute returns whether a Route's parent reference references the Gateway
// that we wrap.
func (g *gatewayMeta) shouldBindRoute(ref structs.ResourceReference) bool {
	return (ref.Kind == structs.APIGateway || ref.Kind == "") && g.Gateway.Name == ref.Name && g.Gateway.EnterpriseMeta.IsSame(&ref.EnterpriseMeta)
}

// shouldBindRouteToListener returns whether a Route's parent reference should attempt
// to bind to the given listener because it is either explicitly named or the Route
// is attempting to wildcard bind to the listener.
func (g *gatewayMeta) shouldBindRouteToListener(l *structs.BoundAPIGatewayListener, ref structs.ResourceReference) bool {
	return l.Name == ref.SectionName || ref.SectionName == ""
}

// bindRoute takes a particular listener that a Route is attempting to bind to with a given reference
// and returns whether the Route successfully bound to the listener or if it errored in the process.
func (g *gatewayMeta) bindRoute(listener *structs.APIGatewayListener, bound *structs.BoundAPIGatewayListener, route structs.BoundRoute, ref structs.ResourceReference) (bool, error) {
	if !g.shouldBindRouteToListener(bound, ref) {
		return false, nil
	}

	// check to make sure we're not binding to an invalid gateway
	if !g.Gateway.Status.MatchesConditionStatus(gatewayAccepted()) {
		return false, fmt.Errorf("failed to bind route to gateway %s: gateway has not been accepted", g.Gateway.Name)
	}

	// check to make sure we're not binding to an invalid route
	status := route.GetStatus()
	if !status.MatchesConditionStatus(routeAccepted()) {
		return false, fmt.Errorf("failed to bind route to gateway %s: route has not been accepted", g.Gateway.Name)
	}

	if route, ok := route.(*structs.HTTPRouteConfigEntry); ok {
		// check our hostnames
		hostnames := route.FilteredHostnames(listener.GetHostname())
		if len(hostnames) == 0 {
			return false, fmt.Errorf("failed to bind route to gateway %s: listener %s is does not have any hostnames that match the route", g.Gateway.Name, listener.Name)
		}
	}

	if listener.Protocol == route.GetProtocol() && bound.BindRoute(structs.ResourceReference{
		Kind:           route.GetKind(),
		Name:           route.GetName(),
		EnterpriseMeta: *route.GetEnterpriseMeta(),
	}) {
		return true, nil
	}

	if ref.SectionName != "" {
		return false, fmt.Errorf("failed to bind route %s to gateway %s: listener %s is not a %s listener", route.GetName(), g.Gateway.Name, bound.Name, route.GetProtocol())
	}

	return false, nil
}

// unbindRoute takes a route and unbinds it from all of the listeners on a gateway.
// It returns true if the route was unbound and false if it was not.
func (g *gatewayMeta) unbindRoute(route structs.ResourceReference) bool {
	didUnbind := false
	for _, listener := range g.boundListeners {
		if listener.UnbindRoute(route) {
			didUnbind = true
		}
	}

	return didUnbind
}

// eachListener iterates over all of the listeners for our underlying Gateway, it takes
// a callback function that can return an error, if an error is returned it halts execution
// and immediately returns the error.
func (g *gatewayMeta) eachListener(fn func(listener *structs.APIGatewayListener, bound *structs.BoundAPIGatewayListener) error) error {
	for name, listener := range g.listeners {
		if err := fn(listener, g.boundListeners[name]); err != nil {
			return err
		}
	}
	return nil
}

// checkCertificates verifies that all certificates referenced by the listeners on the gateway
// exist and collects them onto the bound gateway
func (g *gatewayMeta) checkCertificates(store *state.Store) (map[structs.ResourceReference]error, error) {
	certificateErrors := map[structs.ResourceReference]error{}

	err := g.eachListener(func(listener *structs.APIGatewayListener, bound *structs.BoundAPIGatewayListener) error {
		for _, ref := range listener.TLS.Certificates {
			_, certificate, err := store.ConfigEntry(nil, ref.Kind, ref.Name, &ref.EnterpriseMeta)
			if err != nil {
				return err
			}
			listenerRef := structs.ResourceReference{
				Kind:           structs.APIGateway,
				Name:           g.BoundGateway.Name,
				SectionName:    bound.Name,
				EnterpriseMeta: g.BoundGateway.EnterpriseMeta,
			}
			if certificate == nil {
				certificateErrors[listenerRef] = fmt.Errorf("certificate %q not found", ref.Name)
			} else {
				bound.Certificates = append(bound.Certificates, ref)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return certificateErrors, nil
}

// checkConflicts returns whether a gateway status needs to be updated with
// conflicting route statuses
func (g *gatewayMeta) checkConflicts() (structs.ControlledConfigEntry, bool) {
	updater := structs.NewStatusUpdater(g.Gateway)
	g.setConflicts(updater)
	return updater.UpdateEntry()
}

// setConflicts ensures that no TCP listener has more than the one allowed route and
// assigns an appropriate status
func (g *gatewayMeta) setConflicts(updater *structs.StatusUpdater) {
	g.eachListener(func(listener *structs.APIGatewayListener, bound *structs.BoundAPIGatewayListener) error {
		ref := structs.ResourceReference{
			Kind:           structs.APIGateway,
			Name:           g.Gateway.Name,
			SectionName:    listener.Name,
			EnterpriseMeta: g.Gateway.EnterpriseMeta,
		}
		switch listener.Protocol {
		case structs.ListenerProtocolTCP:
			if len(bound.Routes) > 1 {
				updater.SetCondition(gatewayListenerConflicts(ref))
				return nil
			}
		}
		updater.SetCondition(gatewayListenerNoConflicts(ref))
		return nil
	})
}

// initialize sets up the listener maps that we use for quickly indexing the listeners in our binding logic
func (g *gatewayMeta) initialize() *gatewayMeta {
	// set up the maps for fast access
	g.boundListeners = make(map[string]*structs.BoundAPIGatewayListener, len(g.BoundGateway.Listeners))
	for i, listener := range g.BoundGateway.Listeners {
		g.boundListeners[listener.Name] = &g.BoundGateway.Listeners[i]
	}
	g.listeners = make(map[string]*structs.APIGatewayListener, len(g.Gateway.Listeners))
	for i, listener := range g.Gateway.Listeners {
		g.listeners[listener.Name] = &g.Gateway.Listeners[i]
	}
	return g
}

// newGatewayMeta returns an object that wraps the given APIGateway and BoundAPIGateway
func newGatewayMeta(gateway *structs.APIGatewayConfigEntry, bound structs.ConfigEntry, jwtProviders map[string]*structs.JWTProviderConfigEntry) *gatewayMeta {
	var b *structs.BoundAPIGatewayConfigEntry
	if bound == nil {
		b = &structs.BoundAPIGatewayConfigEntry{
			Kind:           structs.BoundAPIGateway,
			Name:           gateway.Name,
			EnterpriseMeta: gateway.EnterpriseMeta,
		}
	} else {
		b = bound.(*structs.BoundAPIGatewayConfigEntry).DeepCopy()
	}

	// we just clear out the bound state here since we recalculate it entirely
	// in the gateway control loop
	listeners := make([]structs.BoundAPIGatewayListener, 0, len(gateway.Listeners))
	for _, listener := range gateway.Listeners {
		listeners = append(listeners, structs.BoundAPIGatewayListener{
			Name: listener.Name,
		})
	}

	b.Listeners = listeners

	return (&gatewayMeta{
		BoundGateway: b,
		Gateway:      gateway,
		jwtProviders: jwtProviders,
	}).initialize()
}

// gatewayAccepted marks the APIGateway as valid.
func gatewayAccepted() structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionAccepted,
		api.ConditionStatusTrue,
		api.GatewayReasonAccepted,
		"gateway is valid",
		structs.ResourceReference{},
	)
}

// invalidCertificate returns a condition used when a gateway references a
// certificate that does not exist. It takes a ref used to scope the condition
// to a given APIGateway listener.
func resolvedRefs(ref structs.ResourceReference) structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionResolvedRefs,
		api.ConditionStatusTrue,
		api.GatewayReasonResolvedRefs,
		"resolved refs",
		ref,
	)
}

// invalidCertificate returns a condition used when a gateway references a
// certificate that does not exist. It takes a ref used to scope the condition
// to a given APIGateway listener.
func invalidCertificate(ref structs.ResourceReference, err error) structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionResolvedRefs,
		api.ConditionStatusFalse,
		api.GatewayListenerReasonInvalidCertificateRef,
		err.Error(),
		ref,
	)
}

// invalidCertificates is used to set the overall condition of the APIGateway
// to invalid due to missing certificates that it references.
func invalidCertificates() structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionAccepted,
		api.ConditionStatusFalse,
		api.GatewayReasonInvalidCertificates,
		"gateway references invalid certificates",
		structs.ResourceReference{},
	)
}

// invalidJWTProvider returns a condition used when a gateway listener references
// a JWTProvider that does not exist. It takes a ref used to scope the condition
// to a given APIGateway listener.
func invalidJWTProvider(ref structs.ResourceReference, err error) structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionResolvedRefs,
		api.ConditionStatusFalse,
		api.GatewayListenerReasonInvalidJWTProviderRef,
		err.Error(),
		ref,
	)
}

// invalidJWTProviders is used to set the overall condition of the APIGateway
// to invalid due to missing JWT providers that it references.
func invalidJWTProviders() structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionAccepted,
		api.ConditionStatusFalse,
		api.GatewayReasonInvalidJWTProviders,
		"gateway references invalid JWT Providers",
		structs.ResourceReference{},
	)
}

// gatewayListenerNoConflicts marks an APIGateway listener as having no conflicts within its
// bound routes
func gatewayListenerNoConflicts(ref structs.ResourceReference) structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionConflicted,
		api.ConditionStatusFalse,
		api.GatewayReasonNoConflict,
		"listener has no route conflicts",
		ref,
	)
}

// gatewayListenerConflicts marks an APIGateway listener as having bound routes that conflict with each other
// and make the listener, therefore invalid
func gatewayListenerConflicts(ref structs.ResourceReference) structs.Condition {
	return structs.NewGatewayCondition(
		api.GatewayConditionConflicted,
		api.ConditionStatusTrue,
		api.GatewayReasonRouteConflict,
		"TCP-based listeners currently only support binding a single route",
		ref,
	)
}

// routeBound marks a Route as bound to the referenced APIGateway
func routeBound(ref structs.ResourceReference) structs.Condition {
	return structs.NewRouteCondition(
		api.RouteConditionBound,
		api.ConditionStatusTrue,
		api.RouteReasonBound,
		"successfully bound route",
		ref,
	)
}

// gatewayNotFound marks a Route as having failed to bind to a referenced APIGateway due to
// the Gateway not existing (or having not been reconciled yet)
func gatewayNotFound(ref structs.ResourceReference) structs.Condition {
	return structs.NewRouteCondition(
		api.RouteConditionBound,
		api.ConditionStatusFalse,
		api.RouteReasonGatewayNotFound,
		"gateway was not found",
		ref,
	)
}

// routeUnbound marks the route as having failed to bind to the referenced APIGateway
func routeUnbound(ref structs.ResourceReference, err error) structs.Condition {
	return structs.NewRouteCondition(
		api.RouteConditionBound,
		api.ConditionStatusFalse,
		api.RouteReasonFailedToBind,
		err.Error(),
		ref,
	)
}

// routeAccepted marks the Route as valid
func routeAccepted() structs.Condition {
	return structs.NewRouteCondition(
		api.RouteConditionAccepted,
		api.ConditionStatusTrue,
		api.RouteReasonAccepted,
		"route is valid",
		structs.ResourceReference{},
	)
}

// routeInvalidDiscoveryChain marks the route as invalid due to an error while validating its referenced
// discovery chian
func routeInvalidDiscoveryChain(err error) structs.Condition {
	return structs.NewRouteCondition(
		api.RouteConditionAccepted,
		api.ConditionStatusFalse,
		api.RouteReasonInvalidDiscoveryChain,
		err.Error(),
		structs.ResourceReference{},
	)
}

// routeNoUpstreams marks the route as invalid because it has no upstreams that it targets
func routeNoUpstreams() structs.Condition {
	return structs.NewRouteCondition(
		api.RouteConditionAccepted,
		api.ConditionStatusFalse,
		api.RouteReasonNoUpstreamServicesTargeted,
		"route must target at least one upstream service",
		structs.ResourceReference{},
	)
}

// bindRoutesToGateways takes a route variadic number of gateways.
// It iterates over the parent references for the route. These parents are gateways the
// route should be bound to. If the parent matches a bound gateway, the route is bound to the
// gateway. Otherwise, the route is unbound from the gateway if it was previously bound.
//
// The function returns a list of references to the modified BoundAPIGatewayConfigEntry objects,
// a list of parent references on the route that were successfully used to bind the route, and
// a map of resource references to errors that occurred when they were attempted to be
// bound to a gateway.
func bindRoutesToGateways(route structs.BoundRoute, gateways ...*gatewayMeta) ([]*structs.BoundAPIGatewayConfigEntry, []structs.ResourceReference, map[structs.ResourceReference]error) {
	boundRefs := []structs.ResourceReference{}
	modified := make([]*structs.BoundAPIGatewayConfigEntry, 0, len(gateways))

	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	// Iterate over all BoundAPIGateway config entries and try to bind them to the route if they are a parent.
	for _, gateway := range gateways {
		didUpdate, bound, errors := gateway.updateRouteBinding(route)

		if didUpdate {
			modified = append(modified, gateway.BoundGateway)
		}

		for ref, err := range errors {
			errored[ref] = err
		}

		boundRefs = append(boundRefs, bound...)
	}

	return modified, boundRefs, errored
}

// removeGateway sets the route's status appropriately when the gateway that it's
// attempting to bind to does not exist
func removeGateway(gateway structs.ResourceReference, entries ...structs.BoundRoute) []structs.ControlledConfigEntry {
	modified := []structs.ControlledConfigEntry{}

	for _, route := range entries {
		updater := structs.NewStatusUpdater(route)

		for _, parent := range route.GetParents() {
			if parent.Kind == gateway.Kind && parent.Name == gateway.Name && parent.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
				updater.SetCondition(gatewayNotFound(parent))
			}
		}

		if toUpdate, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
			modified = append(modified, toUpdate)
		}
	}

	return modified
}

// removeRoute unbinds the route from the given gateways, returning the list of gateways that were modified.
func removeRoute(route structs.ResourceReference, entries ...*gatewayMeta) []*gatewayMeta {
	modified := []*gatewayMeta{}

	for _, entry := range entries {
		if entry.unbindRoute(route) {
			modified = append(modified, entry)
			entry.BoundGateway.Services.RemoveRouteRef(route)
		}
	}

	return modified
}

// requestToResourceRef constructs a resource reference from the given controller request
func requestToResourceRef(req controller.Request) structs.ResourceReference {
	ref := structs.ResourceReference{
		Kind: req.Kind,
		Name: req.Name,
	}

	if req.Meta != nil {
		ref.EnterpriseMeta = *req.Meta
	}

	return ref
}

// retrieveAllRoutesFromStore retrieves all HTTP and TCP routes from the given store
func retrieveAllRoutesFromStore(store *state.Store) ([]structs.BoundRoute, error) {
	_, httpRoutes, err := store.ConfigEntriesByKind(nil, structs.HTTPRoute, wildcardMeta())
	if err != nil {
		return nil, err
	}

	_, tcpRoutes, err := store.ConfigEntriesByKind(nil, structs.TCPRoute, wildcardMeta())
	if err != nil {
		return nil, err
	}

	routes := make([]structs.BoundRoute, 0, len(tcpRoutes)+len(httpRoutes))

	for _, route := range httpRoutes {
		routes = append(routes, route.(*structs.HTTPRouteConfigEntry))
	}

	for _, route := range tcpRoutes {
		routes = append(routes, route.(*structs.TCPRouteConfigEntry))
	}

	return routes, nil
}

// pointerTo returns a pointer to the value passed as an argument
func pointerTo[T any](value T) *T {
	return &value
}

// requestLogger returns a logger that adds some request-specific fields to the given logger
func requestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("kind", request.Kind, "name", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

// certificateRequestLogger returns a logger that adds some certificate-specific fields to the given logger
func certificateRequestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("inline-certificate", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

// gatewayRequestLogger returns a logger that adds some gateway-specific fields to the given logger
func gatewayRequestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("gateway", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

// gatewayLogger returns a logger that adds some gateway-specific fields to the given logger,
// it should be used when logging info about a gateway resource being modified from a non-gateway
// reconciliation funciton
func gatewayLogger(logger hclog.Logger, gateway structs.ConfigEntry) hclog.Logger {
	meta := gateway.GetEnterpriseMeta()
	return logger.With("gateway.name", gateway.GetName(), "gateway.namespace", meta.NamespaceOrDefault(), "gateway.partition", meta.PartitionOrDefault())
}

// routeRequestLogger returns a logger that adds some route-specific fields to the given logger
func routeRequestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("kind", request.Kind, "route", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

// routeLogger returns a logger that adds some route-specific fields to the given logger,
// it should be used when logging info about a route resource being modified from a non-route
// reconciliation funciton
func routeLogger(logger hclog.Logger, route structs.ConfigEntry) hclog.Logger {
	meta := route.GetEnterpriseMeta()
	return logger.With("route.kind", route.GetKind(), "route.name", route.GetName(), "route.namespace", meta.NamespaceOrDefault(), "route.partition", meta.PartitionOrDefault())
}

func wildcardMeta() *acl.EnterpriseMeta {
	meta := acl.WildcardEnterpriseMeta()
	meta.OverridePartition(acl.WildcardPartitionName)
	return meta
}
