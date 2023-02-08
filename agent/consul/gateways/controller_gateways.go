package gateways

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
)

type Updater struct {
	UpdateWithStatus func(entry structs.ControlledConfigEntry) error
	Update           func(entry structs.ConfigEntry) error
	Delete           func(entry structs.ConfigEntry) error
}

type apiGatewayReconciler struct {
	fsm        *fsm.FSM
	logger     hclog.Logger
	updater    *Updater
	controller controller.Controller
}

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
	case structs.InlineCertificate:
		return r.enqueueCertificateReferencedGateways(r.fsm.State(), ctx, req)
	default:
		return nil
	}
}

func reconcileEntry[T structs.ControlledConfigEntry](store *state.Store, logger hclog.Logger, ctx context.Context, req controller.Request, reconciler func(ctx context.Context, req controller.Request, store *state.Store, entry T) error, cleaner func(ctx context.Context, req controller.Request, store *state.Store) error) error {
	_, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, req.Meta)
	if err != nil {
		requestLogger(logger, req).Error("error fetching config entry for reconciliation request", "error", err)
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

	logger.Debug("certificate changed, enqueueing dependent gateways")
	defer logger.Debug("finished enqueuing gateways")

	_, entries, err := store.ConfigEntriesByKind(nil, structs.APIGateway, acl.WildcardEnterpriseMeta())
	if err != nil {
		logger.Error("error retrieving api gateways", "error", err)
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

	logger.Debug("cleaning up bound gateway")
	defer logger.Debug("finished cleaning up bound gateway")

	routes, err := retrieveAllRoutesFromStore(store)
	if err != nil {
		logger.Error("error retrieving routes", "error", err)
		return err
	}

	resource := requestToResourceRef(req)
	resource.Kind = structs.APIGateway

	for _, modifiedRoute := range RemoveGateway(resource, routes...) {
		routeLogger := routeLogger(logger, modifiedRoute)
		routeLogger.Debug("persisting route status")
		if err := r.updater.Update(modifiedRoute); err != nil {
			routeLogger.Error("error removing gateway from route", "error", err)
			return err
		}
	}

	return nil
}

// reconcileBoundGateway mainly handles orphaned bound gateways at startup, it just checks
// to make sure there's still an existing gateway, and if not, it deletes the bound gateway
func (r *apiGatewayReconciler) reconcileBoundGateway(_ context.Context, req controller.Request, store *state.Store, bound *structs.BoundAPIGatewayConfigEntry) error {
	logger := gatewayRequestLogger(r.logger, req)

	logger.Debug("reconciling bound gateway")
	defer logger.Debug("finished reconciling bound gateway")

	_, gateway, err := store.ConfigEntry(nil, structs.APIGateway, req.Name, req.Meta)
	if err != nil {
		logger.Error("error retrieving api gateway", "error", err)
		return err
	}

	if gateway == nil {
		// delete the bound gateway
		logger.Debug("deleting bound api gateway")
		if err := r.updater.Delete(bound); err != nil {
			logger.Error("error deleting bound api gateway", "error", err)
			return err
		}
	}

	return nil
}

func (r *apiGatewayReconciler) cleanupGateway(_ context.Context, req controller.Request, store *state.Store) error {
	logger := gatewayRequestLogger(r.logger, req)

	logger.Debug("cleaning up deleted gateway")
	defer logger.Debug("finished cleaning up deleted gateway")

	_, bound, err := store.ConfigEntry(nil, structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		logger.Error("error retrieving bound api gateway", "error", err)
		return err
	}

	logger.Debug("deleting bound api gateway")
	if err := r.updater.Delete(bound); err != nil {
		logger.Error("error deleting bound api gateway", "error", err)
		return err
	}

	return nil
}

func (r *apiGatewayReconciler) reconcileGateway(_ context.Context, req controller.Request, store *state.Store, gateway *structs.APIGatewayConfigEntry) error {
	now := pointerTo(time.Now().UTC())

	logger := gatewayRequestLogger(r.logger, req)

	logger.Debug("started reconciling gateway")
	defer logger.Debug("finished reconciling gateway")

	updater := structs.NewStatusUpdater(gateway)
	// we clear out the initial status conditions since we're doing a full update
	// of this gateway's status
	updater.ClearConditions()

	routes, err := retrieveAllRoutesFromStore(store)
	if err != nil {
		logger.Error("error retrieving routes", "error", err)
		return err
	}

	// construct the tuple we'll be working on to update state
	_, bound, err := store.ConfigEntry(nil, structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		logger.Error("error retrieving bound api gateway", "error", err)
		return err
	}

	meta := ensureInitializedMeta(gateway, bound)

	certificateErrors, err := meta.checkCertificates(store)
	if err != nil {
		logger.Error("error checking gateway certificates", "error", err)
		return err
	}

	for ref, err := range certificateErrors {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "False",
			Reason:             "InvalidCertificate",
			Message:            err.Error(),
			Resource:           pointerTo(ref),
			LastTransitionTime: now,
		})
	}

	if len(certificateErrors) > 0 {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "False",
			Reason:             "InvalidCertificates",
			Message:            "gateway references invalid certificates",
			LastTransitionTime: now,
		})
	} else {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "True",
			Reason:             "Accepted",
			Message:            "gateway is valid",
			LastTransitionTime: now,
		})
	}

	// now we bind all of the routes we can
	updatedRoutes := []structs.ControlledConfigEntry{}
	for _, route := range routes {
		routeUpdater := structs.NewStatusUpdater(route)
		_, boundRefs, bindErrors := BindRoutesToGateways([]*gatewayMeta{meta}, route)

		// unset the old gateway binding in case it's stale
		for _, parent := range route.GetParents() {
			if parent.Kind == gateway.Kind && parent.Name == gateway.Name && parent.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
				routeUpdater.RemoveCondition(structs.Condition{
					Type:     "Bound",
					Resource: pointerTo(parent),
				})
			}
		}

		// set the status for parents that have bound successfully
		for _, ref := range boundRefs {
			routeUpdater.SetCondition(structs.Condition{
				Type:               "Bound",
				Status:             "True",
				Reason:             "Bound",
				Resource:           pointerTo(ref),
				Message:            "successfully bound route",
				LastTransitionTime: now,
			})
		}

		// set the status for any parents that have errored trying to
		// bind
		for ref, err := range bindErrors {
			routeUpdater.SetCondition(structs.Condition{
				Type:               "Bound",
				Status:             "False",
				Reason:             "FailedToBind",
				Resource:           pointerTo(ref),
				Message:            err.Error(),
				LastTransitionTime: now,
			})
		}

		// if we've updated any statuses, then store them as needing
		// to be updated
		if entry, updated := routeUpdater.UpdateEntry(); updated {
			updatedRoutes = append(updatedRoutes, entry)
		}
	}

	// first check for gateway conflicts
	for i, listener := range meta.BoundGateway.Listeners {
		// TODO: refactor this to leverage something like checkConflicts
		// that will require the ability to do something like pass in
		// an updater since it's currently scoped to the function itself
		protocol := meta.Gateway.Listeners[i].Protocol

		switch protocol {
		case structs.ListenerProtocolTCP:
			if len(listener.Routes) > 1 {
				updater.SetCondition(structs.Condition{
					Type:    "Conflicted",
					Status:  "True",
					Reason:  "RouteConflict",
					Message: "TCP-based listeners currently only support binding a single route",
					Resource: &structs.ResourceReference{
						Kind:           structs.APIGateway,
						Name:           meta.Gateway.Name,
						SectionName:    listener.Name,
						EnterpriseMeta: meta.Gateway.EnterpriseMeta,
					},
					LastTransitionTime: now,
				})
				continue
			}
		}

		updater.SetCondition(structs.Condition{
			Type:   "Conflicted",
			Status: "False",
			Reason: "NoConflict",
			Resource: &structs.ResourceReference{
				Kind:           structs.APIGateway,
				Name:           meta.Gateway.Name,
				SectionName:    listener.Name,
				EnterpriseMeta: meta.Gateway.EnterpriseMeta,
			},
			Message:            "listener has no route conflicts",
			LastTransitionTime: now,
		})
	}

	// now check if we need to update the gateway status
	if modifiedGateway, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
		logger.Debug("persisting gateway status")
		if err := r.updater.UpdateWithStatus(modifiedGateway); err != nil {
			logger.Error("error persisting gateway status", "error", err)
			return err
		}
	}

	// next update route statuses
	for _, modifiedRoute := range updatedRoutes {
		routeLogger := routeLogger(logger, modifiedRoute)
		routeLogger.Debug("persisting route status")
		if err := r.updater.UpdateWithStatus(modifiedRoute); err != nil {
			routeLogger.Error("error persisting route status", "error", err)
			return err
		}
	}

	// now update the bound state if it changed
	if bound == nil || stateIsDirty(bound.(*structs.BoundAPIGatewayConfigEntry), meta.BoundGateway) {
		logger.Debug("persisting bound api gateway")
		if err := r.updater.Update(meta.BoundGateway); err != nil {
			logger.Error("error persisting bound api gateway", "error", err)
			return err
		}
	}

	return nil
}

