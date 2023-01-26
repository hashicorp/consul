package gateways

import (
	"context"
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

func (r apiGatewayReconciler) retrieveAllRoutesFromStore() ([]structs.BoundRoute, error) {
	tcpRoutes, err := r.store.GetConfigEntriesByKind(structs.TCPRoute)
	if err != nil {
		return nil, err
	}

	//TODO not implemented
	//httpRoutes, err := r.store.GetConfigEntriesByKind(structs.HTTPRoute)
	//if err != nil {
	//	return nil, err
	//}

	routes := []structs.BoundRoute{}
	for _, r := range tcpRoutes {
		if r == nil {
			continue
		}
		routes = append(routes, r.(*structs.TCPRouteConfigEntry))
	}
	//TODO not implemented
	//for _, r := range httpRoutes {
	//	routes = append(routes, r.(*structs.HTTPRouteConfigEntry))
	//}
	return routes, nil
}

func (r apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	entry, err := r.store.GetConfigEntry(req.Kind, req.Name, req.Meta)
	if err != nil {
		return err
	}

	if entry == nil {
		r.logger.Info("cleaning up deleted gateway object", "request", req)
		if err := r.store.Delete(&structs.BoundAPIGatewayConfigEntry{
			Kind:           structs.BoundAPIGateway,
			Name:           req.Name,
			EnterpriseMeta: *req.Meta,
		}); err != nil {
			r.logger.Error("error cleaning up deleted gateway object", err)
			return err
		}
		return nil
	}

	gatewayEntry := entry.(*structs.APIGatewayConfigEntry)
	//TODO is this what needs to happen for the validation step
	err = gatewayEntry.Validate()
	if err != nil {
		r.logger.Debug("persisting gateway status", "gateway", gatewayEntry)
		if updateErr := r.store.UpdateStatus(gatewayEntry); err != nil {
			return fmt.Errorf("%v: %v", err, updateErr)
		}
		return err
	}

	var boundGatewayEntry *structs.BoundAPIGatewayConfigEntry
	boundEntry, err := r.store.GetConfigEntry(structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		return err
	}
	if boundEntry == nil {
		boundGatewayEntry = &structs.BoundAPIGatewayConfigEntry{
			Kind:           structs.BoundAPIGateway,
			Name:           gatewayEntry.Name,
			EnterpriseMeta: gatewayEntry.EnterpriseMeta,
		}
	} else {
		boundGatewayEntry = boundEntry.(*structs.BoundAPIGatewayConfigEntry)
	}

	r.logger.Debug("started reconciling gateway")

	routes, err := r.retrieveAllRoutesFromStore()

	if err != nil {
		return err
	}
	boundGateways, routeErrors := BindRoutesToGateways(wrapGatewaysInSlice(boundGatewayEntry), routes...)

	if len(boundGateways) > 1 {
		err := fmt.Errorf("bind returned more gateways (%d) than it was given (1)", len(boundGateways))
		r.logger.Error("API Gateway Reconciler failed to reconcile: %v", err)
		return err
	}

	if len(boundGateways) == 0 && len(routeErrors) == 0 {
		r.logger.Debug("API Gateway Reconciler: gateway %s reconciled without updates.")
		return nil
	}

	boundGateway := boundGateways[0]
	fmt.Println(boundGateway)
	fmt.Println(boundGatewayEntry)
	fmt.Println("hellp")

	// now update the gateway state
	r.logger.Debug("persisting gateway state", "state", boundGateway)
	if err := r.store.Update(boundGateway); err != nil {
		r.logger.Error("error persisting state", "error", err)
		return err
	}

	// then update the gateway status
	r.logger.Debug("persisting gateway status", "gateway", gatewayEntry)
	if err := r.store.UpdateStatus(gatewayEntry); err != nil {
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

	return nil
}

// convenience wrapper
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
