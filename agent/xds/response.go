package xds

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
)

func createResponse(typeURL string, version, nonce string, resources []proto.Message) (*envoy_discovery_v3.DiscoveryResponse, error) {
	anys := make([]*any.Any, 0, len(resources))
	for _, r := range resources {
		if r == nil {
			continue
		}
		if any, ok := r.(*any.Any); ok {
			anys = append(anys, any)
			continue
		}
		data, err := proto.Marshal(r)
		if err != nil {
			return nil, err
		}
		anys = append(anys, &any.Any{
			TypeUrl: typeURL,
			Value:   data,
		})
	}
	resp := &envoy_discovery_v3.DiscoveryResponse{
		VersionInfo: version,
		Resources:   anys,
		TypeUrl:     typeURL,
		Nonce:       nonce,
	}
	return resp, nil
}

func makePipeAddress(path string, mode uint32) *envoy_core_v3.Address {
	return &envoy_core_v3.Address{
		Address: &envoy_core_v3.Address_Pipe{
			Pipe: &envoy_core_v3.Pipe{
				Path: path,
				Mode: mode,
			},
		},
	}
}

func makeAddress(ip string, port int) *envoy_core_v3.Address {
	return &envoy_core_v3.Address{
		Address: &envoy_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_core_v3.SocketAddress{
				Address: ip,
				PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

func makeUint32Value(n int) *wrappers.UInt32Value {
	return &wrappers.UInt32Value{Value: uint32(n)}
}

func makeBoolValue(n bool) *wrappers.BoolValue {
	return &wrappers.BoolValue{Value: n}
}

func makeEnvoyRegexMatch(patt string) *envoy_matcher_v3.RegexMatcher {
	return &envoy_matcher_v3.RegexMatcher{
		EngineType: &envoy_matcher_v3.RegexMatcher_GoogleRe2{
			GoogleRe2: &envoy_matcher_v3.RegexMatcher_GoogleRE2{},
		},
		Regex: patt,
	}
}
