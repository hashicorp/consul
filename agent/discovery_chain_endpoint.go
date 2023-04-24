package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/decode"
)

func (s *HTTPHandlers) DiscoveryChainRead(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DiscoveryChainRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	args.Name = strings.TrimPrefix(req.URL.Path, "/v1/discovery-chain/")
	if args.Name == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing chain name"}
	}

	args.EvaluateInDatacenter = req.URL.Query().Get("compile-dc")
	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}
	args.WithEnterpriseMeta(&entMeta)

	if req.Method == "POST" {
		var raw map[string]interface{}
		if err := decodeBody(req.Body, &raw); err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decoding failed: %v", err)}
		}

		apiReq, err := decodeDiscoveryChainReadRequest(raw)
		if err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decoding failed: %v", err)}
		}

		args.OverrideProtocol = apiReq.OverrideProtocol
		args.OverrideConnectTimeout = apiReq.OverrideConnectTimeout

		if apiReq.OverrideMeshGateway.Mode != "" {
			_, err := structs.ValidateMeshGatewayMode(string(apiReq.OverrideMeshGateway.Mode))
			if err != nil {
				return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Invalid OverrideMeshGateway.Mode parameter"}
			}
			args.OverrideMeshGateway = apiReq.OverrideMeshGateway
		}
	}

	// Make the RPC request
	var out structs.DiscoveryChainResponse
	defer setMeta(resp, &out.QueryMeta)

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(req.Context(), cachetype.CompiledDiscoveryChainName, &args)
		if err != nil {
			return nil, err
		}
		defer setCacheMeta(resp, &m)

		reply, ok := raw.(*structs.DiscoveryChainResponse)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		out = *reply
	} else {
	RETRY_ONCE:
		if err := s.agent.RPC("DiscoveryChain.Get", &args, &out); err != nil {
			return nil, err
		}
		if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
			args.AllowStale = false
			args.MaxStaleDuration = 0
			goto RETRY_ONCE
		}
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	return discoveryChainReadResponse{Chain: out.Chain}, nil
}

// discoveryChainReadRequest is the API variation of structs.DiscoveryChainRequest
type discoveryChainReadRequest struct {
	OverrideMeshGateway    structs.MeshGatewayConfig `alias:"override_mesh_gateway"`
	OverrideProtocol       string                    `alias:"override_protocol"`
	OverrideConnectTimeout time.Duration             `alias:"override_connect_timeout"`
}

// discoveryChainReadResponse is the API variation of structs.DiscoveryChainResponse
type discoveryChainReadResponse struct {
	Chain *structs.CompiledDiscoveryChain
}

func decodeDiscoveryChainReadRequest(raw map[string]interface{}) (*discoveryChainReadRequest, error) {
	var apiReq discoveryChainReadRequest
	// TODO(dnephin): at this time only JSON payloads are read, so it is unlikely
	// that HookWeakDecodeFromSlice is necessary. It was added while porting
	// from lib.PatchSliceOfMaps to decode.HookWeakDecodeFromSlice. It may be
	// safe to remove in the future.
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			decode.HookWeakDecodeFromSlice,
			decode.HookTranslateKeys,
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:           &apiReq,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(raw); err != nil {
		return nil, err
	}

	return &apiReq, nil
}
