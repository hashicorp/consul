package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
)

// GET /v1/connect/ca/roots
func (s *HTTPServer) ConnectCARoots(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.IndexedCARoots
	defer setMeta(resp, &reply.QueryMeta)
	if err := s.agent.RPC("ConnectCA.Roots", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// /v1/connect/ca/configuration
func (s *HTTPServer) ConnectCAConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.ConnectCAConfigurationGet(resp, req)

	case "PUT":
		return s.ConnectCAConfigurationSet(resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "POST"}}
	}
}

// GEt /v1/connect/ca/configuration
func (s *HTTPServer) ConnectCAConfigurationGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in ConnectCAConfiguration
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.CAConfiguration
	err := s.agent.RPC("ConnectCA.ConfigurationGet", &args, &reply)
	if err != nil {
		return nil, err
	}

	fixupConfig(&reply)
	return reply, nil
}

// PUT /v1/connect/ca/configuration
func (s *HTTPServer) ConnectCAConfigurationSet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in ConnectCAConfiguration

	var args structs.CARequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req, &args.Config, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	var reply interface{}
	err := s.agent.RPC("ConnectCA.ConfigurationSet", &args, &reply)
	return nil, err
}

// A hack to fix up the config types inside of the map[string]interface{}
// so that they get formatted correctly during json.Marshal. Without this,
// string values that get converted to []uint8 end up getting output back
// to the user in base64-encoded form.
func fixupConfig(conf *structs.CAConfiguration) {
	for k, v := range conf.Config {
		if raw, ok := v.([]uint8); ok {
			strVal := ca.Uint8ToString(raw)
			conf.Config[k] = strVal
			switch conf.Provider {
			case structs.ConsulCAProvider:
				if k == "PrivateKey" && strVal != "" {
					conf.Config["PrivateKey"] = "hidden"
				}
			case structs.VaultCAProvider:
				if k == "Token" && strVal != "" {
					conf.Config["Token"] = "hidden"
				}
			}
		}
	}
}
