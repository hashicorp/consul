package gateways

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
)

var (
	errServiceDoesNotExist = errors.New("service does not exist")
	errInvalidProtocol     = errors.New("route protocol does not match targeted service protocol")
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
	conditions := newConditionGenerator()

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
		updater.SetCondition(conditions.invalidCertificate(ref, err))
	}

	if len(certificateErrors) > 0 {
		updater.SetCondition(conditions.invalidCertificates())
	} else {
		updater.SetCondition(conditions.gatewayAccepted())
	}

	// now we bind all of the routes we can
	updatedRoutes := []structs.ControlledConfigEntry{}
	for _, route := range routes {
		routeUpdater := structs.NewStatusUpdater(route)
		_, boundRefs, bindErrors := BindRoutesToGateways([]*gatewayMeta{meta}, route)

		// unset the old gateway binding in case it's stale
		for _, parent := range route.GetParents() {
			if parent.Kind == gateway.Kind && parent.Name == gateway.Name && parent.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
				routeUpdater.RemoveCondition(conditions.routeBound(parent))
			}
		}

		// set the status for parents that have bound successfully
		for _, ref := range boundRefs {
			routeUpdater.SetCondition(conditions.routeBound(ref))
		}

		// set the status for any parents that have errored trying to
		// bind
		for ref, err := range bindErrors {
			routeUpdater.SetCondition(conditions.routeUnbound(ref, err))
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
	if bound == nil || !bound.(*structs.BoundAPIGatewayConfigEntry).IsSame(meta.BoundGateway) {
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
	conditions := newConditionGenerator()

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
			updater.SetCondition(conditions.routeInvalidDiscoveryChain(errServiceDoesNotExist))
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
				updater.SetCondition(conditions.routeInvalidDiscoveryChain(err))
				validTargets = false
			}
			continue
		}

		if chain.Protocol != string(route.GetProtocol()) {
			if validTargets {
				updater.SetCondition(conditions.routeInvalidDiscoveryChain(errInvalidProtocol))
				validTargets = false
			}
			continue
		}

		// this makes sure we don't override an already set status
		if validTargets {
			updater.SetCondition(conditions.routeAccepted())
		}
	}

	// if we have no upstream targets, then set the route as invalid
	// this should already happen in the validation check on write, but
	// we'll do it here too just in case
	if len(route.GetServiceNames()) == 0 {
		updater.SetCondition(conditions.routeNoUpstreams())
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
		updater.SetCondition(conditions.routeBound(ref))
	}

	// set any binding errors
	for ref, err := range bindErrors {
		updater.SetCondition(conditions.routeUnbound(ref, err))
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

// referenceSet stores an O(1) accessible set of ResourceReference objects.
type referenceSet = map[structs.ResourceReference]any

// gatewayRefs maps a gateway kind/name to a set of resource references.
type gatewayRefs = map[configentry.KindName][]structs.ResourceReference

// BindRoutesToGateways takes a slice of bound API gateways and a variadic number of routes.
// It iterates over the parent references for each route. These parents are gateways the
// route should be bound to. If the parent matches a bound gateway, the route is bound to the
// gateway. Otherwise, the route is unbound from the gateway if it was previously bound.
//
// The function returns a list of references to the modified BoundAPIGatewayConfigEntry objects,
// a map of resource references to errors that occurred when they were attempted to be
// bound to a gateway.
func BindRoutesToGateways(gateways []*gatewayMeta, routes ...structs.BoundRoute) ([]*structs.BoundAPIGatewayConfigEntry, []structs.ResourceReference, map[structs.ResourceReference]error) {
	boundRefs := []structs.ResourceReference{}
	modified := make([]*structs.BoundAPIGatewayConfigEntry, 0, len(gateways))

	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	for _, route := range routes {
		parentRefs, gatewayRefs := getReferences(route)
		routeRef := structs.ResourceReference{
			Kind:           route.GetKind(),
			Name:           route.GetName(),
			EnterpriseMeta: *route.GetEnterpriseMeta(),
		}

		// Iterate over all BoundAPIGateway config entries and try to bind them to the route if they are a parent.
		for _, gateway := range gateways {
			references, routeReferencesGateway := gatewayRefs[configentry.NewKindNameForEntry(gateway.BoundGateway)]

			if routeReferencesGateway {
				didUpdate, errors := gateway.updateRouteBinding(references, route)

				if didUpdate {
					modified = append(modified, gateway.BoundGateway)
				}

				for ref, err := range errors {
					errored[ref] = err
				}

				for _, ref := range references {
					delete(parentRefs, ref)

					// this ref successfully bound, add it to the set that we'll update the
					// status for
					if _, found := errored[ref]; !found {
						boundRefs = append(boundRefs, references...)
					}
				}

				continue
			}

			if gateway.unbindRoute(routeRef) {
				modified = append(modified, gateway.BoundGateway)
			}
		}

		// Add all references that aren't bound at this point to the error set.
		for reference := range parentRefs {
			errored[reference] = errors.New("invalid reference to missing parent")
		}
	}

	return modified, boundRefs, errored
}

// getReferences returns a set of all the resource references for a given route as well as
// a map of gateway kind/name to a list of resource references for that gateway.
func getReferences(route structs.BoundRoute) (referenceSet, gatewayRefs) {
	parentRefs := make(referenceSet)
	gatewayRefs := make(gatewayRefs)

	for _, ref := range route.GetParents() {
		parentRefs[ref] = struct{}{}
		kindName := configentry.NewKindName(structs.BoundAPIGateway, ref.Name, pointerTo(ref.EnterpriseMeta))
		gatewayRefs[kindName] = append(gatewayRefs[kindName], ref)
	}

	return parentRefs, gatewayRefs
}

// RemoveGateway sets the route's status appropriately when the gateway that it's
// attempting to bind to does not exist
func RemoveGateway(gateway structs.ResourceReference, entries ...structs.BoundRoute) []structs.ControlledConfigEntry {
	conditions := newConditionGenerator()
	modified := []structs.ControlledConfigEntry{}

	for _, route := range entries {
		updater := structs.NewStatusUpdater(route)

		for _, parent := range route.GetParents() {
			if parent.Kind == gateway.Kind && parent.Name == gateway.Name && parent.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
				updater.SetCondition(conditions.gatewayNotFound(parent))
			}
		}

		if toUpdate, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
			modified = append(modified, toUpdate)
		}
	}

	return modified
}

