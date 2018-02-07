package agent

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
)

type Self struct {
	Config      interface{}
	DebugConfig map[string]interface{}
	Coord       *coordinate.Coordinate
	Member      serf.Member
	Stats       map[string]map[string]string
	Meta        map[string]string
}

func (s *HTTPServer) AgentSelf(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentRead(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	var cs lib.CoordinateSet
	if !s.agent.config.DisableCoordinates {
		var err error
		if cs, err = s.agent.GetLANCoordinate(); err != nil {
			return nil, err
		}
	}

	config := struct {
		Datacenter string
		NodeName   string
		NodeID     string
		Revision   string
		Server     bool
		Version    string
	}{
		Datacenter: s.agent.config.Datacenter,
		NodeName:   s.agent.config.NodeName,
		NodeID:     string(s.agent.config.NodeID),
		Revision:   s.agent.config.Revision,
		Server:     s.agent.config.ServerMode,
		Version:    s.agent.config.Version,
	}
	return Self{
		Config:      config,
		DebugConfig: s.agent.config.Sanitized(),
		Coord:       cs[s.agent.config.SegmentName],
		Member:      s.agent.LocalMember(),
		Stats:       s.agent.Stats(),
		Meta:        s.agent.State.Metadata(),
	}, nil
}

func (s *HTTPServer) AgentMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentRead(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	return s.agent.MemSink.DisplayMetrics(resp, req)
}

func (s *HTTPServer) AgentReload(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentWrite(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	// Trigger the reload
	errCh := make(chan error, 0)
	select {
	case <-s.agent.shutdownCh:
		return nil, fmt.Errorf("Agent was shutdown before reload could be completed")
	case s.agent.reloadCh <- errCh:
	}

	// Wait for the result of the reload, or for the agent to shutdown
	select {
	case <-s.agent.shutdownCh:
		return nil, fmt.Errorf("Agent was shutdown before reload could be completed")
	case err := <-errCh:
		return nil, err
	}
}

func (s *HTTPServer) AgentServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	services := s.agent.State.Services()
	if err := s.agent.filterServices(token, &services); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	for id, s := range services {
		if s.Tags == nil {
			clone := *s
			clone.Tags = make([]string, 0)
			services[id] = &clone
		}
	}

	return services, nil
}

func (s *HTTPServer) AgentChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	checks := s.agent.State.Checks()
	if err := s.agent.filterChecks(token, &checks); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	for id, c := range checks {
		if c.ServiceTags == nil {
			clone := *c
			clone.ServiceTags = make([]string, 0)
			checks[id] = &clone
		}
	}

	return checks, nil
}

