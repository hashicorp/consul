package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/mitchellh/mapstructure"
)

func (s *HTTPServer) DiscoveryChainRead(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DiscoveryChainRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	args.Name = strings.TrimPrefix(req.URL.Path, "/v1/discovery-chain/")
	if args.Name == "" {
		return nil, BadRequestError{Reason: "Missing chain name"}
	}

	args.EvaluateInDatacenter = req.URL.Query().Get("compile-dc")
	// TODO(namespaces): args.EvaluateInNamespace = req.URL.Query().Get("compile-namespace")

	if req.Method == "POST" {
		var raw map[string]interface{}
		if err := decodeBody(req.Body, &raw); err != nil {
			return nil, BadRequestError{Reason: fmt.Sprintf("Request decoding failed: %v", err)}
		}

		apiReq, err := decodeDiscoveryChainReadRequest(raw)
		if err != nil {
			return nil, BadRequestError{Reason: fmt.Sprintf("Request decoding failed: %v", err)}
		}

		args.OverrideProtocol = apiReq.OverrideProtocol
		args.OverrideConnectTimeout = apiReq.OverrideConnectTimeout

		if apiReq.OverrideMeshGateway.Mode != "" {
			_, err := structs.ValidateMeshGatewayMode(string(apiReq.OverrideMeshGateway.Mode))
			if err != nil {
				resp.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(resp, "Invalid OverrideMeshGateway.Mode parameter")
				return nil, nil
			}
			args.OverrideMeshGateway = apiReq.OverrideMeshGateway
		}
	}

	// Make the RPC request
	var out structs.DiscoveryChainResponse
	defer setMeta(resp, &out.QueryMeta)

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(cachetype.CompiledDiscoveryChainName, &args)
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
	OverrideMeshGateway    structs.MeshGatewayConfig
	OverrideProtocol       string
	OverrideConnectTimeout time.Duration
}

func (t *discoveryChainReadRequest) UnmarshalJSON(data []byte) (err error) {
	type Alias discoveryChainReadRequest
	aux := &struct {
		OverrideConnectTimeout interface{}
		OverrideProtocol       interface{}
		OverrideMeshGateway    *struct{ Mode interface{} }

		OverrideConnectTimeoutSnake interface{}                 `json:"override_connect_timeout"`
		OverrideProtocolSnake       interface{}                 `json:"override_protocol"`
		OverrideMeshGatewaySnake    *struct{ Mode interface{} } `json:"override_mesh_gateway"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}

	if aux.OverrideConnectTimeout == nil {
		aux.OverrideConnectTimeout = aux.OverrideConnectTimeoutSnake
	}
	if aux.OverrideProtocol == nil {
		aux.OverrideProtocol = aux.OverrideProtocolSnake
	}
	if aux.OverrideMeshGateway == nil {
		aux.OverrideMeshGateway = aux.OverrideMeshGatewaySnake
	}

	// weakly typed input
	if aux.OverrideProtocol != nil {
		switch v := aux.OverrideProtocol.(type) {
		case string, float64, bool:
			t.OverrideProtocol = fmt.Sprintf("%v", v)
		default:
			return fmt.Errorf("OverrideProtocol: invalid type %T", v)
		}
	}
	if aux.OverrideMeshGateway != nil {
		t.OverrideMeshGateway.Mode = structs.MeshGatewayMode(fmt.Sprintf("%v", aux.OverrideMeshGateway.Mode))
	}

	// duration
	if aux.OverrideConnectTimeout != nil {
		switch v := aux.OverrideConnectTimeout.(type) {
		case string:
			if t.OverrideConnectTimeout, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.OverrideConnectTimeout = time.Duration(v)
		}
	}

	return nil
}

// discoveryChainReadResponse is the API variation of structs.DiscoveryChainResponse
type discoveryChainReadResponse struct {
	Chain *structs.CompiledDiscoveryChain
}

func decodeDiscoveryChainReadRequest(raw map[string]interface{}) (*discoveryChainReadRequest, error) {
	// lib.TranslateKeys doesn't understand []map[string]interface{} so we have
	// to do this part first.
	raw = lib.PatchSliceOfMaps(raw, nil, nil)

	lib.TranslateKeys(raw, map[string]string{
		"override_mesh_gateway":    "overridemeshgateway",
		"override_protocol":        "overrideprotocol",
		"override_connect_timeout": "overrideconnecttimeout",
	})

	var apiReq discoveryChainReadRequest
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
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
