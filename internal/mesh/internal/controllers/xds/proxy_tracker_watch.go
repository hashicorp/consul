package xds

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	proxytracker "github.com/hashicorp/consul/internal/mesh/proxy-tracker"
)

func proxySource(updater ProxyUpdater) *controller.Source {
	return &controller.Source{Source: updater.EventChannel()}
}

func proxyMapper(ctx context.Context, rt controller.Runtime, event controller.Event) ([]controller.Request, error) {
	connection, ok := event.Obj.(*proxytracker.ProxyConnection)
	if !ok {
		return nil, fmt.Errorf("expected event to be of type *proxytracker.ProxyConnection but was %+v", event)
	}
	return []controller.Request{{ID: connection.ProxyID}}, nil
}
