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
	// TODO: do initial distributed validation here, if we're invalid, then set the status

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
	for _, listener := range state.Listeners {
		//TODO swap over to Thomases method
		for _, route := range listener.Routes {
			fmt.Println(route)
			//routes that didn't have a gateway that exists
			routeEntry, err := r.store.GetConfigEntry(route.Kind, route.Name, &route.EnterpriseMeta)
			if err != nil {
				return err
			}

			var routeBoundRouter structs.BoundRouter
			switch routeEntry.GetKind() {
			case structs.TCPRoute:
				fmt.Println("tcp")
				routeBoundRouter = (routeEntry).(*structs.TCPRouteConfigEntry)
			case structs.HTTPRoute:
				fmt.Println("not implemented")
			default:
				return errors.New("route type doesn't exist")
			}
			_, _, err = BindRouteToGateways(store, routeBoundRouter)
			if err != nil {
				return err
			}

			// now update the gateway state
			r.logger.Debug("persisting gateway state", "state", state)
			if err := r.store.Update(state); err != nil {
				r.logger.Error("error persisting state", "error", err)
				return err
			}

			// then update the gateway status
			r.logger.Debug("persisting gateway status", "gateway", gateway)
			if err := r.store.UpdateStatus(gateway); err != nil {
				return err
			}

			//// and update the route statuses ?
			//r.logger.Debug("persisting route status", "route", route)
			//if err := r.store.UpdateStatus(route); err != nil {
			//	return err
			//}

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
