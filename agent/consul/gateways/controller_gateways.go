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
	//TODO is this what needs to happen for the validation step
	err = gateway.Validate()
	if err != nil {
		return err
	}

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

		boundGateways, routeErrors, err := BindRoutesToGateways(wrapGatewaysInSlice(gateway), routes)
		if err != nil {
			r.logger.Error("error binding route", "error", err)
			return err
		}

		if len(boundGateways) > 1 {
			r.logger.Warn("imlpementation error in bind routes, state should be impossible")
			return errors.New("multiple gateways bound")
		}

		boundGateway := boundGateways[0]

		// now update the gateway state
		r.logger.Debug("persisting gateway state", "state", state)
		if err := r.store.Update(boundGateway); err != nil {
			r.logger.Error("error persisting state", "error", err)
			return err
		}

		// then update the gateway status
		r.logger.Debug("persisting gateway status", "gateway", gateway)
		if err := r.store.UpdateStatus(gateway); err != nil {
			return err
		}

		//// and update the route statuses
		for _, listener := range boundGateway.Listeners {
			for _, route := range listener.Routes {
				routeErr, ok := routeErrors[route]
				if ok {
					r.logger.Error("route error", routeErr)
					//TODO does anything special need to be done in this situaion?
				}
				//TODO find out if parents are needed for status updates
				configEntry := resourceReferenceToBoundRoute(route, []structs.ResourceReference{})
				if err := r.store.UpdateStatus(configEntry); err != nil {
					return err
				}

			}
		}

	}

	return nil
}

//convenience wrapper
func wrapGatewaysInSlice(gateways ...*structs.BoundAPIGatewayConfigEntry) []*structs.BoundAPIGatewayConfigEntry {
	return gateways
}

func resourceReferenceToBoundRoute(ref structs.ResourceReference, parents []structs.ResourceReference) structs.ConfigEntry {
	//TODO handle other types
	return &structs.TCPRouteConfigEntry{
		Kind:    ref.Kind,
		Name:    ref.Name,
		Parents: parents,
	}

}

func NewAPIGatewayController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := apiGatewayReconciler{
		logger: logger,
	}
	return controller.New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicAPIGateway,
			Subject: stream.SubjectWildcard,
		},
	)
}
