package xds

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
)

func createResponse(typeURL string, version, nonce string, resources []proto.Message) (*v2.DiscoveryResponse, error) {
	anys := make([]types.Any, len(resources))
	for i, r := range resources {
		if r == nil {
			continue
		}
		if any, ok := r.(*types.Any); ok {
			anys[i] = *any
			continue
		}
		data, err := proto.Marshal(r)
		if err != nil {
			return nil, err
		}
		anys[i] = types.Any{
			TypeUrl: typeURL,
			Value:   data,
		}
	}
	resp := &v2.DiscoveryResponse{
		VersionInfo: version,
		Resources:   anys,
		TypeUrl:     typeURL,
		Nonce:       nonce,
	}
	return resp, nil
}

func makeAddress(ip string, port int) core.Address {
	return core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Address: ip,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

func makeAddressPtr(ip string, port int) *core.Address {
	a := makeAddress(ip, port)
	return &a
}
