package controller

import (
	"context"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-hclog"
)

type apiGatewayReconciler struct {
	baseReconciler
}

func (r apiGatewayReconciler) Reconcile(ctx context.Context, req Request) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return nil
}

func APIGatewayController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) Controller {
	reconciler := apiGatewayReconciler{
		baseReconciler{
			fsm:    fsm,
			logger: logger,
		},
	}
	return New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicAPIGateway,
			Subject: stream.SubjectWildcard,
		},
	)
}
