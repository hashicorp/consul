package stream

import (
	fmt "fmt"
	"reflect"

	"github.com/gogo/protobuf/types"
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
			TaggedAddresses:            ToTaggedAddresses(service.TaggedAddresses),
			Meta:                       service.Meta,
			Port:                       service.Port,
			Weights:                    ToWeights(service.Weights),
			EnableTagOverride:          service.EnableTagOverride,
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

func ToTaggedAddresses(t map[string]structs.ServiceAddress) map[string]*ServiceAddress {
	if t == nil {
		return nil
	}

	out := make(map[string]*ServiceAddress, len(t))
	for k, v := range t {
		out[k] = &ServiceAddress{Address: v.Address, Port: v.Port}
	}
	return out
}

func ToRaftIndex(r structs.RaftIndex) RaftIndex {
	return RaftIndex{
		CreateIndex: r.CreateIndex,
		ModifyIndex: r.ModifyIndex,
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
		Config:                 MapToPBStruct(c.Config),
		Upstreams:              ToUpstreams(c.Upstreams),
		MeshGateway:            ToMeshGatewayConfig(c.MeshGateway),
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
			Config:               MapToPBStruct(u.Config),
			MeshGateway:          ToMeshGatewayConfig(u.MeshGateway),
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
		TaggedAddresses:   ToTaggedAddresses(s.TaggedAddresses),
		Meta:              s.Meta,
		Port:              s.Port,
		Weights:           ToWeights(s.Weights),
		Token:             s.Token,
		EnableTagOverride: s.EnableTagOverride,
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

func ToMeshGatewayConfig(c structs.MeshGatewayConfig) *MeshGatewayConfig {
	if reflect.DeepEqual(c, structs.MeshGatewayConfig{}) {
		return nil
	}

	return &MeshGatewayConfig{Mode: c.Mode}
}

func MapToPBStruct(m map[string]interface{}) *types.Struct {
	if len(m) == 0 {
		return nil
	}

	fields := make(map[string]*types.Value, len(m))

	for k, v := range m {
		fields[k] = interfaceToPBValue(v)
	}

	return &types.Struct{Fields: fields}
}

func SliceToPBListValue(s []interface{}) *types.ListValue {
	if len(s) == 0 {
		return nil
	}

	vals := make([]*types.Value, len(s))

	for i, v := range s {
		vals[i] = interfaceToPBValue(v)
	}

	return &types.ListValue{Values: vals}
}

func interfaceToPBValue(v interface{}) *types.Value {
	switch v := v.(type) {
	case nil:
		return nil
	case int:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int8:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int32:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case int64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint8:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint32:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case uint64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case float32:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v),
			},
		}
	case float64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: v,
			},
		}
	case string:
		return &types.Value{
			Kind: &types.Value_StringValue{
				StringValue: v,
			},
		}
	case error:
		return &types.Value{
			Kind: &types.Value_StringValue{
				StringValue: v.Error(),
			},
		}
	case map[string]interface{}:
		return &types.Value{
			Kind: &types.Value_StructValue{
				StructValue: MapToPBStruct(v),
			},
		}
	case []interface{}:
		return &types.Value{
			Kind: &types.Value_ListValue{
				ListValue: SliceToPBListValue(v),
			},
		}
	default:
		return interfaceToPBValueReflect(reflect.ValueOf(v))
	}
}

func interfaceToPBValueReflect(v reflect.Value) *types.Value {
	switch v.Kind() {
	case reflect.Interface:
		return interfaceToPBValue(v.Interface())
	case reflect.Bool:
		return &types.Value{
			Kind: &types.Value_BoolValue{
				BoolValue: v.Bool(),
			},
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v.Int()),
			},
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(v.Uint()),
			},
		}
	case reflect.Float32, reflect.Float64:
		return &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: v.Float(),
			},
		}
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return interfaceToPBValueReflect(reflect.Indirect(v))
	case reflect.Array, reflect.Slice:
		size := v.Len()
		if size == 0 {
			return nil
		}
		values := make([]*types.Value, size)
		for i := 0; i < size; i++ {
			values[i] = interfaceToPBValue(v.Index(i))
		}
		return &types.Value{
			Kind: &types.Value_ListValue{
				ListValue: &types.ListValue{
					Values: values,
				},
			},
		}
	case reflect.Struct:
		t := v.Type()
		size := v.NumField()
		if size == 0 {
			return nil
		}
		fields := make(map[string]*types.Value, size)
		for i := 0; i < size; i++ {
			name := t.Field(i).Name
			// Only include public fields. There may be a better way with struct tags
			// but this works for now.
			if len(name) > 0 && 'A' <= name[0] && name[0] <= 'Z' {
				fields[name] = interfaceToPBValue(v.Field(i))
			}
		}
		if len(fields) == 0 {
			return nil
		}
		return &types.Value{
			Kind: &types.Value_StructValue{
				StructValue: &types.Struct{
					Fields: fields,
				},
			},
		}
	case reflect.Map:
		keys := v.MapKeys()
		if len(keys) == 0 {
			return nil
		}
		fields := make(map[string]*types.Value, len(keys))
		for _, k := range keys {
			if k.Kind() == reflect.String {
				fields[k.String()] = interfaceToPBValue(v.MapIndex(k))
			}
		}
		if len(fields) == 0 {
			return nil
		}
		return &types.Value{
			Kind: &types.Value_StructValue{
				StructValue: &types.Struct{
					Fields: fields,
				},
			},
		}
	default:
		// Last resort
		return &types.Value{
			Kind: &types.Value_StringValue{
				StringValue: fmt.Sprint(v),
			},
		}
	}
}