func (r *apiGatewayReconciler) cleanupRoute(_ context.Context, req controller.Request, store *state.Store) error {
	logger := routeRequestLogger(r.logger, req)

	logger.Debug("cleaning up route")
	defer logger.Debug("finished cleaning up route")

	meta, err := getAllGatewayMeta(store)
	if err != nil {
		logger.Error("error retrieving gateways", "error", err)
		return err
	}

	for _, modifiedGateway := range RemoveRoute(requestToResourceRef(req), meta...) {
		gatewayLogger := gatewayLogger(logger, modifiedGateway.BoundGateway)
		gatewayLogger.Debug("persisting bound gateway state")
		if err := r.updater.Update(modifiedGateway.BoundGateway); err != nil {
			gatewayLogger.Error("error updating bound api gateway", "error", err)
			return err
		}
	}

	r.controller.RemoveTrigger(req)

	return nil
}

// Reconcile reconciles Route config entries.
func (r *apiGatewayReconciler) reconcileRoute(_ context.Context, req controller.Request, store *state.Store, route structs.BoundRoute) error {
	now := pointerTo(time.Now().UTC())

	logger := routeRequestLogger(r.logger, req)

	logger.Debug("reconciling route")
	defer logger.Debug("finished reconciling route")

	meta, err := getAllGatewayMeta(store)
	if err != nil {
		logger.Error("error retrieving gateways", "error", err)
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
				gatewayLogger.Debug("persisting gateway status")
				if err := r.updater.UpdateWithStatus(modifiedGateway); err != nil {
					gatewayLogger.Error("error persisting gateway", "error", err)
					return err
				}
			}
		}

		// next update the route status
		if modifiedRoute, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
			r.logger.Debug("persisting route status")
			if err := r.updater.UpdateWithStatus(modifiedRoute); err != nil {
				r.logger.Error("error persisting route", "error", err)
				return err
			}
		}

		// now update all of the bound gateways that have been modified
		for _, bound := range modifiedGateways {
			gatewayLogger := gatewayLogger(logger, bound)
			gatewayLogger.Debug("persisting bound api gateway")
			if err := r.updater.Update(bound); err != nil {
				gatewayLogger.Error("error persisting bound api gateway", "error", err)
				return err
			}
		}

		return nil
	}

	var triggerOnce sync.Once
	validTargets := true
	for _, service := range route.GetServiceNames() {
		_, chainSet, err := store.ReadDiscoveryChainConfigEntries(ws, service.Name, pointerTo(service.EnterpriseMeta))
		if err != nil {
			logger.Error("error reading discovery chain", "error", err)
			return err
		}

		// trigger a watch since we now need to check when the discovery chain gets updated
		triggerOnce.Do(func() {
			r.controller.AddTrigger(req, ws.WatchCtx)
		})

		if chainSet.IsEmpty() {
			updater.SetCondition(structs.Condition{
				Type:               "Accepted",
				Status:             "False",
				Reason:             "InvalidDiscoveryChain",
				Message:            "service does not exist",
				LastTransitionTime: now,
			})
			continue
		}

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
			// we only really need to return the first error for an invalid
			// discovery chain, but we still want to set watches on everything in the
			// store
			if validTargets {
				updater.SetCondition(structs.Condition{
					Type:               "Accepted",
					Status:             "False",
					Reason:             "InvalidDiscoveryChain",
					Message:            err.Error(),
					LastTransitionTime: now,
				})
				validTargets = false
			}
			continue
		}

		if chain.Protocol != string(route.GetProtocol()) {
			if validTargets {
				updater.SetCondition(structs.Condition{
					Type:               "Accepted",
					Status:             "False",
					Reason:             "InvalidDiscoveryChain",
					Message:            "route protocol does not match targeted service protocol",
					LastTransitionTime: now,
				})
				validTargets = false
			}
			continue
		}

		// this makes sure we don't override an already set status
		if validTargets {
			updater.SetCondition(structs.Condition{
				Type:               "Accepted",
				Status:             "True",
				Reason:             "Accepted",
				Message:            "route is valid",
				LastTransitionTime: now,
			})
		}
	}

	// if we have no upstream targets, then set the route as invalid
	// this should already happen in the validation check on write, but
	// we'll do it here too just in case
	if len(route.GetServiceNames()) == 0 {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "False",
			Reason:             "NoUpstreamServicesTargeted",
			Message:            "route must target at least one upstream service",
			LastTransitionTime: now,
		})
		validTargets = false
	}

	if !validTargets {
		// we return early, but need to make sure we're removed from all referencing
		// gateways and our status is updated properly
		updated := []*structs.BoundAPIGatewayConfigEntry{}
		for _, modifiedGateway := range RemoveRoute(requestToResourceRef(req), meta...) {
			updated = append(updated, modifiedGateway.BoundGateway)
		}
		return finalize(updated)
	}

	// the route is valid, attempt to bind it to all gateways
	r.logger.Debug("binding routes to gateway")
	modifiedGateways, boundRefs, bindErrors := BindRoutesToGateways(meta, route)

	// set the status of the references that are bound
	for _, ref := range boundRefs {
		updater.SetCondition(structs.Condition{
			Type:               "Bound",
			Status:             "True",
			Reason:             "Bound",
			Resource:           pointerTo(ref),
			Message:            "successfully bound route",
			LastTransitionTime: now,
		})
	}

	// set any binding errors
	for ref, err := range bindErrors {
		updater.SetCondition(structs.Condition{
			Type:               "Bound",
			Status:             "False",
			Reason:             "FailedToBind",
			Resource:           pointerTo(ref),
			Message:            err.Error(),
			LastTransitionTime: now,
		})
	}

	return finalize(modifiedGateways)
}

