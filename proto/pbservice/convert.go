package pbservice

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommongogo"
)

func RaftIndexToStructs(s pbcommongogo.RaftIndex) structs.RaftIndex {
	return structs.RaftIndex{
		CreateIndex: s.CreateIndex,
		ModifyIndex: s.ModifyIndex,
	}
}

func NewRaftIndexFromStructs(s structs.RaftIndex) pbcommongogo.RaftIndex {
	return pbcommongogo.RaftIndex{
		CreateIndex: s.CreateIndex,
		ModifyIndex: s.ModifyIndex,
	}
}

func MapHeadersToStructs(s map[string]HeaderValue) map[string][]string {
	t := make(map[string][]string, len(s))
	for k, v := range s {
		t[k] = v.Value
	}
	return t
}

func NewMapHeadersFromStructs(t map[string][]string) map[string]HeaderValue {
	s := make(map[string]HeaderValue, len(t))
	for k, v := range t {
		s[k] = HeaderValue{Value: v}
	}
	return s
}

// TODO: use mog once it supports pointers and slices
func CheckServiceNodeToStructs(s *CheckServiceNode) (*structs.CheckServiceNode, error) {
	if s == nil {
		return nil, nil
	}
	var t structs.CheckServiceNode
	if s.Node != nil {
		n := NodeToStructs(*s.Node)
		t.Node = &n
	}
	if s.Service != nil {
		r := NodeServiceToStructs(*s.Service)
		t.Service = &r
	}
	t.Checks = make(structs.HealthChecks, len(s.Checks))
	for i, c := range s.Checks {
		if c == nil {
			continue
		}
		h, err := HealthCheckToStructs(*c)
		if err != nil {
			return &t, err
		}
		t.Checks[i] = &h
	}
	return &t, nil
}

// TODO: use mog once it supports pointers and slices
func NewCheckServiceNodeFromStructs(t *structs.CheckServiceNode) *CheckServiceNode {
	if t == nil {
		return nil
	}
	var s CheckServiceNode
	if t.Node != nil {
		n := NewNodeFromStructs(*t.Node)
		s.Node = &n
	}
	if t.Service != nil {
		r := NewNodeServiceFromStructs(*t.Service)
		s.Service = &r
	}
	s.Checks = make([]*HealthCheck, len(t.Checks))
	for i, c := range t.Checks {
		if c == nil {
			continue
		}
		h := NewHealthCheckFromStructs(*c)
		s.Checks[i] = &h
	}
	return &s
}

// TODO: handle this with mog, once mog handles pointers
func WeightsPtrToStructs(s *Weights) *structs.Weights {
	if s == nil {
		return nil
	}
	var t structs.Weights
	t.Passing = int(s.Passing)
	t.Warning = int(s.Warning)
	return &t
}

// TODO: handle this with mog, once mog handles pointers
func NewWeightsPtrFromStructs(t *structs.Weights) *Weights {
	if t == nil {
		return nil
	}
	var s Weights
	s.Passing = int32(t.Passing)
	s.Warning = int32(t.Warning)
	return &s
}

// TODO: handle this with mog
func MapStringServiceAddressToStructs(s map[string]ServiceAddress) map[string]structs.ServiceAddress {
	t := make(map[string]structs.ServiceAddress, len(s))
	for k, v := range s {
		t[k] = structs.ServiceAddress{Address: v.Address, Port: int(v.Port)}
	}
	return t
}

// TODO: handle this with mog
func NewMapStringServiceAddressFromStructs(t map[string]structs.ServiceAddress) map[string]ServiceAddress {
	s := make(map[string]ServiceAddress, len(t))
	for k, v := range t {
		s[k] = ServiceAddress{Address: v.Address, Port: int32(v.Port)}
	}
	return s
}

// TODO: handle this with mog
func ExposePathSliceToStructs(s []ExposePath) []structs.ExposePath {
	t := make([]structs.ExposePath, len(s))
	for i, v := range s {
		t[i] = ExposePathToStructs(v)
	}
	return t
}

// TODO: handle this with mog
func NewExposePathSliceFromStructs(t []structs.ExposePath) []ExposePath {
	s := make([]ExposePath, len(t))
	for i, v := range t {
		s[i] = NewExposePathFromStructs(v)
	}
	return s
}

// TODO: handle this with mog
func UpstreamsToStructs(s []Upstream) structs.Upstreams {
	t := make(structs.Upstreams, len(s))
	for i, v := range s {
		t[i] = UpstreamToStructs(v)
	}
	return t
}

// TODO: handle this with mog
func NewUpstreamsFromStructs(t structs.Upstreams) []Upstream {
	s := make([]Upstream, len(t))
	for i, v := range t {
		s[i] = NewUpstreamFromStructs(v)
	}
	return s
}

// TODO: handle this with mog
func CheckTypesToStructs(s []*CheckType) (structs.CheckTypes, error) {
	t := make(structs.CheckTypes, len(s))
	for i, v := range s {
		if v == nil {
			continue
		}
		newV, err := CheckTypeToStructs(*v)
		if err != nil {
			return t, err
		}
		t[i] = &newV
	}
	return t, nil
}

// TODO: handle this with mog
func NewCheckTypesFromStructs(t structs.CheckTypes) []*CheckType {
	s := make([]*CheckType, len(t))
	for i, v := range t {
		if v == nil {
			continue
		}
		newV := NewCheckTypeFromStructs(*v)
		s[i] = &newV
	}
	return s
}

// TODO: handle this with mog
func ConnectProxyConfigPtrToStructs(s *ConnectProxyConfig) *structs.ConnectProxyConfig {
	if s == nil {
		return nil
	}
	t := ConnectProxyConfigToStructs(*s)
	return &t
}

// TODO: handle this with mog
func NewConnectProxyConfigPtrFromStructs(t *structs.ConnectProxyConfig) *ConnectProxyConfig {
	if t == nil {
		return nil
	}
	s := NewConnectProxyConfigFromStructs(*t)
	return &s
}

// TODO: handle this with mog
func ServiceConnectPtrToStructs(s *ServiceConnect) *structs.ServiceConnect {
	if s == nil {
		return nil
	}
	t := ServiceConnectToStructs(*s)
	return &t
}

// TODO: handle this with mog
func NewServiceConnectPtrFromStructs(t *structs.ServiceConnect) *ServiceConnect {
	if t == nil {
		return nil
	}
	s := NewServiceConnectFromStructs(*t)
	return &s
}

// TODO: handle this with mog
func ServiceDefinitionPtrToStructs(s *ServiceDefinition) *structs.ServiceDefinition {
	if s == nil {
		return nil
	}
	t, err := ServiceDefinitionToStructs(*s)
	if err != nil {
		return nil
	}
	return &t
}

// TODO: handle this with mog
func NewServiceDefinitionPtrFromStructs(t *structs.ServiceDefinition) *ServiceDefinition {
	if t == nil {
		return nil
	}
	s := NewServiceDefinitionFromStructs(*t)
	return &s
}
