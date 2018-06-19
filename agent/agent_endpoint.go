package agent

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

// enablePrometheusOutput will look for Prometheus mime-type or format Query parameter the same way as Nomad
func enablePrometheusOutput(req *http.Request) bool {
	if format := req.URL.Query().Get("format"); format == "prometheus" {
		return true
	}
	return false
}

func (s *HTTPServer) AgentMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
	if enablePrometheusOutput(req) {
		if s.agent.config.Telemetry.PrometheusRetentionTime < 1 {
			resp.WriteHeader(http.StatusUnsupportedMediaType)
			fmt.Fprint(resp, "Prometheus is not enable since its retention time is not positive")
			return nil, nil
		}
		handlerOptions := promhttp.HandlerOpts{
			ErrorLog:      s.agent.logger,
			ErrorHandling: promhttp.ContinueOnError,
		}

		handler := promhttp.HandlerFor(prometheus.DefaultGatherer, handlerOptions)
		handler.ServeHTTP(resp, req)
		return nil, nil
	}
	return s.agent.MemSink.DisplayMetrics(resp, req)
}

func (s *HTTPServer) AgentReload(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	services := s.agent.State.Services()
	if err := s.agent.filterServices(token, &services); err != nil {
		return nil, err
	}

	proxies := s.agent.State.Proxies()

	// Convert into api.AgentService since that includes Connect config but so far
	// NodeService doesn't need to internally. They are otherwise identical since
	// that is the struct used in client for reading the one we output here
	// anyway.
	agentSvcs := make(map[string]*api.AgentService)

	// Use empty list instead of nil
	for id, s := range services {
		as := &api.AgentService{
			Kind:              api.ServiceKind(s.Kind),
			ID:                s.ID,
			Service:           s.Service,
			Tags:              s.Tags,
			Meta:              s.Meta,
			Port:              s.Port,
			Address:           s.Address,
			EnableTagOverride: s.EnableTagOverride,
			CreateIndex:       s.CreateIndex,
			ModifyIndex:       s.ModifyIndex,
			ProxyDestination:  s.ProxyDestination,
		}
		if as.Tags == nil {
			as.Tags = []string{}
		}
		if as.Meta == nil {
			as.Meta = map[string]string{}
		}
		// Attach Connect configs if the exist
		if proxy, ok := proxies[id+"-proxy"]; ok {
			as.Connect = &api.AgentServiceConnect{
				Proxy: &api.AgentServiceConnectProxy{
					ExecMode: api.ProxyExecMode(proxy.Proxy.ExecMode.String()),
					Command:  proxy.Proxy.Command,
					Config:   proxy.Proxy.Config,
				},
			}
		}
		agentSvcs[id] = as
	}

	return agentSvcs, nil
}

