package stream

import (
	"github.com/hashicorp/consul/agent/structs"
)

func FromCheckServiceNode(n *CheckServiceNode) structs.CheckServiceNode {
	node := n.Node
	checkServiceNode := structs.CheckServiceNode{
		Node: &structs.Node{
			ID:              node.ID,
			Node:            node.Node,
			Address:         node.Address,
			Datacenter:      node.Datacenter,
			TaggedAddresses: node.TaggedAddresses,
			Meta:            node.Meta,
			RaftIndex:       FromRaftIndex(node.RaftIndex),
		},
	}

	if n.Service != nil {
		service := n.Service
		checkServiceNode.Service = &structs.NodeService{
			Kind:                       service.Kind,
			ID:                         service.ID,
			Service:                    service.Service,
			Tags:                       service.Tags,
			Address:                    service.Address,
			TaggedAddresses:            FromTaggedAddresses(service.TaggedAddresses),
			Meta:                       service.Meta,
			Port:                       service.Port,
			Weights:                    FromWeights(service.Weights),
			EnableTagOverride:          service.EnableTagOverride,
			ProxyDestination:           service.ProxyDestination,
			LocallyRegisteredAsSidecar: service.LocallyRegisteredAsSidecar,
			RaftIndex:                  FromRaftIndex(service.RaftIndex),
		}
		connect := FromServiceConnect(&service.Connect)
		if connect != nil {
			checkServiceNode.Service.Connect = *connect
		}
		proxy := FromConnectProxyConfig(&service.Proxy)
		if proxy != nil {
			checkServiceNode.Service.Proxy = *proxy
		}
	}
	for _, check := range n.Checks {
		checkServiceNode.Checks = append(checkServiceNode.Checks, FromHealthCheck(check))
	}

	return checkServiceNode
}

func FromTaggedAddresses(t map[string]*ServiceAddress) map[string]structs.ServiceAddress {
	if t == nil {
		return nil
	}

	out := make(map[string]structs.ServiceAddress, len(t))
	for k, v := range t {
		out[k] = structs.ServiceAddress{Address: v.Address, Port: v.Port}
	}
	return out
}

func FromRaftIndex(r RaftIndex) structs.RaftIndex {
	return structs.RaftIndex{
		CreateIndex: r.CreateIndex,
		ModifyIndex: r.ModifyIndex,
	}
}

func FromServiceDefinitionConnectProxy(p *ServiceDefinitionConnectProxy) *structs.ServiceDefinitionConnectProxy {
	if p == nil {
		return nil
	}

	return &structs.ServiceDefinitionConnectProxy{
		Command:   p.Command,
		ExecMode:  p.ExecMode,
		Config:    p.Config,
		Upstreams: FromUpstreams(p.Upstreams),
	}
}

func FromConnectProxyConfig(c *ConnectProxyConfig) *structs.ConnectProxyConfig {
	if c == nil {
		return nil
	}

	return &structs.ConnectProxyConfig{
		DestinationServiceName: c.DestinationServiceName,
		DestinationServiceID:   c.DestinationServiceID,
		LocalServiceAddress:    c.LocalServiceAddress,
		LocalServicePort:       c.LocalServicePort,
		Config:                 c.Config,
		Upstreams:              FromUpstreams(c.Upstreams),
		MeshGateway:            FromMeshGatewayConfig(c.MeshGateway),
	}
}

func FromUpstreams(other []Upstream) structs.Upstreams {
	var upstreams structs.Upstreams
	for _, u := range other {
		upstreams = append(upstreams, structs.Upstream{
			DestinationType:      u.DestinationType,
			DestinationNamespace: u.DestinationNamespace,
			DestinationName:      u.DestinationName,
			Datacenter:           u.Datacenter,
			LocalBindAddress:     u.LocalBindAddress,
			LocalBindPort:        u.LocalBindPort,
			Config:               u.Config,
			MeshGateway:          FromMeshGatewayConfig(u.MeshGateway),
		})
	}
	return upstreams
}