// RemoveRoute unbinds the route from the given gateways, returning the list of gateways that were modified.
func RemoveRoute(route structs.ResourceReference, entries ...*gatewayMeta) []*gatewayMeta {
	modified := []*gatewayMeta{}

	for _, entry := range entries {
		if entry.unbindRoute(route) {
			modified = append(modified, entry)
		}
	}

	return modified
}

// gatewayMeta embeds both a BoundAPIGateway and its corresponding APIGateway.
// This is used when binding routes to a gateway to ensure that a route's protocol (e.g. http)
// matches the protocol of the listener it wants to bind to. The binding modifies the
// "bound" gateway, but relies on the "gateway" to determine the protocol of the listener.
type gatewayMeta struct {
	// BoundGateway is the bound-api-gateway config entry for a given gateway.
	BoundGateway *structs.BoundAPIGatewayConfigEntry
	// Gateway is the api-gateway config entry for the gateway.
	Gateway *structs.APIGatewayConfigEntry
}

// getAllGatewayMeta returns a pre-constructed list of all valid gateway and state
// tuples based on the state coming from the store. Any gateway that does not have
// a corresponding bound-api-gateway config entry will be filtered out.
func getAllGatewayMeta(store *state.Store) ([]*gatewayMeta, error) {
	_, gateways, err := store.ConfigEntriesByKind(nil, structs.APIGateway, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, err
	}
	_, boundGateways, err := store.ConfigEntriesByKind(nil, structs.BoundAPIGateway, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, err
	}

	meta := make([]*gatewayMeta, 0, len(boundGateways))
	for _, b := range boundGateways {
		bound := b.(*structs.BoundAPIGatewayConfigEntry)
		for _, g := range gateways {
			gateway := g.(*structs.APIGatewayConfigEntry)
			if bound.IsInitializedForGateway(gateway) {
				meta = append(meta, &gatewayMeta{
					BoundGateway: bound,
					Gateway:      gateway,
				})
				break
			}
		}
	}
	return meta, nil
}

