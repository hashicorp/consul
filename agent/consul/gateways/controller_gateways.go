package gateways

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/gateways/updater"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/consul/agent/structs"
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
		if err := r.updater.Delete(&structs.BoundGatewayConfigEntry{
			Kind:           structs.BoundGateway,
			Name:           req.Name,
			EnterpriseMeta: req.Meta,
		}); err != nil {
			return err
		}
		return nil
	}

	gateway := entry.(*structs.GatewayConfigEntry)
	// TODO: do initial distributed validation here, if we're invalid, then set the status

	var state *structs.BoundGatewayConfigEntry
	_, boundEntry, err := store.ConfigEntry(nil, structs.BoundGateway, req.Name, &req.Meta)
	if err != nil {
		return err
	}
	if boundEntry == nil {
		state = &structs.BoundGatewayConfigEntry{
			Kind:           structs.BoundGateway,
			Name:           gateway.Name,
			EnterpriseMeta: gateway.EnterpriseMeta,
		}
	} else {
		state = boundEntry.(*structs.BoundGatewayConfigEntry)
	}

	r.logger.Debug("started reconciling gateway")
	routes, err := BindTCPRoutes(store, state)
	if err != nil {
		return err
	}

	// now update the gateway state
	r.logger.Debug("persisting gateway state", "state", state)
	if err := r.updater.Update(state); err != nil {
		r.logger.Error("error persisting state", "error", err)
		return err
	}

	// then update the gateway status
	r.logger.Debug("persisting gateway status", "gateway", gateway)
	if err := r.updater.UpdateStatus(gateway); err != nil {
		return err
	}

	// and update the route statuses
	for _, route := range routes {
		r.logger.Debug("persisting route status", "route", route)
		if err := r.updater.UpdateStatus(route); err != nil {
			return err
		}
	}

	r.logger.Debug("finished reconciling gateway")
	return nil
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
