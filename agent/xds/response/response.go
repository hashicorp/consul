// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package response

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func CreateResponse(typeURL string, version, nonce string, resources []proto.Message) (*envoy_discovery_v3.DiscoveryResponse, error) {
	anys := make([]*anypb.Any, 0, len(resources))
	for _, r := range resources {
		if r == nil {
			continue
		}
		if any, ok := r.(*anypb.Any); ok {
			anys = append(anys, any)
			continue
		}
		data, err := proto.Marshal(r)
		if err != nil {
			return nil, err
		}
		anys = append(anys, &anypb.Any{
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

func MakePipeAddress(path string, mode uint32) *envoy_core_v3.Address {
	return &envoy_core_v3.Address{
		Address: &envoy_core_v3.Address_Pipe{
			Pipe: &envoy_core_v3.Pipe{
				Path: path,
				Mode: mode,
			},
		},
	}
}

func MakeAddress(ip string, port int) *envoy_core_v3.Address {
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

func MakeUint32Value(n int) *wrapperspb.UInt32Value {
	return &wrapperspb.UInt32Value{Value: uint32(n)}
}

func MakeBoolValue(n bool) *wrapperspb.BoolValue {
	return &wrapperspb.BoolValue{Value: n}
}

func MakeEnvoyRegexMatch(patt string) *envoy_matcher_v3.RegexMatcher {
	return &envoy_matcher_v3.RegexMatcher{
		Regex: patt,
	}
}