// updateRouteBinding takes a parent resource reference and a BoundRoute and
// modifies the listeners on the BoundAPIGateway config entry in GatewayMeta
// to reflect the binding of the route to the gateway.
//
// If the reference is not valid or the route's protocol does not match the
// targeted listener's protocol, a mapping of parent references to associated
// errors is returned.
func (g *gatewayMeta) updateRouteBinding(refs []structs.ResourceReference, route structs.BoundRoute) (bool, map[structs.ResourceReference]error) {
	if g.BoundGateway == nil || g.Gateway == nil {
		return false, nil
	}

	didUpdate := false
	errors := make(map[structs.ResourceReference]error)

	if len(g.BoundGateway.Listeners) == 0 {
		for _, ref := range refs {
			errors[ref] = fmt.Errorf("route cannot bind because gateway has no listeners")
		}
		return false, errors
	}

	unboundListeners := make([]bool, 0, len(g.BoundGateway.Listeners))
	for i, listener := range g.BoundGateway.Listeners {
		routeRef := structs.ResourceReference{
			Kind:           route.GetKind(),
			Name:           route.GetName(),
			EnterpriseMeta: *route.GetEnterpriseMeta(),
		}
		// Unbind to handle any stale route references.
		unboundListeners = append(unboundListeners, listener.UnbindRoute(routeRef))
		g.BoundGateway.Listeners[i] = listener
	}

	for _, ref := range refs {
		boundListeners, err := g.bindRoute(ref, route)
		if err != nil {
			errors[ref] = err
		} else {
			for i, didUnbind := range unboundListeners {
				if i >= len(boundListeners) || didUnbind != boundListeners[i] {
					didUpdate = true
				}
			}
		}
	}

	return didUpdate, errors
}

// bindRoute takes a parent reference and a route and attempts to bind the route to the
// bound gateway in the gatewayMeta struct. It returns true if the route was bound and
// false if it was not. If the route fails to bind, an error is returned.
//
// Binding logic binds a route to one or more listeners on the Bound gateway.
// For a route to successfully bind it must:
//   - have a parent reference to the gateway
//   - have a parent reference with a section name matching the name of a listener
//     on the gateway. If the section name is `""`, the route will be bound to all
//     listeners on the gateway whose protocol matches the route's protocol.
//   - have a protocol that matches the protocol of the listener it is being bound to.
func (g *gatewayMeta) bindRoute(ref structs.ResourceReference, route structs.BoundRoute) ([]bool, error) {
	if g.BoundGateway == nil || g.Gateway == nil {
		return nil, fmt.Errorf("gateway cannot be found")
	}

	if ref.Kind != structs.APIGateway || g.Gateway.Name != ref.Name || !g.Gateway.EnterpriseMeta.IsSame(&ref.EnterpriseMeta) {
		return nil, nil
	}

	if len(g.BoundGateway.Listeners) == 0 {
		return nil, fmt.Errorf("route cannot bind because gateway has no listeners")
	}

	binding := make([]bool, 0, len(g.Gateway.Listeners))
	for _, listener := range g.Gateway.Listeners {
		// A route with a section name of "" is bound to all listeners on the gateway.
		if listener.Name != ref.SectionName && ref.SectionName != "" {
			binding = append(binding, false)
			continue
		}

		if listener.Protocol == route.GetProtocol() {
			routeRef := structs.ResourceReference{
				Kind:           route.GetKind(),
				Name:           route.GetName(),
				EnterpriseMeta: *route.GetEnterpriseMeta(),
			}
			i, boundListener := g.boundListenerByName(listener.Name)
			if boundListener != nil && boundListener.BindRoute(routeRef) {
				binding = append(binding, true)
				g.BoundGateway.Listeners[i] = *boundListener
			} else {
				binding = append(binding, false)
			}
		} else if ref.SectionName != "" {
			// Failure to bind to a specific listener is an error
			return nil, fmt.Errorf("failed to bind route %s to gateway %s: listener %s is not a %s listener", route.GetName(), g.Gateway.Name, listener.Name, route.GetProtocol())
		}
	}

	didBind := false
	for _, bound := range binding {
		if bound {
			didBind = true
		}
	}

	if !didBind {
		return nil, fmt.Errorf("failed to bind route %s to gateway %s: no valid listener has name '%s' and uses %s protocol", route.GetName(), g.Gateway.Name, ref.SectionName, route.GetProtocol())
	}

	return binding, nil
}

// unbindRoute takes a route and unbinds it from all of the listeners on a gateway.
// It returns true if the route was unbound and false if it was not.
func (g *gatewayMeta) unbindRoute(route structs.ResourceReference) bool {
	if g.BoundGateway == nil {
		return false
	}

	didUnbind := false
	for i, listener := range g.BoundGateway.Listeners {
		if listener.UnbindRoute(route) {
			didUnbind = true
			g.BoundGateway.Listeners[i] = listener
		}
	}

	return didUnbind
}

func (g *gatewayMeta) boundListenerByName(name string) (int, *structs.BoundAPIGatewayListener) {
	for i, listener := range g.BoundGateway.Listeners {
		if listener.Name == name {
			return i, &listener
		}
	}
	return -1, nil
}

