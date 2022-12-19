package gateways

import (
	"context"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-hclog"
)

//baseReconciler contains fields shared across multiple controller.Reconciler implementations in the gateways package
type baseReconciler struct {
	fsm    *fsm.FSM
	logger hclog.Logger
}

type apiGatewayReconciler struct {
	baseReconciler
}

func (r apiGatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	return nil
}

func APIGatewayController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := apiGatewayReconciler{
		baseReconciler{
			fsm:    fsm,
			logger: logger,
		},
	}
	return controller.New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicAPIGateway,
			Subject: stream.SubjectWildcard,
		},
	)
}
