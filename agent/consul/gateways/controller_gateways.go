package gateways

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/gateways/updater"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/consul/agent/structs"
	"errors"
)

type apiGatewayReconciler struct {
	fsm    *fsm.FSM
	logger hclog.Logger
	updater updater.Updater
}

func (r apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	store := r.fsm.State()

	_, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, req.Meta)
	if err != nil {
		return err
	}

	if entry == nil {
		r.logger.Error("cleaning up deleted gateway object", "request", req)
		if err := r.updater.Delete(&structs.BoundAPIGatewayConfigEntry{
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
	_, boundEntry, err := store.ConfigEntry(nil, structs.BoundAPIGateway, req.Name, req.Meta)
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
		for _, route := range listener.Routes {
			fmt.Println(route)
			//routes that didn't have a gateway that exists
			_, routeEntry, err := store.ConfigEntry(nil, route.Kind, route.Name, &route.EnterpriseMeta)
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
			gateways, routesWithNoGateways, err := BindRouteToGateways(store, routeBoundRouter)
			if err != nil {
				return err
			}
		}
	}




}
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