func (r *apiGatewayReconciler) reconcileHTTPRoute(ctx context.Context, req controller.Request, store *state.Store, route *structs.HTTPRouteConfigEntry) error {
	return r.reconcileRoute(ctx, req, store, route)
}

func (r *apiGatewayReconciler) reconcileTCPRoute(ctx context.Context, req controller.Request, store *state.Store, route *structs.TCPRouteConfigEntry) error {
	return r.reconcileRoute(ctx, req, store, route)
}

func NewAPIGatewayController(fsm *fsm.FSM, publisher state.EventPublisher, updater *Updater, logger hclog.Logger) controller.Controller {
	reconciler := &apiGatewayReconciler{
		fsm:     fsm,
		logger:  logger,
		updater: updater,
	}
	reconciler.controller = controller.New(publisher, reconciler)
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
		})
}

func retrieveAllRoutesFromStore(store *state.Store) ([]structs.BoundRoute, error) {
	_, httpRoutes, err := store.ConfigEntriesByKind(nil, structs.HTTPRoute, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, err
	}

	_, tcpRoutes, err := store.ConfigEntriesByKind(nil, structs.TCPRoute, acl.WildcardEnterpriseMeta())
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

func pointerTo[T any](value T) *T {
	return &value
}

func requestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("kind", request.Kind, "name", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

func certificateRequestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("inline-certificate", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

func gatewayRequestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("gateway", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

func gatewayLogger(logger hclog.Logger, gateway structs.ConfigEntry) hclog.Logger {
	meta := gateway.GetEnterpriseMeta()
	return logger.With("gateway.name", gateway.GetName(), "gateway.namespace", meta.NamespaceOrDefault(), "gateway.partition", meta.PartitionOrDefault())
}

func routeRequestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	meta := request.Meta
	return logger.With("kind", request.Kind, "route", request.Name, "namespace", meta.NamespaceOrDefault(), "partition", meta.PartitionOrDefault())
}

func routeLogger(logger hclog.Logger, route structs.ConfigEntry) hclog.Logger {
	meta := route.GetEnterpriseMeta()
	return logger.With("route.kind", route.GetKind(), "route.name", route.GetName(), "route.namespace", meta.NamespaceOrDefault(), "route.partition", meta.PartitionOrDefault())
}
