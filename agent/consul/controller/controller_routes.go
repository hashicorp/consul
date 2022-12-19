package controller

import (
	"context"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-hclog"
)

type tcpRouteReconciler struct {
	baseReconciler
}

func (r tcpRouteReconciler) Reconcile(ctx context.Context, req Request) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return nil
}

func TCPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) Controller {
	reconciler := tcpRouteReconciler{
		baseReconciler{
			fsm:    fsm,
			logger: logger,
		},
	}
	return New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicTCPRoute,
			Subject: stream.SubjectWildcard,
		},
	)
}

type httpRouteReconciler struct {
	baseReconciler
}

func (r httpRouteReconciler) Reconcile(ctx context.Context, req Request) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return nil
}

func HTTPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) Controller {
	reconciler := httpRouteReconciler{
		baseReconciler{
			fsm:    fsm,
			logger: logger,
		},
	}
	return New(publisher, reconciler).Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicHTTPRoute,
			Subject: stream.SubjectWildcard,
		},
	)
}