func (s *HTTPServer) AgentChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
	// since service registration isn't synchronous.
	if ipaddr.IsAny(args.Address) {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Invalid service address")
		return nil, nil
	}

	// Get the node service.
	ns := args.NodeService()
	if err := structs.ValidateMetadata(ns.Meta, false); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, fmt.Errorf("Invalid Service Meta: %v", err))
		return nil, nil
	}

	// Run validation. This is the same validation that would happen on
	// the catalog endpoint so it helps ensure the sync will work properly.
	if err := ns.Validate(); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, err.Error())
		return nil, nil
	}

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

	// Get any proxy registrations
	proxy, err := args.ConnectManagedProxy()
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, err.Error())
		return nil, nil
	}

	// If we have a proxy, verify that we're allowed to add a proxy via the API
	if proxy != nil && !s.agent.config.ConnectProxyAllowManagedAPIRegistration {
		return nil, &BadRequestError{
			Reason: "Managed proxy registration via the API is disallowed."}
	}

	// Add the service.
	if err := s.agent.AddService(ns, chkTypes, true, token); err != nil {
		return nil, err
	}
	// Add proxy (which will add proxy service so do it before we trigger sync)
	if proxy != nil {
		if err := s.agent.AddProxy(proxy, true, ""); err != nil {
			return nil, err
		}
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentDeregisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	serviceID := strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/")

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetServiceUpdate(token, serviceID); err != nil {
		return nil, err
	}

	// Verify this isn't a proxy
	if s.agent.State.Proxy(serviceID) != nil {
		return nil, &BadRequestError{
			Reason: "Managed proxy service cannot be deregistered directly. " +
				"Deregister the service that has a managed proxy to automatically " +
				"deregister the managed proxy itself."}
	}

	if err := s.agent.RemoveService(serviceID, true); err != nil {
		return nil, err
	}

	// Remove the associated managed proxy if it exists
	for proxyID, p := range s.agent.State.Proxies() {
		if p.Proxy.TargetServiceID == serviceID {
			if err := s.agent.RemoveProxy(proxyID, true); err != nil {
				return nil, err
			}
		}
	}

	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentServiceMaintenance(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

	// Send header so client can start streaming body
	resp.WriteHeader(http.StatusOK)

	// 0 byte write is needed before the Flush call so that if we are using
	// a gzip stream it will go ahead and write out the HTTP response header
	resp.Write([]byte(""))
	flusher.Flush()

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

	s.agent.logger.Printf("[INFO] agent: Updated agent's ACL token %q", target)
	return nil, nil
}

// AgentConnectCARoots returns the trusted CA roots.
func (s *HTTPServer) AgentConnectCARoots(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	raw, m, err := s.agent.cache.Get(cachetype.ConnectCARootName, &args)
	if err != nil {
		return nil, err
	}
	defer setCacheMeta(resp, &m)

	// Add cache hit

	reply, ok := raw.(*structs.IndexedCARoots)
	if !ok {
		// This should never happen, but we want to protect against panics
		return nil, fmt.Errorf("internal error: response type not correct")
	}
	defer setMeta(resp, &reply.QueryMeta)

	return *reply, nil
}

// AgentConnectCALeafCert returns the certificate bundle for a service
// instance. This supports blocking queries to update the returned bundle.
func (s *HTTPServer) AgentConnectCALeafCert(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the service name. Note that this is the name of the sevice,
	// not the ID of the service instance.
	serviceName := strings.TrimPrefix(req.URL.Path, "/v1/agent/connect/ca/leaf/")

	args := cachetype.ConnectCALeafRequest{
		Service: serviceName, // Need name not ID
	}
	var qOpts structs.QueryOptions
	// Store DC in the ConnectCALeafRequest but query opts separately
	if done := s.parse(resp, req, &args.Datacenter, &qOpts); done {
		return nil, nil
	}
	args.MinQueryIndex = qOpts.MinQueryIndex

	// Verify the proxy token. This will check both the local proxy token
	// as well as the ACL if the token isn't local.
	effectiveToken, _, err := s.agent.verifyProxyToken(qOpts.Token, serviceName, "")
	if err != nil {
		return nil, err
	}
	args.Token = effectiveToken

	raw, m, err := s.agent.cache.Get(cachetype.ConnectCALeafName, &args)
	if err != nil {
		return nil, err
	}
	defer setCacheMeta(resp, &m)

	reply, ok := raw.(*structs.IssuedCert)
	if !ok {
		// This should never happen, but we want to protect against panics
		return nil, fmt.Errorf("internal error: response type not correct")
	}
	setIndex(resp, reply.ModifyIndex)

	return reply, nil
}

// GET /v1/agent/connect/proxy/:proxy_service_id
//
// Returns the local proxy config for the identified proxy. Requires token=
// param with the correct local ProxyToken (not ACL token).
func (s *HTTPServer) AgentConnectProxyConfig(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the proxy ID. Note that this is the ID of a proxy's service instance.
	id := strings.TrimPrefix(req.URL.Path, "/v1/agent/connect/proxy/")

	// Maybe block
	var queryOpts structs.QueryOptions
	if parseWait(resp, req, &queryOpts) {
		// parseWait returns an error itself
		return nil, nil
	}

	// Parse the token
	var token string
	s.parseToken(req, &token)

	// Parse hash specially since it's only this endpoint that uses it currently.
	// Eventually this should happen in parseWait and end up in QueryOptions but I
	// didn't want to make very general changes right away.
	hash := req.URL.Query().Get("hash")

	return s.agentLocalBlockingQuery(resp, hash, &queryOpts,
		func(ws memdb.WatchSet) (string, interface{}, error) {
			// Retrieve the proxy specified
			proxy := s.agent.State.Proxy(id)
			if proxy == nil {
				resp.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(resp, "unknown proxy service ID: %s", id)
				return "", nil, nil
			}

			// Lookup the target service as a convenience
			target := s.agent.State.Service(proxy.Proxy.TargetServiceID)
			if target == nil {
				// Not found since this endpoint is only useful for agent-managed proxies so
				// service missing means the service was deregistered racily with this call.
				resp.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(resp, "unknown target service ID: %s", proxy.Proxy.TargetServiceID)
				return "", nil, nil
			}

			// Validate the ACL token
			_, isProxyToken, err := s.agent.verifyProxyToken(token, target.Service, id)
			if err != nil {
				return "", nil, err
			}

			// Watch the proxy for changes
			ws.Add(proxy.WatchCh)

			hash, err := hashstructure.Hash(proxy.Proxy, nil)
			if err != nil {
				return "", nil, err
			}
			contentHash := fmt.Sprintf("%x", hash)

			// Set defaults
			config, err := s.agent.applyProxyConfigDefaults(proxy.Proxy)
			if err != nil {
				return "", nil, err
			}

			// Only merge in telemetry config from agent if the requested is
			// authorized with a proxy token. This prevents us leaking potentially
			// sensitive config like Circonus API token via a public endpoint. Proxy
			// tokens are only ever generated in-memory and passed via ENV to a child
			// proxy process so potential for abuse here seems small. This endpoint in
			// general is only useful for managed proxies now so it should _always_ be
			// true that auth is via a proxy token but inconvenient for testing if we
			// lock it down so strictly.
			if isProxyToken {
				// Add telemetry config. Copy the global config so we can customize the
				// prefix.
				telemetryCfg := s.agent.config.Telemetry
				telemetryCfg.MetricsPrefix = telemetryCfg.MetricsPrefix + ".proxy." + target.ID

				// First see if the user has specified telemetry
				if userRaw, ok := config["telemetry"]; ok {
					// User specified domething, see if it is compatible with agent
					// telemetry config:
					var uCfg lib.TelemetryConfig
					dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
						Result: &uCfg,
						// Make sure that if the user passes something that isn't just a
						// simple override of a valid TelemetryConfig that we fail so that we
						// don't clobber their custom config.
						ErrorUnused: true,
					})
					if err == nil {
						if err = dec.Decode(userRaw); err == nil {
							// It did decode! Merge any unspecified fields from agent config.
							uCfg.MergeDefaults(&telemetryCfg)
							config["telemetry"] = uCfg
						}
					}
					// Failed to decode, just keep user's config["telemetry"] verbatim
					// with no agent merge.
				} else {
					// Add agent telemetry config.
					config["telemetry"] = telemetryCfg
				}
			}

			reply := &api.ConnectProxyConfig{
				ProxyServiceID:    proxy.Proxy.ProxyService.ID,
				TargetServiceID:   target.ID,
				TargetServiceName: target.Service,
				ContentHash:       contentHash,
				ExecMode:          api.ProxyExecMode(proxy.Proxy.ExecMode.String()),
				Command:           proxy.Proxy.Command,
				Config:            config,
			}
			return contentHash, reply, nil
		})
}

