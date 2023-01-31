package gateways

import (
	"context"
	"github.com/hashicorp/consul/agent/consul/controller"
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

// NewAPIGatewayController returns a new APIGateway controller
func NewAPIGatewayController(store datastore.DataStore, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := apiGatewayReconciler{
		logger: logger,
		store:  store,
	}
	return controller.New(publisher, &reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicAPIGateway,
			Subject: stream.SubjectWildcard,
		},
	)
}

// Reconcile takes in a controller request and ensures this api gateways corresponding BoundAPIGateway exists and is
// up to date
func (r *apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	metaGateway, err := r.initGatewayMeta(req)
	if err != nil {
		return err
	} else if metaGateway == nil && err == nil {
		//delete meta gateway
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

	r.ensureBoundGateway(metaGateway)

	r.logger.Debug("started reconciling gateway")

	routes, err := r.retrieveAllRoutesFromStore()

	if err != nil {
		return err
	}

	boundGateways, routeErrors := BindRoutesToGateways([]*gatewayMeta{metaGateway}, routes...)

	//In this loop there should only be 1 bound gateway returned, but looping over all returned gateways
	//to make sure nothing gets dropped and handle case where 0 gateways are returned
	for _, boundGateway := range boundGateways {
		// now update the gateway state
		r.logger.Debug("persisting gateway state", "state", boundGateway)
		if err := r.store.Update(boundGateway); err != nil {
			r.logger.Error("error persisting state", "error", err)
			return err
		}

		// then update the gateway status
		r.logger.Debug("persisting gateway status", "gateway", metaGateway.Gateway)
		if err := r.store.UpdateStatus(metaGateway.Gateway); err != nil {
			return err
		}
	}

	//// and update the route statuses
	for route, routeError := range routeErrors {
		//TODO find out if parents are needed for status updates

		configEntry := resourceReferenceToBoundRoute(route, []structs.ResourceReference{})
		//TODO pull in update status work
		r.logger.Error("route binding error:", routeError)
		if err := r.store.UpdateStatus(configEntry); err != nil {
			return err
		}
	}

	return nil
}

func (r *apiGatewayReconciler) retrieveAllRoutesFromStore() ([]structs.BoundRoute, error) {
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

func (r *apiGatewayReconciler) initGatewayMeta(req controller.Request) (*gatewayMeta, error) {
	metaGateway := &gatewayMeta{}

	apiGateway, err := r.store.GetConfigEntry(req.Kind, req.Name, req.Meta)
	if err != nil {
		return nil, err
	}

	if apiGateway == nil {
		//gateway needs to be deleted, don't init
		return nil, nil
	}

	metaGateway.Gateway = apiGateway.(*structs.APIGatewayConfigEntry)

	boundGateway, err := r.store.GetConfigEntry(structs.BoundAPIGateway, req.Name, req.Meta)
	if err != nil {
		return nil, err
	}

	//initialize object, values get copied over in ensureBoundGateway if they don't exist
	metaGateway.BoundGateway = boundGateway.(*structs.BoundAPIGatewayConfigEntry)
	return metaGateway, nil
}

func resourceReferenceToBoundRoute(ref structs.ResourceReference, parents []structs.ResourceReference) structs.ConfigEntry {
	//TODO handle other types
	return &structs.TCPRouteConfigEntry{
		Kind:    ref.Kind,
		Name:    ref.Name,
		Parents: parents,
	}
}

// ensureBoundGateway copies all relevant data from a gatewayMeta's APIGateway to BoundAPIGateway
func (r *apiGatewayReconciler) ensureBoundGateway(gw *gatewayMeta) {
	if gw.BoundGateway == nil {
		gw.BoundGateway = &structs.BoundAPIGatewayConfigEntry{
			Kind:           structs.BoundAPIGateway,
			Name:           gw.Gateway.Name,
			EnterpriseMeta: gw.Gateway.EnterpriseMeta,
		}
	}

	r.ensureListeners(gw)
}

func (r *apiGatewayReconciler) ensureListeners(gw *gatewayMeta) {

	//rebuild the list from scratch, just copying over the ones that already exist
	listeners := []structs.BoundAPIGatewayListener{}
	for _, l := range gw.Gateway.Listeners {
		boundListener := getBoundGatewayListener(l, gw.BoundGateway.Listeners)
		if boundListener != nil {
			//listener is already on gateway, copy onto our new list
			listeners = append(listeners, *boundListener)
			continue
		}
		//create new listener to add to our gateway
		listeners = append(listeners, structs.BoundAPIGatewayListener{
			Name: l.Name,
		})
	}
	gw.BoundGateway.Listeners = listeners
}

func getBoundGatewayListener(listener structs.APIGatewayListener, boundListeners []structs.BoundAPIGatewayListener) *structs.BoundAPIGatewayListener {
	for _, bl := range boundListeners {
		if bl.Name == listener.Name {
			return &bl
		}
	}
	return nil
}
