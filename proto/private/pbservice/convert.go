// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pbservice

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/types"
)

type CheckIDType = types.CheckID
type NodeIDType = types.NodeID

// Function variables to support proto generation
// This allows for using functions in the local package without having to generate imports
var EnvoyExtensionsToStructs = pbcommon.EnvoyExtensionsToStructs
var EnvoyExtensionsFromStructs = pbcommon.EnvoyExtensionsFromStructs
var ProtobufTypesStructToMapStringInterface = pbcommon.ProtobufTypesStructToMapStringInterface
var MapStringInterfaceToProtobufTypesStruct = pbcommon.MapStringInterfaceToProtobufTypesStruct

func RaftIndexToStructs(s *pbcommon.RaftIndex) structs.RaftIndex {
	if s == nil {
		return structs.RaftIndex{}
	}
	return structs.RaftIndex{
		CreateIndex: s.CreateIndex,
		ModifyIndex: s.ModifyIndex,
	}
}

func NewRaftIndexFromStructs(s structs.RaftIndex) *pbcommon.RaftIndex {
	return &pbcommon.RaftIndex{
		CreateIndex: s.CreateIndex,
		ModifyIndex: s.ModifyIndex,
	}
}

func MapHeadersToStructs(s map[string]*HeaderValue) map[string][]string {
	t := make(map[string][]string, len(s))
	for k, v := range s {
		t[k] = v.Value
	}
	return t
}

func NewMapHeadersFromStructs(t map[string][]string) map[string]*HeaderValue {
	s := make(map[string]*HeaderValue, len(t))
	for k, v := range t {
		s[k] = &HeaderValue{Value: v}
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
		n := new(structs.Node)
		NodeToStructs(s.Node, n)
		t.Node = n
	}
	if s.Service != nil {
		r := new(structs.NodeService)
		NodeServiceToStructs(s.Service, r)
		t.Service = r
	}
	t.Checks = make(structs.HealthChecks, len(s.Checks))
	for i, c := range s.Checks {
		if c == nil {
			continue
		}
		h := new(structs.HealthCheck)
		HealthCheckToStructs(c, h)
		t.Checks[i] = h
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
		n := new(Node)
		NodeFromStructs(t.Node, n)
		s.Node = n
	}
	if t.Service != nil {
		r := new(NodeService)
		NodeServiceFromStructs(t.Service, r)
		s.Service = r
	}
	s.Checks = make([]*HealthCheck, len(t.Checks))
	for i, c := range t.Checks {
		if c == nil {
			continue
		}
		h := new(HealthCheck)
		HealthCheckFromStructs(c, h)
		s.Checks[i] = h
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
func MapStringServiceAddressToStructs(s map[string]*ServiceAddress) map[string]structs.ServiceAddress {
	t := make(map[string]structs.ServiceAddress, len(s))
	for k, v := range s {
		t[k] = structs.ServiceAddress{Address: v.Address, Port: int(v.Port)}
	}
	return t
}

// TODO: handle this with mog
func NewMapStringServiceAddressFromStructs(t map[string]structs.ServiceAddress) map[string]*ServiceAddress {
	s := make(map[string]*ServiceAddress, len(t))
	for k, v := range t {
		s[k] = &ServiceAddress{Address: v.Address, Port: int32(v.Port)}
	}
	return s
}

// TODO: handle this with mog
func ExposePathSliceToStructs(s []*ExposePath) []structs.ExposePath {
	t := make([]structs.ExposePath, len(s))
	for i, v := range s {
		e := new(structs.ExposePath)
		ExposePathToStructs(v, e)
		t[i] = *e
	}
	return t
}

// TODO: handle this with mog
func NewExposePathSliceFromStructs(t []structs.ExposePath) []*ExposePath {
	s := make([]*ExposePath, len(t))
	for i, v := range t {
		ep := new(ExposePath)
		ExposePathFromStructs(&v, ep)
		s[i] = ep
	}
	return s
}

// TODO: handle this with mog
func UpstreamsToStructs(s []*Upstream) structs.Upstreams {
	t := make(structs.Upstreams, len(s))
	for i, v := range s {
		u := new(structs.Upstream)
		UpstreamToStructs(v, u)
		t[i] = *u
	}
	return t
}

// TODO: handle this with mog
func NewUpstreamsFromStructs(t structs.Upstreams) []*Upstream {
	s := make([]*Upstream, len(t))
	for i, v := range t {
		u := new(Upstream)
		UpstreamFromStructs(&v, u)
		s[i] = u
	}
	return s
}

// TODO: handle this with mog
func CheckTypesToStructs(s []*CheckType) structs.CheckTypes {
	t := make(structs.CheckTypes, len(s))
	for i, v := range s {
		if v == nil {
			continue
		}
		c := new(structs.CheckType)
		CheckTypeToStructs(v, c)
		t[i] = c
	}
	return t
}

// TODO: handle this with mog
func NewCheckTypesFromStructs(t structs.CheckTypes) []*CheckType {
	s := make([]*CheckType, len(t))
	for i, v := range t {
		if v == nil {
			continue
		}
		newV := new(CheckType)
		CheckTypeFromStructs(v, newV)
		s[i] = newV
	}
	return s
}

// TODO: handle this with mog
func ConnectProxyConfigPtrToStructs(s *ConnectProxyConfig) *structs.ConnectProxyConfig {
	if s == nil {
		return nil
	}
	c := new(structs.ConnectProxyConfig)
	ConnectProxyConfigToStructs(s, c)
	return c
}

// TODO: handle this with mog
func NewConnectProxyConfigPtrFromStructs(t *structs.ConnectProxyConfig) *ConnectProxyConfig {
	if t == nil {
		return nil
	}
	cp := new(ConnectProxyConfig)
	ConnectProxyConfigFromStructs(t, cp)
	return cp
}

// TODO: handle this with mog
func ServiceConnectPtrToStructs(s *ServiceConnect) *structs.ServiceConnect {
	if s == nil {
		return nil
	}
	sc := new(structs.ServiceConnect)
	ServiceConnectToStructs(s, sc)
	return sc
}

// TODO: handle this with mog
func NewServiceConnectPtrFromStructs(t *structs.ServiceConnect) *ServiceConnect {
	if t == nil {
		return nil
	}
	sc := new(ServiceConnect)
	ServiceConnectFromStructs(t, sc)
	return sc
}

// TODO: handle this with mog
func ServiceDefinitionPtrToStructs(s *ServiceDefinition) *structs.ServiceDefinition {
	if s == nil {
		return nil
	}
	sd := new(structs.ServiceDefinition)
	ServiceDefinitionToStructs(s, sd)
	return sd
}

// TODO: handle this with mog
func NewServiceDefinitionPtrFromStructs(t *structs.ServiceDefinition) *ServiceDefinition {
	if t == nil {
		return nil
	}
	sd := new(ServiceDefinition)
	ServiceDefinitionFromStructs(t, sd)
	return sd
}

// TODO: handle this with mog
func LocalityToStructs(l *pbcommon.Locality) *structs.Locality {
	if l == nil {
		return nil
	}
	return &structs.Locality{
		Region: l.Region,
		Zone:   l.Zone,
	}
}

// TODO: handle this with mog
func LocalityFromStructs(l *structs.Locality) *pbcommon.Locality {
	if l == nil {
		return nil
	}
	return &pbcommon.Locality{
		Region: l.Region,
		Zone:   l.Zone,
	}
}