type agentLocalBlockingFunc func(ws memdb.WatchSet) (string, interface{}, error)

// agentLocalBlockingQuery performs a blocking query in a generic way against
// local agent state that has no RPC or raft to back it. It uses `hash` paramter
// instead of an `index`. The resp is needed to write the `X-Consul-ContentHash`
// header back on return no Status nor body content is ever written to it.
func (s *HTTPServer) agentLocalBlockingQuery(resp http.ResponseWriter, hash string,
	queryOpts *structs.QueryOptions, fn agentLocalBlockingFunc) (interface{}, error) {

	// If we are not blocking we can skip tracking and allocating - nil WatchSet
	// is still valid to call Add on and will just be a no op.
	var ws memdb.WatchSet
	var timeout *time.Timer

	if hash != "" {
		// TODO(banks) at least define these defaults somewhere in a const. Would be
		// nice not to duplicate the ones in consul/rpc.go too...
		wait := queryOpts.MaxQueryTime
		if wait == 0 {
			wait = 5 * time.Minute
		}
		if wait > 10*time.Minute {
			wait = 10 * time.Minute
		}
		// Apply a small amount of jitter to the request.
		wait += lib.RandomStagger(wait / 16)
		timeout = time.NewTimer(wait)
	}

	for {
		// Must reset this every loop in case the Watch set is already closed but
		// hash remains same. In that case we'll need to re-block on ws.Watch()
		// again.
		ws = memdb.NewWatchSet()
		curHash, curResp, err := fn(ws)
		if err != nil {
			return curResp, err
		}
		// Return immediately if there is no timeout, the hash is different or the
		// Watch returns true (indicating timeout fired). Note that Watch on a nil
		// WatchSet immediately returns false which would incorrectly cause this to
		// loop and repeat again, however we rely on the invariant that ws == nil
		// IFF timeout == nil in which case the Watch call is never invoked.
		if timeout == nil || hash != curHash || ws.Watch(timeout.C) {
			resp.Header().Set("X-Consul-ContentHash", curHash)
			return curResp, err
		}
		// Watch returned false indicating a change was detected, loop and repeat
		// the callback to load the new value.
	}
}

