package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"strings"
)

func (s *HTTPServer) AgentServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	services := s.agent.state.Services()
	return services, nil
}

func (s *HTTPServer) AgentChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checks := s.agent.state.Checks()
	return checks, nil
}

func (s *HTTPServer) AgentMembers(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Check if the WAN is being queried
	wan := false
	if other := req.URL.Query().Get("wan"); other != "" {
		wan = true
	}
	if wan {
		return s.agent.WANMembers(), nil
	} else {
		return s.agent.LANMembers(), nil
	}
}

func (s *HTTPServer) AgentJoin(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Check if the WAN is being queried
	wan := false
	if other := req.URL.Query().Get("wan"); other != "" {
		wan = true
	}

	// Get the address
	addr := strings.TrimPrefix(req.URL.Path, "/v1/agent/join/")
	if wan {
		_, err := s.agent.JoinWAN([]string{addr})
		return nil, err
	} else {
		_, err := s.agent.JoinLAN([]string{addr})
		return nil, err
	}
}

func (s *HTTPServer) AgentForceLeave(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	addr := strings.TrimPrefix(req.URL.Path, "/v1/agent/force-leave/")
	return nil, s.agent.ForceLeave(addr)
}

func (s *HTTPServer) AgentRegisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args CheckDefinition
	// Fixup the type decode of TTL or Interval
	decodeCB := func(raw interface{}) error {
		return FixupCheckType(raw)
	}
	if err := decodeBody(req, &args, decodeCB); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}

	// Verify the check has a name
	if args.Name == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing check name"))
		return nil, nil
	}

	// Construct the health check
	health := args.HealthCheck(s.agent.config.NodeName)

	// Verify the check type
	chkType := &args.CheckType
	if !chkType.Valid() {
		resp.WriteHeader(400)
		resp.Write([]byte("Must provide TTL or Script and Interval!"))
		return nil, nil
	}

	// Add the check
	return nil, s.agent.AddCheck(health, chkType)
}

func (s *HTTPServer) AgentDeregisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := strings.TrimPrefix(req.URL.Path, "/v1/agent/check/deregister/")
	return nil, s.agent.RemoveCheck(checkID)
}

func (s *HTTPServer) AgentCheckPass(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := strings.TrimPrefix(req.URL.Path, "/v1/agent/check/pass/")
	note := req.URL.Query().Get("note")
	return nil, s.agent.UpdateCheck(checkID, structs.HealthPassing, note)
}

func (s *HTTPServer) AgentCheckWarn(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := strings.TrimPrefix(req.URL.Path, "/v1/agent/check/warn/")
	note := req.URL.Query().Get("note")
	return nil, s.agent.UpdateCheck(checkID, structs.HealthWarning, note)
}

func (s *HTTPServer) AgentCheckFail(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := strings.TrimPrefix(req.URL.Path, "/v1/agent/check/fail/")
	note := req.URL.Query().Get("note")
	return nil, s.agent.UpdateCheck(checkID, structs.HealthCritical, note)
}

func (s *HTTPServer) AgentRegisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args ServiceDefinition
	// Fixup the type decode of TTL or Interval if a check if provided
	decodeCB := func(raw interface{}) error {
		rawMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil
		}
		check, ok := rawMap["check"]
		if !ok {
			return nil
		}
		return FixupCheckType(check)
	}
	if err := decodeBody(req, &args, decodeCB); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}

	// Verify the service has a name
	if args.Name == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing service name"))
		return nil, nil
	}

	// Get the node service
	ns := args.NodeService()

	// Verify the check type
	chkType := args.CheckType()
	if chkType != nil && !chkType.Valid() {
		resp.WriteHeader(400)
		resp.Write([]byte("Must provide TTL or Script and Interval!"))
		return nil, nil
	}

	// Add the check
	return nil, s.agent.AddService(ns, chkType)
}

func (s *HTTPServer) AgentDeregisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	serviceID := strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/")
	return nil, s.agent.RemoveService(serviceID)
}
