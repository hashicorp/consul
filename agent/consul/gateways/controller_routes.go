package gateways

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

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

// routeReconciler is a reconciliation control loop
// handler for routes
type routeReconciler[T structs.BoundRoute] struct {
	logger     hclog.Logger
	fsm        *fsm.FSM
	controller controller.Controller
	updater    *Updater
}

func newRouteReconciler[T structs.BoundRoute](
	logger hclog.Logger,
	fsm *fsm.FSM,
	updater *Updater,
) *routeReconciler[T] {
	reconciler := new(routeReconciler[T])
	reconciler.logger = logger
	reconciler.fsm = fsm
	reconciler.updater = updater
	return reconciler
}

// Reconcile reconciles Route config entries.
func (r *routeReconciler[T]) Reconcile(ctx context.Context, req controller.Request) error {
	store := r.fsm.State()

	_, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, req.Meta)
	if err != nil {
		return err
	}

	meta, err := getAllGatewayMeta(store)
	if err != nil {
		return err
	}

	if entry == nil {
		r.logger.Error("cleaning up deleted route object", "request", req)
		for _, toUpdate := range RemoveRoute(requestToResourceRef(req), meta...) {
			if err := r.updater.Update(toUpdate.BoundGateway); err != nil {
				return err
			}
		}
		r.controller.RemoveTrigger(req)
		return nil
	}

	now := time.Now()

	r.logger.Error("got route reconcile call", "request", req)

	updater := structs.NewStatusUpdater(entry.(structs.ControlledConfigEntry))
	// we clear out the initial status conditions since we're doing a full update
	// of this route's status
	updater.ClearConditions()

	ws := memdb.NewWatchSet()
	ws.Add(store.AbandonCh())

	route := entry.(T)
	for _, service := range route.GetTargetedServices() {
		_, chainSet, err := store.ReadDiscoveryChainConfigEntries(ws, service.Name, &service.EnterpriseMeta)
		if err != nil {
			return err
		}
		// add a simple router and corresponding defaults to make sure we have protocol alignment
		chainSet.AddServices(&structs.ServiceConfigEntry{
			Kind:           structs.ServiceDefaults,
			Name:           route.GetName(),
			EnterpriseMeta: *route.GetEnterpriseMeta(),
			Protocol:       string(route.GetProtocol()),
		})
		chainSet.AddRouters(&structs.ServiceRouterConfigEntry{
			Kind:           structs.ServiceRouter,
			Name:           route.GetName(),
			EnterpriseMeta: *route.GetEnterpriseMeta(),
			Routes: []structs.ServiceRoute{{
				Destination: &structs.ServiceRouteDestination{
					Service:   service.Name,
					Namespace: service.NamespaceOrDefault(),
					Partition: service.PartitionOrDefault(),
				},
			}},
		})

		// make sure that we can actually compile a discovery chain based on this route
		// the main check is to make sure that all of the protocols align
		_, err = discoverychain.Compile(discoverychain.CompileRequest{
			ServiceName:          route.GetName(),
			EvaluateInNamespace:  route.GetEnterpriseMeta().NamespaceOrDefault(),
			EvaluateInPartition:  route.GetEnterpriseMeta().PartitionOrDefault(),
			EvaluateInDatacenter: "dc1", // just mock out a fake dc since we're just checking for compilation errors
			Entries:              chainSet,
		})
		if err != nil {
			updater.SetCondition(structs.Condition{
				Type:               "Accepted",
				Status:             "False",
				Reason:             "InvalidDiscoveryChain",
				Message:            err.Error(),
				LastTransitionTime: &now,
			})
			if toUpdate, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
				return r.updater.UpdateWithStatus(toUpdate)
			}
			return nil
		}
	}

	// the route is valid, attempt to bind it to all gateways
	modifiedGateways, boundRefs, bindErrors := BindRoutesToGateways(meta, route)
	if err != nil {
		return err
	}
	for _, ref := range boundRefs {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "True",
			Reason:             "Bound",
			Resource:           &ref,
			Message:            "successfully bound route",
			LastTransitionTime: &now,
		})
	}
	for reference, err := range bindErrors {
		updater.SetCondition(structs.Condition{
			Type:               "Accepted",
			Status:             "False",
			Reason:             "FailedToBind",
			Resource:           &reference,
			Message:            err.Error(),
			LastTransitionTime: &now,
		})
	}

	// first update all of the state values
	for _, state := range modifiedGateways {
		r.logger.Debug("persisting gateway state", "state", state)
		if err := r.updater.Update(state); err != nil {
			r.logger.Error("error persisting state", "error", err)
			return err
		}
	}

	// now update the route status
	if toUpdate, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
		return r.updater.UpdateWithStatus(toUpdate)
	}

	if ws != nil {
		// finally add a trigger to re-reconcile when the discovery chain is
		// invalidated
		r.controller.AddTrigger(req, ws.WatchCtx)
	}

	return nil
}

func NewTCPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, updater *Updater, logger hclog.Logger) controller.Controller {
	reconciler := newRouteReconciler[*structs.TCPRouteConfigEntry](logger, fsm, updater)
	controller := controller.New(publisher, reconciler)
	reconciler.controller = controller

	return controller.Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicTCPRoute,
		Subject: stream.SubjectWildcard,
	})
}

func NewHTTPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, updater *Updater, logger hclog.Logger) controller.Controller {
	reconciler := newRouteReconciler[*structs.HTTPRouteConfigEntry](logger, fsm, updater)
	controller := controller.New(publisher, reconciler)
	reconciler.controller = controller

	return controller.Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicHTTPRoute,
		Subject: stream.SubjectWildcard,
	})
}
