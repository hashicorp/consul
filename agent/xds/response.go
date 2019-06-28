package xds

import (
	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	prototypes "github.com/gogo/protobuf/types"
)

func createResponse(typeURL string, version, nonce string, resources []proto.Message) (*envoy.DiscoveryResponse, error) {
	anys := make([]types.Any, 0, len(resources))
	for _, r := range resources {
		if r == nil {
			continue
		}
		if any, ok := r.(*types.Any); ok {
			anys = append(anys, *any)
			continue
		}
		data, err := proto.Marshal(r)
		if err != nil {
			return nil, err
		}
		anys = append(anys, types.Any{
			TypeUrl: typeURL,
			Value:   data,
		})
	}
	resp := &envoy.DiscoveryResponse{
		VersionInfo: version,
		Resources:   anys,
		TypeUrl:     typeURL,
		Nonce:       nonce,
	}
	return resp, nil
}

func makeAddress(ip string, port int) envoycore.Address {
	return envoycore.Address{
		Address: &envoycore.Address_SocketAddress{
			SocketAddress: &envoycore.SocketAddress{
				Address: ip,
				PortSpecifier: &envoycore.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

func makeAddressPtr(ip string, port int) *envoycore.Address {
	a := makeAddress(ip, port)
	return &a
}

func makeUint32Value(n int) *prototypes.UInt32Value {
	return &prototypes.UInt32Value{Value: uint32(n)}
}

func makeBoolValue(n bool) *prototypes.BoolValue {
	return &prototypes.BoolValue{Value: n}
}
