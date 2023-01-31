package gateways

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
)

type apiGatewayReconciler struct {
	fsm    *fsm.FSM
	logger hclog.Logger
}

func (r apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
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
