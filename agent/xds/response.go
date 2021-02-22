package xds

import (
	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoymatcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
)

func createResponse(typeURL string, version, nonce string, resources []proto.Message) (*envoy.DiscoveryResponse, error) {
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
	resp := &envoy.DiscoveryResponse{
		VersionInfo: version,
		Resources:   anys,
		TypeUrl:     typeURL,
		Nonce:       nonce,
	}
	return resp, nil
}

func makeAddress(ip string, port int) *envoycore.Address {
	return &envoycore.Address{
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

func makeUint32Value(n int) *wrappers.UInt32Value {
	return &wrappers.UInt32Value{Value: uint32(n)}
}

func makeBoolValue(n bool) *wrappers.BoolValue {
	return &wrappers.BoolValue{Value: n}
}

func makeEnvoyRegexMatch(patt string) *envoymatcher.RegexMatcher {
	return &envoymatcher.RegexMatcher{
		EngineType: &envoymatcher.RegexMatcher_GoogleRe2{
			GoogleRe2: &envoymatcher.RegexMatcher_GoogleRE2{},
		},
		Regex: patt,
	}
}