// AgentConnectAuthorize
//
// POST /v1/agent/connect/authorize
//
// Note: when this logic changes, consider if the Intention.Check RPC method
// also needs to be updated.
func (s *HTTPServer) AgentConnectAuthorize(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the token
	var token string
	s.parseToken(req, &token)

	// Decode the request from the request body
	var authReq structs.ConnectAuthorizeRequest
	if err := decodeBody(req, &authReq, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	// We need to have a target to check intentions
	if authReq.Target == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Target service must be specified")
		return nil, nil
	}

	// Parse the certificate URI from the client ID
	uriRaw, err := url.Parse(authReq.ClientCertURI)
	if err != nil {
		return &connectAuthorizeResp{
			Authorized: false,
			Reason:     fmt.Sprintf("Client ID must be a URI: %s", err),
		}, nil
	}
	uri, err := connect.ParseCertURI(uriRaw)
	if err != nil {
		return &connectAuthorizeResp{
			Authorized: false,
			Reason:     fmt.Sprintf("Invalid client ID: %s", err),
		}, nil
	}

	uriService, ok := uri.(*connect.SpiffeIDService)
	if !ok {
		return &connectAuthorizeResp{
			Authorized: false,
			Reason:     "Client ID must be a valid SPIFFE service URI",
		}, nil
	}

	// We need to verify service:write permissions for the given token.
	// We do this manually here since the RPC request below only verifies
	// service:read.
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && !rule.ServiceWrite(authReq.Target, nil) {
		return nil, acl.ErrPermissionDenied
	}

	// Validate the trust domain matches ours. Later we will support explicit
	// external federation but not built yet.
	rootArgs := &structs.DCSpecificRequest{Datacenter: s.agent.config.Datacenter}
	raw, _, err := s.agent.cache.Get(cachetype.ConnectCARootName, rootArgs)
	if err != nil {
		return nil, err
	}

	roots, ok := raw.(*structs.IndexedCARoots)
	if !ok {
		return nil, fmt.Errorf("internal error: roots response type not correct")
	}
	if roots.TrustDomain == "" {
		return nil, fmt.Errorf("connect CA not bootstrapped yet")
	}
	if roots.TrustDomain != strings.ToLower(uriService.Host) {
		return &connectAuthorizeResp{
			Authorized: false,
			Reason: fmt.Sprintf("Identity from an external trust domain: %s",
				uriService.Host),
		}, nil
	}

	// TODO(banks): Implement revocation list checking here.

	// Get the intentions for this target service.
	args := &structs.IntentionQueryRequest{
		Datacenter: s.agent.config.Datacenter,
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: structs.IntentionDefaultNamespace,
					Name:      authReq.Target,
				},
			},
		},
	}
	args.Token = token

	raw, m, err := s.agent.cache.Get(cachetype.IntentionMatchName, args)
	if err != nil {
		return nil, err
	}
	setCacheMeta(resp, &m)

	reply, ok := raw.(*structs.IndexedIntentionMatches)
	if !ok {
		return nil, fmt.Errorf("internal error: response type not correct")
	}
	if len(reply.Matches) != 1 {
		return nil, fmt.Errorf("Internal error loading matches")
	}

	// Test the authorization for each match
	for _, ixn := range reply.Matches[0] {
		if auth, ok := uriService.Authorize(ixn); ok {
			return &connectAuthorizeResp{
				Authorized: auth,
				Reason:     fmt.Sprintf("Matched intention: %s", ixn.String()),
			}, nil
		}
	}

	// No match, we need to determine the default behavior. We do this by
	// specifying the anonymous token token, which will get that behavior.
	// The default behavior if ACLs are disabled is to allow connections
	// to mimic the behavior of Consul itself: everything is allowed if
	// ACLs are disabled.
	rule, err = s.agent.resolveToken("")
	if err != nil {
		return nil, err
	}
	authz := true
	reason := "ACLs disabled, access is allowed by default"
	if rule != nil {
		authz = rule.IntentionDefaultAllow()
		reason = "Default behavior configured by ACLs"
	}

	return &connectAuthorizeResp{
		Authorized: authz,
		Reason:     reason,
	}, nil
}

// connectAuthorizeResp is the response format/structure for the
// /v1/agent/connect/authorize endpoint.
type connectAuthorizeResp struct {
	Authorized bool   // True if authorized, false if not
	Reason     string // Reason for the Authorized value (whether true or false)
}
