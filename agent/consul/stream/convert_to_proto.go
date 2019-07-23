package stream

import (
	"github.com/hashicorp/consul/agent/structs"
)

func ToCheckServiceNode(n *structs.CheckServiceNode) *CheckServiceNode {
	node := n.Node
	checkServiceNode := &CheckServiceNode{
		Node: &Node{
			ID:              node.ID,
			Node:            node.Node,
			Address:         node.Address,
			Datacenter:      node.Datacenter,
			TaggedAddresses: node.TaggedAddresses,
			Meta:            node.Meta,
			RaftIndex:       ToRaftIndex(node.RaftIndex),
		},
	}

	if n.Service != nil {
		service := n.Service
		checkServiceNode.Service = &NodeService{
			Kind:                       service.Kind,
			ID:                         service.ID,
			Service:                    service.Service,
			Tags:                       service.Tags,
			Address:                    service.Address,
			Meta:                       service.Meta,
			Port:                       service.Port,
			Weights:                    ToWeights(service.Weights),
			EnableTagOverride:          service.EnableTagOverride,
			ProxyDestination:           service.ProxyDestination,
			LocallyRegisteredAsSidecar: service.LocallyRegisteredAsSidecar,
			RaftIndex:                  ToRaftIndex(service.RaftIndex),
		}
		connect := ToServiceConnect(&service.Connect)
		if connect != nil {
			checkServiceNode.Service.Connect = *connect
		}
		proxy := ToConnectProxyConfig(&service.Proxy)
		if proxy != nil {
			checkServiceNode.Service.Proxy = *proxy
		}
	}
	for _, check := range n.Checks {
		checkServiceNode.Checks = append(checkServiceNode.Checks, ToHealthCheck(check))
	}

	return checkServiceNode
}

func ToRaftIndex(r structs.RaftIndex) RaftIndex {
	return RaftIndex{
		CreateIndex: r.CreateIndex,
		ModifyIndex: r.ModifyIndex,
	}
}

func ToServiceDefinitionConnectProxy(p *structs.ServiceDefinitionConnectProxy) *ServiceDefinitionConnectProxy {
	if p == nil {
		return nil
	}

	return &ServiceDefinitionConnectProxy{
		Command:   p.Command,
		ExecMode:  p.ExecMode,
		Config:    p.Config,
		Upstreams: ToUpstreams(p.Upstreams),
	}
}

func ToConnectProxyConfig(c *structs.ConnectProxyConfig) *ConnectProxyConfig {
	if c == nil {
		return nil
	}

	return &ConnectProxyConfig{
		DestinationServiceName: c.DestinationServiceName,
		DestinationServiceID:   c.DestinationServiceID,
		LocalServiceAddress:    c.LocalServiceAddress,
		LocalServicePort:       c.LocalServicePort,
		Config:                 c.Config,
		Upstreams:              ToUpstreams(c.Upstreams),
	}
}

func ToUpstreams(other structs.Upstreams) []Upstream {
	var upstreams []Upstream
	for _, u := range other {
		upstreams = append(upstreams, Upstream{
			DestinationType:      u.DestinationType,
			DestinationNamespace: u.DestinationNamespace,
			DestinationName:      u.DestinationName,
			Datacenter:           u.Datacenter,
			LocalBindAddress:     u.LocalBindAddress,
			LocalBindPort:        u.LocalBindPort,
			Config:               u.Config,
		})
	}
	return upstreams
}

func ToServiceConnect(s *structs.ServiceConnect) *ServiceConnect {
	if s == nil {
		return nil
	}

	return &ServiceConnect{
		Native:         s.Native,
		Proxy:          ToServiceDefinitionConnectProxy(s.Proxy),
		SidecarService: ToServiceDefinition(s.SidecarService),
	}
}

func ToServiceDefinition(s *structs.ServiceDefinition) *ServiceDefinition {
	if s == nil {
		return nil
	}

	definition := &ServiceDefinition{
		Kind:              s.Kind,
		ID:                s.ID,
		Name:              s.Name,
		Tags:              s.Tags,
		Address:           s.Address,
		Meta:              s.Meta,
		Port:              s.Port,
		Weights:           ToWeights(s.Weights),
		Token:             s.Token,
		EnableTagOverride: s.EnableTagOverride,
		ProxyDestination:  s.ProxyDestination,
		Proxy:             ToConnectProxyConfig(s.Proxy),
		Connect:           ToServiceConnect(s.Connect),
	}

	check := ToCheckType(&s.Check)
	if check != nil {
		definition.Check = *check
	}
	var checks []*CheckType
	for _, c := range s.Checks {
		checks = append(checks, ToCheckType(c))
	}
	definition.Checks = checks

	return definition
}

func ToWeights(w *structs.Weights) *Weights {
	if w == nil {
		return nil
	}

	return &Weights{
		Passing: w.Passing,
		Warning: w.Warning,
	}
}

func ToCheckType(c *structs.CheckType) *CheckType {
	if c == nil {
		return nil
	}

	return &CheckType{
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

func ToHealthCheck(h *structs.HealthCheck) *HealthCheck {
	if h == nil {
		return nil
	}
	return &HealthCheck{
		Node:        h.Node,
		CheckID:     h.CheckID,
		Name:        h.Name,
		Status:      h.Status,
		Notes:       h.Notes,
		Output:      h.Output,
		ServiceID:   h.ServiceID,
		ServiceName: h.ServiceName,
		ServiceTags: h.ServiceTags,
		Definition: HealthCheckDefinition{
			HTTP:                           h.Definition.HTTP,
			TLSSkipVerify:                  h.Definition.TLSSkipVerify,
			Header:                         h.Definition.Header,
			Method:                         h.Definition.Method,
			TCP:                            h.Definition.TCP,
			Interval:                       h.Definition.Interval,
			Timeout:                        h.Definition.Timeout,
			DeregisterCriticalServiceAfter: h.Definition.DeregisterCriticalServiceAfter,
		},
		RaftIndex: ToRaftIndex(h.RaftIndex),
	}
}
