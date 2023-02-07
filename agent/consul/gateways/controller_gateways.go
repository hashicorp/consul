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

func (r apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	// We do this in a single threaded way to avoid race conditions around setting
	// shared state. In our current out-of-repo code, this is handled via a global
	// lock on our shared store, but this makes it so we don't have to deal with lock
	// contention, and instead just work with a single control loop.
	switch req.Kind {
	case structs.APIGateway:
		return reconcileEntry(r.fsm.State(), ctx, req, r.reconcileGateway, r.cleanupGateway)
	case structs.BoundAPIGateway:
		return reconcileEntry(r.fsm.State(), ctx, req, r.reconcileBoundGateway, r.cleanupBoundGateway)
	case structs.HTTPRoute:
		return reconcileEntry(r.fsm.State(), ctx, req, func(ctx context.Context, req controller.Request, store *state.Store, route *structs.HTTPRouteConfigEntry) error {
			return r.reconcileRoute(ctx, req, store, route)
		}, r.cleanupRoute)
	case structs.TCPRoute:
		return reconcileEntry(r.fsm.State(), ctx, req, func(ctx context.Context, req controller.Request, store *state.Store, route *structs.TCPRouteConfigEntry) error {
			return r.reconcileRoute(ctx, req, store, route)
		}, r.cleanupRoute)
	case structs.InlineCertificate:
		return r.enqueueCertificateReferencedGateways(r.fsm.State(), ctx, req)
	default:
		return nil
	}
}

func reconcileEntry[T structs.ControlledConfigEntry](store *state.Store, ctx context.Context, req controller.Request, reconciler func(ctx context.Context, req controller.Request, store *state.Store, entry T) error, cleaner func(ctx context.Context, req controller.Request, store *state.Store) error) error {
	_, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, req.Meta)
	if err != nil {
		return err
	}
	if entry == nil {
		return cleaner(ctx, req, store)
	}
	return reconciler(ctx, req, store, entry.(T))
}

