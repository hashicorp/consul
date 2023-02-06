package gateways

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
)

type tcpRouteReconciler struct {
	fsm    *fsm.FSM
	logger hclog.Logger
}

func (r tcpRouteReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	return nil
}

func NewTCPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := tcpRouteReconciler{
		fsm:    fsm,
		logger: logger,
	}
	return controller.New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicTCPRoute,
			Subject: stream.SubjectWildcard,
		},
	)
}

type httpRouteReconciler struct {
	fsm    *fsm.FSM
	logger hclog.Logger
}

func (r httpRouteReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	return nil
}

func NewHTTPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := httpRouteReconciler{
		fsm:    fsm,
		logger: logger,
	}
	return controller.New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicHTTPRoute,
			Subject: stream.SubjectWildcard,
		},
	)
}
