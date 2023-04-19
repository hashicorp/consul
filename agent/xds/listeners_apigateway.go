package xds

import (
	"fmt"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func refToAPIGatewayListenerKey(cfgSnap *proxycfg.ConfigSnapshot, listenerRef structs.ResourceReference) (*structs.APIGatewayListener, *structs.BoundAPIGatewayListener, *proxycfg.APIGatewayListenerKey, error) {
	listenerCfg, ok := cfgSnap.APIGateway.Listeners[listenerRef.Name]
	if !ok {
		return nil, nil, nil, fmt.Errorf("no listener config found for listener %s", listenerCfg.Name)
	}

	boundListenerCfg, ok := cfgSnap.APIGateway.BoundListeners[listenerRef.Name]
	if !ok {
		return nil, nil, nil, fmt.Errorf("no listener config found for listener %s", listenerCfg.Name)
	}
	return &listenerCfg, &boundListenerCfg, &proxycfg.APIGatewayListenerKey{Port: listenerCfg.Port, Protocol: string(listenerCfg.Protocol)}, nil

}