func (r apiGatewayReconciler) enqueueCertificateReferencedGateways(store *state.Store, _ context.Context, req controller.Request) error {
	logger := r.logger.With("inline-certificate", req.Name, "partition", req.Meta.PartitionOrDefault(), "namespace", req.Meta.NamespaceOrDefault())
	logger.Debug("certificate changed, enqueueing dependent gateways")
	defer logger.Debug("finished enqueuing gateways")

	_, entries, err := store.ConfigEntriesByKind(nil, structs.APIGateway, acl.WildcardEnterpriseMeta())
	if err != nil {
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

func (r apiGatewayReconciler) cleanupBoundGateway(_ context.Context, req controller.Request, store *state.Store) error {
	logger := r.logger.With("bound-gateway", req.Name, "partition", req.Meta.PartitionOrDefault(), "namespace", req.Meta.NamespaceOrDefault())
	logger.Debug("cleaning up bound gateway")
	defer logger.Debug("finished cleaning up bound gateway")

	routes, err := retrieveAllRoutesFromStore(store)
	if err != nil {
		return err
	}

	resource := requestToResourceRef(req)
	resource.Kind = structs.APIGateway

	for _, toUpdate := range RemoveGateway(resource, routes...) {
		if err := r.updater.Update(toUpdate); err != nil {
			return err
		}
	}
	return nil
}

func (r apiGatewayReconciler) reconcileBoundGateway(_ context.Context, req controller.Request, store *state.Store, bound *structs.BoundAPIGatewayConfigEntry) error {
	// this reconciler handles orphaned bound gateways at startup, it just checks to make sure there's still an existing gateway, and if not, it deletes the bound gateway
	logger := r.logger.With("bound-gateway", req.Name, "partition", req.Meta.PartitionOrDefault(), "namespace", req.Meta.NamespaceOrDefault())
	logger.Debug("reconciling bound gateway")
	defer logger.Debug("finished reconciling bound gateway")

	_, gateway, err := store.ConfigEntry(nil, structs.APIGateway, req.Name, req.Meta)
	if err != nil {
		return err
	}
	if gateway == nil {
		// delete the bound gateway
		return r.updater.Delete(bound)
	}
	return nil
}

func (r apiGatewayReconciler) cleanupGateway(_ context.Context, req controller.Request, store *state.Store) error {
	logger := r.logger.With("gateway", req.Name, "partition", req.Meta.PartitionOrDefault(), "namespace", req.Meta.NamespaceOrDefault())
	logger.Debug("cleaning up deleted gateway")
	defer logger.Debug("finished cleaning up deleted gateway")

	_, bound, err := store.ConfigEntry(nil, structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		return err
	}
	return r.updater.Delete(bound)
}

func (r apiGatewayReconciler) reconcileGateway(_ context.Context, req controller.Request, store *state.Store, gateway *structs.APIGatewayConfigEntry) error {
	now := time.Now().UTC()

	logger := r.logger.With("gateway", req.Name, "partition", req.Meta.PartitionOrDefault(), "namespace", req.Meta.NamespaceOrDefault())
	logger.Debug("started reconciling gateway")
	defer logger.Debug("finished reconciling gateway")

	updater := structs.NewStatusUpdater(gateway)
	// we clear out the initial status conditions since we're doing a full update
	// of this gateway's status
	updater.ClearConditions()

	routes, err := retrieveAllRoutesFromStore(store)
	if err != nil {
		return err
	}

	// construct the tuple we'll be working on to update state
	_, bound, err := store.ConfigEntry(nil, structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		return err
	}
	meta := ensureInitializedMeta(gateway, bound)
	certificateErrors, err := meta.checkCertificates(store)
	if err != nil {
		return err
	}

	for ref, err := range certificateErrors {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "False",
			Reason:             "InvalidCertificate",
			Message:            err.Error(),
			Resource:           &ref,
			LastTransitionTime: &now,
		})
	}
	if len(certificateErrors) > 0 {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "False",
			Reason:             "InvalidCertificates",
			Message:            "gateway references invalid certificates",
			LastTransitionTime: &now,
		})
	} else {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "True",
			Reason:             "Accepted",
			Message:            "gateway is valid",
			LastTransitionTime: &now,
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
					Resource: &parent,
				})
			}
		}
		for _, ref := range boundRefs {
			routeUpdater.SetCondition(structs.Condition{
				Type:               "Bound",
				Status:             "True",
				Reason:             "Bound",
				Resource:           &ref,
				Message:            "successfully bound route",
				LastTransitionTime: &now,
			})
		}
		for reference, err := range bindErrors {
			routeUpdater.SetCondition(structs.Condition{
				Type:               "Bound",
				Status:             "False",
				Reason:             "FailedToBind",
				Resource:           &reference,
				Message:            err.Error(),
				LastTransitionTime: &now,
			})
		}
		if entry, updated := routeUpdater.UpdateEntry(); updated {
			updatedRoutes = append(updatedRoutes, entry)
		}
	}

	// first check for gateway conflicts
	for i, listener := range meta.BoundGateway.Listeners {
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
					LastTransitionTime: &now,
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
			LastTransitionTime: &now,
		})
	}

	// now check if we need to update the gateway status
	if toUpdate, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
		r.logger.Debug("persisting gateway status", "gateway", gateway)
		if err := r.updater.UpdateWithStatus(toUpdate); err != nil {
			r.logger.Error("error persisting gateway status", "error", err)
			return err
		}
	}

	// next update route statuses
	for _, toUpdate := range updatedRoutes {
		r.logger.Debug("persisting route status", "route", toUpdate)
		if err := r.updater.UpdateWithStatus(toUpdate); err != nil {
			r.logger.Error("error persisting route status", "error", err)
			return err
		}
	}

	// now update the bound state if it changed
	if bound == nil || stateIsDirty(bound.(*structs.BoundAPIGatewayConfigEntry), meta.BoundGateway) {
		r.logger.Debug("persisting gateway state", "state", meta.BoundGateway)
		if err := r.updater.Update(meta.BoundGateway); err != nil {
			r.logger.Error("error persisting state", "error", err)
			return err
		}
	}

	return nil
}

