package gateways

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/gateways/datastore"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
)

type apiGatewayReconciler struct {
	fsm    *fsm.FSM
	logger hclog.Logger
	store  datastore.DataStore
}

func (r apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	entry, err := r.store.GetConfigEntry(req.Kind, req.Name, req.Meta)
	if err != nil {
		return err
	}

	if entry == nil {
		r.logger.Error("cleaning up deleted gateway object", "request", req)
		if err := r.store.Delete(&structs.BoundAPIGatewayConfigEntry{
			Kind:           structs.BoundAPIGateway,
			Name:           req.Name,
			EnterpriseMeta: *req.Meta,
		}); err != nil {
			return err
		}
		return nil
	}

	gateway := entry.(*structs.BoundAPIGatewayConfigEntry)
	// TODO: validation?

	var state *structs.BoundAPIGatewayConfigEntry
	boundEntry, err := r.store.GetConfigEntry(structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		return err
	}
	if boundEntry == nil {
		state = &structs.BoundAPIGatewayConfigEntry{
			Kind:           structs.BoundAPIGateway,
			Name:           gateway.Name,
			EnterpriseMeta: gateway.EnterpriseMeta,
		}
	} else {
		state = boundEntry.(*structs.BoundAPIGatewayConfigEntry)
	}

	r.logger.Debug("started reconciling gateway")
	routes := []structs.BoundRoute{}
	for _, listener := range state.Listeners {
		for _, route := range listener.Routes {
			routeEntry, err := r.store.GetConfigEntry(route.Kind, route.Name, &route.EnterpriseMeta)
			if err != nil {
				return err
			}

			var routeBoundRouter structs.BoundRoute
			switch routeEntry.GetKind() {
			case structs.TCPRoute:
				fmt.Println("tcp")
				routeBoundRouter = (routeEntry).(*structs.TCPRouteConfigEntry)
				routes = append(routes, routeBoundRouter)
			case structs.HTTPRoute:
				fmt.Println("not implemented")
			default:
				return errors.New("route type doesn't exist")
			}

		}

		boundGateway, routeErrors, err := BindRoutesToGateways(structs.gateway, routes)

		// now update the gateway state
		r.logger.Debug("persisting gateway state", "state", state)
		if err := r.store.Update(&boundGateway); err != nil {
			r.logger.Error("error persisting state", "error", err)
			return err
		}

		// then update the gateway status
		r.logger.Debug("persisting gateway status", "gateway", gateway)
		if err := r.store.UpdateStatus(gateway); err != nil {
			return err
		}

		//// and update the route statuses
		for route, err := range routeErrors {
			r.logger.Debug("persisting route status", "route", route)
			if err := r.store.UpdateStatus(route); err != nil {
				return err
			}
		}

	}

	return nil
}

func NewAPIGatewayController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := apiGatewayReconciler{
		fsm:    fsm,
		logger: logger,
	}
	return controller.New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicAPIGateway,
			Subject: stream.SubjectWildcard,
		},
	)
}