func FromServiceConnect(s *ServiceConnect) *structs.ServiceConnect {
	if s == nil {
		return nil
	}

	return &structs.ServiceConnect{
		Native:         s.Native,
		Proxy:          FromServiceDefinitionConnectProxy(s.Proxy),
		SidecarService: FromServiceDefinition(s.SidecarService),
	}
}

func FromServiceDefinition(s *ServiceDefinition) *structs.ServiceDefinition {
	if s == nil {
		return nil
	}

	definition := &structs.ServiceDefinition{
		Kind:              s.Kind,
		ID:                s.ID,
		Name:              s.Name,
		Tags:              s.Tags,
		Address:           s.Address,
		TaggedAddresses:   FromTaggedAddresses(s.TaggedAddresses),
		Meta:              s.Meta,
		Port:              s.Port,
		Weights:           FromWeights(s.Weights),
		Token:             s.Token,
		EnableTagOverride: s.EnableTagOverride,
		ProxyDestination:  s.ProxyDestination,
		Proxy:             FromConnectProxyConfig(s.Proxy),
		Connect:           FromServiceConnect(s.Connect),
	}

	check := FromCheckType(&s.Check)
	if check != nil {
		definition.Check = *check
	}
	var checks []*structs.CheckType
	for _, c := range s.Checks {
		checks = append(checks, FromCheckType(c))
	}
	definition.Checks = checks

	return definition
}

func FromWeights(w *Weights) *structs.Weights {
	if w == nil {
		return nil
	}

	return &structs.Weights{
		Passing: w.Passing,
		Warning: w.Warning,
	}
}

func FromCheckType(c *CheckType) *structs.CheckType {
	if c == nil {
		return nil
	}

	return &structs.CheckType{
		CheckID:                        c.CheckID,
		Name:                           c.Name,
		Status:                         c.Status,
		Notes:                          c.Notes,
		ScriptArgs:                     c.ScriptArgs,
		HTTP:                           c.HTTP,
		Header:                         c.Header,
		Method:                         c.Method,
		TCP:                            c.TCP,
		Interval:                       c.Interval,
		AliasNode:                      c.AliasNode,
		AliasService:                   c.AliasService,
		DockerContainerID:              c.DockerContainerID,
		Shell:                          c.Shell,
		GRPC:                           c.GRPC,
		GRPCUseTLS:                     c.GRPCUseTLS,
		TLSSkipVerify:                  c.TLSSkipVerify,
		Timeout:                        c.Timeout,
		TTL:                            c.TTL,
		DeregisterCriticalServiceAfter: c.DeregisterCriticalServiceAfter,
	}
}

func FromHealthCheck(h *HealthCheck) *structs.HealthCheck {
	if h == nil {
		return nil
	}
	return &structs.HealthCheck{
		Node:        h.Node,
		CheckID:     h.CheckID,
		Name:        h.Name,
		Status:      h.Status,
		Notes:       h.Notes,
		Output:      h.Output,
		ServiceID:   h.ServiceID,
		ServiceName: h.ServiceName,
		ServiceTags: h.ServiceTags,
		Definition: structs.HealthCheckDefinition{
			HTTP:                           h.Definition.HTTP,
			TLSSkipVerify:                  h.Definition.TLSSkipVerify,
			Header:                         h.Definition.Header,
			Method:                         h.Definition.Method,
			TCP:                            h.Definition.TCP,
			Interval:                       h.Definition.Interval,
			Timeout:                        h.Definition.Timeout,
			DeregisterCriticalServiceAfter: h.Definition.DeregisterCriticalServiceAfter,
		},
		RaftIndex: FromRaftIndex(h.RaftIndex),
	}
}

func FromMeshGatewayConfig(c *MeshGatewayConfig) structs.MeshGatewayConfig {
	if c == nil {
		return structs.MeshGatewayConfig{}
	}

	return structs.MeshGatewayConfig{Mode: c.Mode}
}