func (r apiGatewayReconciler) cleanupRoute(_ context.Context, req controller.Request, store *state.Store) error {
	meta, err := getAllGatewayMeta(store)
	if err != nil {
		return err
	}

	r.logger.Debug("cleaning up deleted route object", "request", req)
	for _, toUpdate := range RemoveRoute(requestToResourceRef(req), meta...) {
		if err := r.updater.Update(toUpdate.BoundGateway); err != nil {
			return err
		}
	}
	r.controller.RemoveTrigger(req)
	return nil
}

// Reconcile reconciles Route config entries.
func (r apiGatewayReconciler) reconcileRoute(_ context.Context, req controller.Request, store *state.Store, route structs.BoundRoute) error {
	now := time.Now().UTC()

	meta, err := getAllGatewayMeta(store)
	if err != nil {
		return err
	}

	r.logger.Debug("got route reconcile call", "request", req)

	updater := structs.NewStatusUpdater(route)
	// we clear out the initial status conditions since we're doing a full update
	// of this route's status
	updater.ClearConditions()

	ws := memdb.NewWatchSet()
	ws.Add(store.AbandonCh())

	finalize := func(modifiedGateways []*structs.BoundAPIGatewayConfigEntry) error {
		// first update any gateway statuses that are now in conflict
		for _, gateway := range meta {
			toUpdate, shouldUpdate := gateway.checkConflicts()
			if shouldUpdate {
				r.logger.Debug("persisting gateway status", "gateway", gateway)
				if err := r.updater.UpdateWithStatus(toUpdate); err != nil {
					r.logger.Error("error persisting gateway", "error", gateway)
					return err
				}
			}
		}

		// next update the route status
		if toUpdate, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
			r.logger.Debug("persisting route status", "route", route)
			if err := r.updater.UpdateWithStatus(toUpdate); err != nil {
				r.logger.Error("error persisting route", "error", route)
				return err
			}
		}

		// now update all of the state values
		for _, state := range modifiedGateways {
			r.logger.Debug("persisting gateway state", "state", state)
			if err := r.updater.Update(state); err != nil {
				r.logger.Error("error persisting state", "error", err)
				return err
			}
		}

		return nil
	}

	var triggerOnce sync.Once
	validTargets := true
	for _, service := range route.GetTargetedServices() {
		_, chainSet, err := store.ReadDiscoveryChainConfigEntries(ws, service.Name, &service.EnterpriseMeta)
		if err != nil {
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
				LastTransitionTime: &now,
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
					LastTransitionTime: &now,
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
					LastTransitionTime: &now,
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
				LastTransitionTime: &now,
			})
		}
	}
	if len(route.GetTargetedServices()) == 0 {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "False",
			Reason:             "NoUpstreamServicesTargeted",
			Message:            "route must target at least one upstream service",
			LastTransitionTime: &now,
		})
		validTargets = false
	}
	if !validTargets {
		// we return early, but need to make sure we're removed from all referencing
		// gateways and our status is updated properly
		updated := []*structs.BoundAPIGatewayConfigEntry{}
		for _, toUpdate := range RemoveRoute(requestToResourceRef(req), meta...) {
			updated = append(updated, toUpdate.BoundGateway)
		}
		return finalize(updated)
	}

	r.logger.Debug("binding routes to gateway")

	// the route is valid, attempt to bind it to all gateways
	modifiedGateways, boundRefs, bindErrors := BindRoutesToGateways(meta, route)
	if err != nil {
		return err
	}

	// set the status of the references that are bound
	for _, ref := range boundRefs {
		updater.SetCondition(structs.Condition{
			Type:               "Bound",
			Status:             "True",
			Reason:             "Bound",
			Resource:           &ref,
			Message:            "successfully bound route",
			LastTransitionTime: &now,
		})
	}

	// set any binding errors
	for reference, err := range bindErrors {
		updater.SetCondition(structs.Condition{
			Type:               "Bound",
			Status:             "False",
			Reason:             "FailedToBind",
			Resource:           &reference,
			Message:            err.Error(),
			LastTransitionTime: &now,
		})
	}

	return finalize(modifiedGateways)
}

func NewAPIGatewayController(fsm *fsm.FSM, publisher state.EventPublisher, updater *Updater, logger hclog.Logger) controller.Controller {
	reconciler := apiGatewayReconciler{
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