func (s *HTTPServer) AgentMembers(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	// Check if the WAN is being queried
	wan := false
	if other := req.URL.Query().Get("wan"); other != "" {
		wan = true
	}

	segment := req.URL.Query().Get("segment")
	if wan {
		switch segment {
		case "", api.AllSegments:
			// The zero value and the special "give me all members"
			// key are ok, otherwise the argument doesn't apply to
			// the WAN.
		default:
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, "Cannot provide a segment with wan=true")
			return nil, nil
		}
	}

	var members []serf.Member
	if wan {
		members = s.agent.WANMembers()
	} else {
		var err error
		if segment == api.AllSegments {
			members, err = s.agent.delegate.LANMembersAllSegments()
		} else {
			members, err = s.agent.delegate.LANSegmentMembers(segment)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := s.agent.filterMembers(token, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (s *HTTPServer) AgentJoin(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentWrite(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	// Check if the WAN is being queried
	wan := false
	if other := req.URL.Query().Get("wan"); other != "" {
		wan = true
	}

	// Get the address
	addr := strings.TrimPrefix(req.URL.Path, "/v1/agent/join/")
	if wan {
		_, err = s.agent.JoinWAN([]string{addr})
	} else {
		_, err = s.agent.JoinLAN([]string{addr})
	}
	return nil, err
}

func (s *HTTPServer) AgentLeave(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentWrite(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	if err := s.agent.Leave(); err != nil {
		return nil, err
	}
	return nil, s.agent.ShutdownAgent()
}

func (s *HTTPServer) AgentForceLeave(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentWrite(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	addr := strings.TrimPrefix(req.URL.Path, "/v1/agent/force-leave/")
	return nil, s.agent.ForceLeave(addr)
}

// syncChanges is a helper function which wraps a blocking call to sync
// services and checks to the server. If the operation fails, we only
// only warn because the write did succeed and anti-entropy will sync later.
func (s *HTTPServer) syncChanges() {
	if err := s.agent.State.SyncChanges(); err != nil {
		s.agent.logger.Printf("[ERR] agent: failed to sync changes: %v", err)
	}
}

func (s *HTTPServer) AgentRegisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	var args structs.CheckDefinition
	// Fixup the type decode of TTL or Interval.
	decodeCB := func(raw interface{}) error {
		return FixupCheckType(raw)
	}
	if err := decodeBody(req, &args, decodeCB); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	// Verify the check has a name.
	if args.Name == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing check name")
		return nil, nil
	}

	if args.Status != "" && !structs.ValidStatus(args.Status) {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Bad check status")
		return nil, nil
	}

	// Construct the health check.
	health := args.HealthCheck(s.agent.config.NodeName)

	// Verify the check type.
	chkType := args.CheckType()
	err := chkType.Validate()
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, fmt.Errorf("Invalid check: %v", err))
		return nil, nil
	}

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetCheckRegister(token, health); err != nil {
		return nil, err
	}

	// Add the check.
	if err := s.agent.AddCheck(health, chkType, true, token); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentDeregisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/deregister/"))

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetCheckUpdate(token, checkID); err != nil {
		return nil, err
	}

	if err := s.agent.RemoveCheck(checkID, true); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentCheckPass(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/pass/"))
	note := req.URL.Query().Get("note")

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetCheckUpdate(token, checkID); err != nil {
		return nil, err
	}

	if err := s.agent.updateTTLCheck(checkID, api.HealthPassing, note); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentCheckWarn(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/warn/"))
	note := req.URL.Query().Get("note")

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetCheckUpdate(token, checkID); err != nil {
		return nil, err
	}

	if err := s.agent.updateTTLCheck(checkID, api.HealthWarning, note); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentCheckFail(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/fail/"))
	note := req.URL.Query().Get("note")

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetCheckUpdate(token, checkID); err != nil {
		return nil, err
	}

	if err := s.agent.updateTTLCheck(checkID, api.HealthCritical, note); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

// checkUpdate is the payload for a PUT to AgentCheckUpdate.
type checkUpdate struct {
	// Status us one of the api.Health* states, "passing", "warning", or
	// "critical".
	Status string

	// Output is the information to post to the UI for operators as the
	// output of the process that decided to hit the TTL check. This is
	// different from the note field that's associated with the check
	// itself.
	Output string
}

// AgentCheckUpdate is a PUT-based alternative to the GET-based Pass/Warn/Fail
// APIs.
func (s *HTTPServer) AgentCheckUpdate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	var update checkUpdate
	if err := decodeBody(req, &update, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	switch update.Status {
	case api.HealthPassing:
	case api.HealthWarning:
	case api.HealthCritical:
	default:
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Invalid check status: '%s'", update.Status)
		return nil, nil
	}

	total := len(update.Output)
	if total > checks.BufSize {
		update.Output = fmt.Sprintf("%s ... (captured %d of %d bytes)",
			update.Output[:checks.BufSize], checks.BufSize, total)
	}

	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/update/"))

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetCheckUpdate(token, checkID); err != nil {
		return nil, err
	}

	if err := s.agent.updateTTLCheck(checkID, update.Status, update.Output); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentRegisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	var args structs.ServiceDefinition
	// Fixup the type decode of TTL or Interval if a check if provided.
	decodeCB := func(raw interface{}) error {
		rawMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil
		}

		// see https://github.com/hashicorp/consul/pull/3557 why we need this
		// and why we should get rid of it.
		config.TranslateKeys(rawMap, map[string]string{
			"enable_tag_override": "EnableTagOverride",
		})

		for k, v := range rawMap {
			switch strings.ToLower(k) {
			case "check":
				if err := FixupCheckType(v); err != nil {
					return err
				}
			case "checks":
				chkTypes, ok := v.([]interface{})
				if !ok {
					continue
				}
				for _, chkType := range chkTypes {
					if err := FixupCheckType(chkType); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := decodeBody(req, &args, decodeCB); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	// Verify the service has a name.
	if args.Name == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing service name")
		return nil, nil
	}

	// Check the service address here and in the catalog RPC endpoint
	// since service registration isn't sychronous.
	if ipaddr.IsAny(args.Address) {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Invalid service address")
		return nil, nil
	}

	// Get the node service.
	ns := args.NodeService()

	// Verify the check type.
	chkTypes, err := args.CheckTypes()
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, fmt.Errorf("Invalid check: %v", err))
		return nil, nil
	}
	for _, check := range chkTypes {
		if check.Status != "" && !structs.ValidStatus(check.Status) {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, "Status for checks must 'passing', 'warning', 'critical'")
			return nil, nil
		}
	}

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetServiceRegister(token, ns); err != nil {
		return nil, err
	}

	// Add the service.
	if err := s.agent.AddService(ns, chkTypes, true, token); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentDeregisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	serviceID := strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/")

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetServiceUpdate(token, serviceID); err != nil {
		return nil, err
	}

	if err := s.agent.RemoveService(serviceID, true); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentServiceMaintenance(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	// Ensure we have a service ID
	serviceID := strings.TrimPrefix(req.URL.Path, "/v1/agent/service/maintenance/")
	if serviceID == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing service ID")
		return nil, nil
	}

	// Ensure we have some action
	params := req.URL.Query()
	if _, ok := params["enable"]; !ok {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing value for enable")
		return nil, nil
	}

	raw := params.Get("enable")
	enable, err := strconv.ParseBool(raw)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Invalid value for enable: %q", raw)
		return nil, nil
	}

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetServiceUpdate(token, serviceID); err != nil {
		return nil, err
	}

	if enable {
		reason := params.Get("reason")
		if err = s.agent.EnableServiceMaintenance(serviceID, reason, token); err != nil {
			resp.WriteHeader(http.StatusNotFound)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
	} else {
		if err = s.agent.DisableServiceMaintenance(serviceID); err != nil {
			resp.WriteHeader(http.StatusNotFound)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentNodeMaintenance(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	// Ensure we have some action
	params := req.URL.Query()
	if _, ok := params["enable"]; !ok {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing value for enable")
		return nil, nil
	}

	raw := params.Get("enable")
	enable, err := strconv.ParseBool(raw)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Invalid value for enable: %q", raw)
		return nil, nil
	}

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.NodeWrite(s.agent.config.NodeName, nil) {
		return nil, acl.ErrPermissionDenied
	}

	if enable {
		s.agent.EnableNodeMaintenance(params.Get("reason"), token)
	} else {
		s.agent.DisableNodeMaintenance()
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentMonitor(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentRead(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	// Get the provided loglevel.
	logLevel := req.URL.Query().Get("loglevel")
	if logLevel == "" {
		logLevel = "INFO"
	}

	// Upper case the level since that's required by the filter.
	logLevel = strings.ToUpper(logLevel)

	// Create a level filter and flusher.
	filter := logger.LevelFilter()
	filter.MinLevel = logutils.LogLevel(logLevel)
	if !logger.ValidateLevelFilter(filter.MinLevel, filter) {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Unknown log level: %s", filter.MinLevel)
		return nil, nil
	}
	flusher, ok := resp.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("Streaming not supported")
	}

	// Set up a log handler.
	handler := &httpLogHandler{
		filter: filter,
		logCh:  make(chan string, 512),
		logger: s.agent.logger,
	}
	s.agent.LogWriter.RegisterHandler(handler)
	defer s.agent.LogWriter.DeregisterHandler(handler)
	notify := resp.(http.CloseNotifier).CloseNotify()

	// Stream logs until the connection is closed.
	for {
		select {
		case <-notify:
			s.agent.LogWriter.DeregisterHandler(handler)
			if handler.droppedCount > 0 {
				s.agent.logger.Printf("[WARN] agent: Dropped %d logs during monitor request", handler.droppedCount)
			}
			return nil, nil
		case log := <-handler.logCh:
			fmt.Fprintln(resp, log)
			flusher.Flush()
		}
	}
}

type httpLogHandler struct {
	filter       *logutils.LevelFilter
	logCh        chan string
	logger       *log.Logger
	droppedCount int
}

func (h *httpLogHandler) HandleLog(log string) {
	// Check the log level
	if !h.filter.Check([]byte(log)) {
		return
	}

	// Do a non-blocking send
	select {
	case h.logCh <- log:
	default:
		// Just increment a counter for dropped logs to this handler; we can't log now
		// because the lock is already held by the LogWriter invoking this
		h.droppedCount++
	}
}

func (s *HTTPServer) AgentToken(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.AgentWrite(s.agent.config.NodeName) {
		return nil, acl.ErrPermissionDenied
	}

	// The body is just the token, but it's in a JSON object so we can add
	// fields to this later if needed.
	var args api.AgentToken
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	// Figure out the target token.
	target := strings.TrimPrefix(req.URL.Path, "/v1/agent/token/")
	switch target {
	case "acl_token":
		s.agent.tokens.UpdateUserToken(args.Token)

	case "acl_agent_token":
		s.agent.tokens.UpdateAgentToken(args.Token)

	case "acl_agent_master_token":
		s.agent.tokens.UpdateAgentMasterToken(args.Token)

	case "acl_replication_token":
		s.agent.tokens.UpdateACLReplicationToken(args.Token)

	default:
		resp.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(resp, "Token %q is unknown", target)
		return nil, nil
	}

	s.agent.logger.Printf("[INFO] Updated agent's ACL token %q", target)
	return nil, nil
}