// checkCertificates verifies that all certificates referenced by the listeners on the gateway
// exist and collects them onto the bound gateway
func (g *gatewayMeta) checkCertificates(store *state.Store) (map[structs.ResourceReference]error, error) {
	certificateErrors := map[structs.ResourceReference]error{}
	for i, listener := range g.Gateway.Listeners {
		bound := g.BoundGateway.Listeners[i]
		for _, ref := range listener.TLS.Certificates {
			_, certificate, err := store.ConfigEntry(nil, ref.Kind, ref.Name, &ref.EnterpriseMeta)
			if err != nil {
				return nil, err
			}
			if certificate == nil {
				certificateErrors[ref] = errors.New("certificate not found")
			} else {
				bound.Certificates = append(bound.Certificates, ref)
			}
		}
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
	conditions := newConditionGenerator()

	for i, listener := range g.BoundGateway.Listeners {
		ref := structs.ResourceReference{
			Kind:           structs.APIGateway,
			Name:           g.Gateway.Name,
			SectionName:    listener.Name,
			EnterpriseMeta: g.Gateway.EnterpriseMeta,
		}

		protocol := g.Gateway.Listeners[i].Protocol
		switch protocol {
		case structs.ListenerProtocolTCP:
			if len(listener.Routes) > 1 {
				updater.SetCondition(conditions.gatewayListenerConflicts(ref))
				continue
			}
		}
		updater.SetCondition(conditions.gatewayListenerNoConflicts(ref))
	}
}

func ensureInitializedMeta(gateway *structs.APIGatewayConfigEntry, bound structs.ConfigEntry) *gatewayMeta {
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

	return &gatewayMeta{
		BoundGateway: b,
		Gateway:      gateway,
	}
}

type conditionGenerator struct {
	now *time.Time
}

func newConditionGenerator() *conditionGenerator {
	return &conditionGenerator{
		now: pointerTo(time.Now().UTC()),
	}
}

func (g *conditionGenerator) invalidCertificate(ref structs.ResourceReference, err error) structs.Condition {
	return structs.Condition{
		Type:               "Accepted",
		Status:             "False",
		Reason:             "InvalidCertificate",
		Message:            err.Error(),
		Resource:           pointerTo(ref),
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) invalidCertificates() structs.Condition {
	return structs.Condition{
		Type:               "Accepted",
		Status:             "False",
		Reason:             "InvalidCertificates",
		Message:            "gateway references invalid certificates",
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) gatewayAccepted() structs.Condition {
	return structs.Condition{
		Type:               "Accepted",
		Status:             "True",
		Reason:             "Accepted",
		Message:            "gateway is valid",
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) routeBound(ref structs.ResourceReference) structs.Condition {
	return structs.Condition{
		Type:               "Bound",
		Status:             "True",
		Reason:             "Bound",
		Resource:           pointerTo(ref),
		Message:            "successfully bound route",
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) routeAccepted() structs.Condition {
	return structs.Condition{
		Type:               "Accepted",
		Status:             "True",
		Reason:             "Accepted",
		Message:            "route is valid",
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) routeUnbound(ref structs.ResourceReference, err error) structs.Condition {
	return structs.Condition{
		Type:               "Bound",
		Status:             "False",
		Reason:             "FailedToBind",
		Resource:           pointerTo(ref),
		Message:            err.Error(),
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) routeInvalidDiscoveryChain(err error) structs.Condition {
	return structs.Condition{
		Type:               "Accepted",
		Status:             "False",
		Reason:             "InvalidDiscoveryChain",
		Message:            err.Error(),
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) routeNoUpstreams() structs.Condition {
	return structs.Condition{
		Type:               "Accepted",
		Status:             "False",
		Reason:             "NoUpstreamServicesTargeted",
		Message:            "route must target at least one upstream service",
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) gatewayListenerConflicts(ref structs.ResourceReference) structs.Condition {
	return structs.Condition{
		Type:               "Conflicted",
		Status:             "True",
		Reason:             "RouteConflict",
		Resource:           pointerTo(ref),
		Message:            "TCP-based listeners currently only support binding a single route",
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) gatewayListenerNoConflicts(ref structs.ResourceReference) structs.Condition {
	return structs.Condition{
		Type:               "Conflicted",
		Status:             "False",
		Reason:             "NoConflict",
		Resource:           pointerTo(ref),
		Message:            "listener has no route conflicts",
		LastTransitionTime: g.now,
	}
}

func (g *conditionGenerator) gatewayNotFound(ref structs.ResourceReference) structs.Condition {
	return structs.Condition{
		Type:               "Bound",
		Status:             "False",
		Reason:             "GatewayNotFound",
		Resource:           pointerTo(ref),
		Message:            "gateway was not found",
		LastTransitionTime: g.now,
	}
}

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
