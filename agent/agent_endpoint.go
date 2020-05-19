package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/debug"
	"github.com/hashicorp/consul/agent/structs"
	token_store "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/file"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/logging/monitor"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-bexpr"
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
	if rule != nil && rule.AgentRead(s.agent.config.NodeName, nil) != acl.Allow {
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

// acceptsOpenMetricsMimeType returns true if mime type is Prometheus-compatible
func acceptsOpenMetricsMimeType(acceptHeader string) bool {
	mimeTypes := strings.Split(acceptHeader, ",")
	for _, v := range mimeTypes {
		mimeInfo := strings.Split(v, ";")
		if len(mimeInfo) > 0 {
			rawMime := strings.ToLower(strings.Trim(mimeInfo[0], " "))
			if rawMime == "application/openmetrics-text" {
				return true
			}
			if rawMime == "text/plain" && (len(mimeInfo) > 1 && strings.Trim(mimeInfo[1], " ") == "version=0.4.0") {
				return true
			}
		}
	}
	return false
}

// enablePrometheusOutput will look for Prometheus mime-type or format Query parameter the same way as Nomad
func enablePrometheusOutput(req *http.Request) bool {
	if format := req.URL.Query().Get("format"); format == "prometheus" {
		return true
	}
	return acceptsOpenMetricsMimeType(req.Header.Get("Accept"))
}

func (s *HTTPServer) AgentMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}
	if rule != nil && rule.AgentRead(s.agent.config.NodeName, nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}
	if enablePrometheusOutput(req) {
		if s.agent.config.Telemetry.PrometheusRetentionTime < 1 {
			resp.WriteHeader(http.StatusUnsupportedMediaType)
			fmt.Fprint(resp, "Prometheus is not enabled since its retention time is not positive")
			return nil, nil
		}
		handlerOptions := promhttp.HandlerOpts{
			ErrorLog: s.agent.logger.StandardLogger(&hclog.StandardLoggerOptions{
				InferLevels: true,
			}),
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
	if rule != nil && rule.AgentWrite(s.agent.config.NodeName, nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	// Trigger the reload
	errCh := make(chan error)
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

func buildAgentService(s *structs.NodeService) api.AgentService {
	weights := api.AgentWeights{Passing: 1, Warning: 1}
	if s.Weights != nil {
		if s.Weights.Passing > 0 {
			weights.Passing = s.Weights.Passing
		}
		weights.Warning = s.Weights.Warning
	}

	var taggedAddrs map[string]api.ServiceAddress
	if len(s.TaggedAddresses) > 0 {
		taggedAddrs = make(map[string]api.ServiceAddress)
		for k, v := range s.TaggedAddresses {
			taggedAddrs[k] = v.ToAPIServiceAddress()
		}
	}

	as := api.AgentService{
		Kind:              api.ServiceKind(s.Kind),
		ID:                s.ID,
		Service:           s.Service,
		Tags:              s.Tags,
		Meta:              s.Meta,
		Port:              s.Port,
		Address:           s.Address,
		TaggedAddresses:   taggedAddrs,
		EnableTagOverride: s.EnableTagOverride,
		CreateIndex:       s.CreateIndex,
		ModifyIndex:       s.ModifyIndex,
		Weights:           weights,
	}

	if as.Tags == nil {
		as.Tags = []string{}
	}
	if as.Meta == nil {
		as.Meta = map[string]string{}
	}
	// Attach Proxy config if exists
	if s.Kind == structs.ServiceKindConnectProxy || s.IsGateway() {
		as.Proxy = s.Proxy.ToAPI()
	}

	// Attach Connect configs if they exist.
	if s.Connect.Native {
		as.Connect = &api.AgentServiceConnect{
			Native: true,
		}
	}

	fillAgentServiceEnterpriseMeta(&as, &s.EnterpriseMeta)
	return as
}

func (s *HTTPServer) AgentServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	var filterExpression string
	s.parseFilter(req, &filterExpression)

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &entMeta, nil)
	if err != nil {
		return nil, err
	}

	services := s.agent.State.Services(&entMeta)
	if err := s.agent.filterServicesWithAuthorizer(authz, &services); err != nil {
		return nil, err
	}

	// Convert into api.AgentService since that includes Connect config but so far
	// NodeService doesn't need to internally. They are otherwise identical since
	// that is the struct used in client for reading the one we output here
	// anyway.
	agentSvcs := make(map[string]*api.AgentService)

	// Use empty list instead of nil
	for id, s := range services {
		agentService := buildAgentService(s)
		agentSvcs[id.ID] = &agentService
	}

	filter, err := bexpr.CreateFilter(filterExpression, nil, agentSvcs)
	if err != nil {
		return nil, err
	}

	return filter.Execute(agentSvcs)
}

// GET /v1/agent/service/:service_id
//
// Returns the service definition for a single local services and allows
// blocking watch using hash-based blocking.
func (s *HTTPServer) AgentService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the proxy ID. Note that this is the ID of a proxy's service instance.
	id := strings.TrimPrefix(req.URL.Path, "/v1/agent/service/")

	// Maybe block
	var queryOpts structs.QueryOptions
	if parseWait(resp, req, &queryOpts) {
		// parseWait returns an error itself
		return nil, nil
	}

	// Parse the token
	var token string
	s.parseToken(req, &token)

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	// need to resolve to default the meta
	_, err := s.agent.resolveTokenAndDefaultMeta(token, &entMeta, nil)
	if err != nil {
		return nil, err
	}

	// Parse hash specially. Eventually this should happen in parseWait and end up
	// in QueryOptions but I didn't want to make very general changes right away.
	hash := req.URL.Query().Get("hash")

	sid := structs.NewServiceID(id, &entMeta)

	resultHash, service, err := s.agent.LocalBlockingQuery(false, hash, queryOpts.MaxQueryTime,
		func(ws memdb.WatchSet) (string, interface{}, error) {

			svcState := s.agent.State.ServiceState(sid)
			if svcState == nil {
				resp.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(resp, "unknown service ID: %s", sid.String())
				return "", nil, nil
			}

			svc := svcState.Service

			// Setup watch on the service
			ws.Add(svcState.WatchCh)

			// Check ACLs.
			authz, err := s.agent.resolveToken(token)
			if err != nil {
				return "", nil, err
			}
			var authzContext acl.AuthorizerContext
			svc.FillAuthzContext(&authzContext)
			if authz != nil && authz.ServiceRead(svc.Service, &authzContext) != acl.Allow {
				return "", nil, acl.ErrPermissionDenied
			}

			// Calculate the content hash over the response, minus the hash field
			aSvc := buildAgentService(svc)
			reply := &aSvc

			rawHash, err := hashstructure.Hash(reply, nil)
			if err != nil {
				return "", nil, err
			}

			// Include the ContentHash in the response body
			reply.ContentHash = fmt.Sprintf("%x", rawHash)

			return reply.ContentHash, reply, nil
		},
	)
	if resultHash != "" {
		resp.Header().Set("X-Consul-ContentHash", resultHash)
	}
	return service, err
}

func (s *HTTPServer) AgentChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &entMeta, nil)
	if err != nil {
		return nil, err
	}

	var filterExpression string
	s.parseFilter(req, &filterExpression)
	filter, err := bexpr.CreateFilter(filterExpression, nil, nil)
	if err != nil {
		return nil, err
	}

	checks := s.agent.State.Checks(&entMeta)
	if err := s.agent.filterChecksWithAuthorizer(authz, &checks); err != nil {
		return nil, err
	}

	agentChecks := make(map[types.CheckID]*structs.HealthCheck)

	// Use empty list instead of nil
	for id, c := range checks {
		if c.ServiceTags == nil {
			clone := *c
			clone.ServiceTags = make([]string, 0)
			agentChecks[id.ID] = &clone
		} else {
			agentChecks[id.ID] = c
		}
	}

	return filter.Execute(agentChecks)
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
	if rule != nil && rule.AgentWrite(s.agent.config.NodeName, nil) != acl.Allow {
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
		if s.agent.config.ConnectMeshGatewayWANFederationEnabled {
			return nil, fmt.Errorf("WAN join is disabled when wan federation via mesh gateways is enabled")
		}
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
	if rule != nil && rule.AgentWrite(s.agent.config.NodeName, nil) != acl.Allow {
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
	if rule != nil && rule.OperatorWrite(nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	//Check the value of the prune query
	_, prune := req.URL.Query()["prune"]

	addr := strings.TrimPrefix(req.URL.Path, "/v1/agent/force-leave/")
	return nil, s.agent.ForceLeave(addr, prune)
}

// syncChanges is a helper function which wraps a blocking call to sync
// services and checks to the server. If the operation fails, we only
// only warn because the write did succeed and anti-entropy will sync later.
func (s *HTTPServer) syncChanges() {
	if err := s.agent.State.SyncChanges(); err != nil {
		s.agent.logger.Error("failed to sync changes", "error", err)
	}
}

func (s *HTTPServer) AgentRegisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var token string
	s.parseToken(req, &token)

	var args structs.CheckDefinition
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	if err := decodeBody(req.Body, &args); err != nil {
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

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &args.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	// Construct the health check.
	health := args.HealthCheck(s.agent.config.NodeName)

	// Verify the check type.
	chkType := args.CheckType()
	err = chkType.Validate()
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, fmt.Errorf("Invalid check: %v", err))
		return nil, nil
	}

	// Store the type of check based on the definition
	health.Type = chkType.Type()

	if health.ServiceID != "" {
		// fixup the service name so that vetCheckRegister requires the right ACLs
		service := s.agent.State.Service(health.CompoundServiceID())
		if service != nil {
			health.ServiceName = service.Service
		}
	}

	// Get the provided token, if any, and vet against any ACL policies.
	if err := s.agent.vetCheckRegisterWithAuthorizer(authz, health); err != nil {
		return nil, err
	}

	// Add the check.
	if err := s.agent.AddCheck(health, chkType, true, token, ConfigSourceRemote); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentDeregisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := structs.NewCheckID(types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/deregister/")), nil)

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)

	if err := s.parseEntMetaNoWildcard(req, &checkID.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &checkID.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	checkID.Normalize()

	if err := s.agent.vetCheckUpdateWithAuthorizer(authz, checkID); err != nil {
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
	return s.agentCheckUpdate(resp, req, checkID, api.HealthPassing, note)
}

func (s *HTTPServer) AgentCheckWarn(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/warn/"))
	note := req.URL.Query().Get("note")

	return s.agentCheckUpdate(resp, req, checkID, api.HealthWarning, note)

}

func (s *HTTPServer) AgentCheckFail(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/fail/"))
	note := req.URL.Query().Get("note")

	return s.agentCheckUpdate(resp, req, checkID, api.HealthCritical, note)
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
	if err := decodeBody(req.Body, &update); err != nil {
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

	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/update/"))

	return s.agentCheckUpdate(resp, req, checkID, update.Status, update.Output)
}

func (s *HTTPServer) agentCheckUpdate(resp http.ResponseWriter, req *http.Request, checkID types.CheckID, status string, output string) (interface{}, error) {
	cid := structs.NewCheckID(checkID, nil)

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)

	if err := s.parseEntMetaNoWildcard(req, &cid.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &cid.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	cid.Normalize()

	if err := s.agent.vetCheckUpdateWithAuthorizer(authz, cid); err != nil {
		return nil, err
	}

	if err := s.agent.updateTTLCheck(cid, status, output); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

// agentHealthService Returns Health for a given service ID
func agentHealthService(serviceID structs.ServiceID, s *HTTPServer) (int, string, api.HealthChecks) {
	checks := s.agent.State.ChecksForService(serviceID, true)
	serviceChecks := make(api.HealthChecks, 0)
	for _, c := range checks {
		// TODO: harmonize struct.HealthCheck and api.HealthCheck (or at least extract conversion function)
		healthCheck := &api.HealthCheck{
			Node:        c.Node,
			CheckID:     string(c.CheckID),
			Name:        c.Name,
			Status:      c.Status,
			Notes:       c.Notes,
			Output:      c.Output,
			ServiceID:   c.ServiceID,
			ServiceName: c.ServiceName,
			ServiceTags: c.ServiceTags,
		}
		fillHealthCheckEnterpriseMeta(healthCheck, &c.EnterpriseMeta)
		serviceChecks = append(serviceChecks, healthCheck)
	}
	status := serviceChecks.AggregatedStatus()
	switch status {
	case api.HealthWarning:
		return http.StatusTooManyRequests, status, serviceChecks
	case api.HealthPassing:
		return http.StatusOK, status, serviceChecks
	default:
		return http.StatusServiceUnavailable, status, serviceChecks
	}
}

func returnTextPlain(req *http.Request) bool {
	if contentType := req.Header.Get("Accept"); strings.HasPrefix(contentType, "text/plain") {
		return true
	}
	if format := req.URL.Query().Get("format"); format != "" {
		return format == "text"
	}
	return false
}

// AgentHealthServiceByID return the local Service Health given its ID
func (s *HTTPServer) AgentHealthServiceByID(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Pull out the service id (service id since there may be several instance of the same service on this host)
	serviceID := strings.TrimPrefix(req.URL.Path, "/v1/agent/health/service/id/")
	if serviceID == "" {
		return nil, &BadRequestError{Reason: "Missing serviceID"}
	}

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	var token string
	s.parseToken(req, &token)

	// need to resolve to default the meta
	var authzContext acl.AuthorizerContext
	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &entMeta, &authzContext)
	if err != nil {
		return nil, err
	}

	sid := structs.NewServiceID(serviceID, &entMeta)

	if service := s.agent.State.Service(sid); service != nil {
		if authz != nil && authz.ServiceRead(service.Service, &authzContext) != acl.Allow {
			return nil, acl.ErrPermissionDenied
		}
		code, status, healthChecks := agentHealthService(sid, s)
		if returnTextPlain(req) {
			return status, CodeWithPayloadError{StatusCode: code, Reason: status, ContentType: "text/plain"}
		}
		serviceInfo := buildAgentService(service)
		result := &api.AgentServiceChecksInfo{
			AggregatedStatus: status,
			Checks:           healthChecks,
			Service:          &serviceInfo,
		}
		return result, CodeWithPayloadError{StatusCode: code, Reason: status, ContentType: "application/json"}
	}
	notFoundReason := fmt.Sprintf("ServiceId %s not found", sid.String())
	if returnTextPlain(req) {
		return notFoundReason, CodeWithPayloadError{StatusCode: http.StatusNotFound, Reason: notFoundReason, ContentType: "application/json"}
	}
	return &api.AgentServiceChecksInfo{
		AggregatedStatus: api.HealthCritical,
		Checks:           nil,
		Service:          nil,
	}, CodeWithPayloadError{StatusCode: http.StatusNotFound, Reason: notFoundReason, ContentType: "application/json"}
}

// AgentHealthServiceByName return the worse status of all the services with given name on an agent
func (s *HTTPServer) AgentHealthServiceByName(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Pull out the service name
	serviceName := strings.TrimPrefix(req.URL.Path, "/v1/agent/health/service/name/")
	if serviceName == "" {
		return nil, &BadRequestError{Reason: "Missing service Name"}
	}

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	var token string
	s.parseToken(req, &token)

	// need to resolve to default the meta
	var authzContext acl.AuthorizerContext
	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &entMeta, &authzContext)
	if err != nil {
		return nil, err
	}

	if authz != nil && authz.ServiceRead(serviceName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	code := http.StatusNotFound
	status := fmt.Sprintf("ServiceName %s Not Found", serviceName)
	services := s.agent.State.Services(&entMeta)
	result := make([]api.AgentServiceChecksInfo, 0, 16)
	for _, service := range services {
		if service.Service == serviceName {
			sid := structs.NewServiceID(service.ID, &entMeta)

			scode, sstatus, healthChecks := agentHealthService(sid, s)
			serviceInfo := buildAgentService(service)
			res := api.AgentServiceChecksInfo{
				AggregatedStatus: sstatus,
				Checks:           healthChecks,
				Service:          &serviceInfo,
			}
			result = append(result, res)
			// When service is not found, we ignore it and keep existing HTTP status
			if code == http.StatusNotFound {
				code = scode
				status = sstatus
			}
			// We take the worst of all statuses, so we keep iterating
			// passing: 200 < warning: 429 < critical: 503
			if code < scode {
				code = scode
				status = sstatus
			}
		}
	}
	if returnTextPlain(req) {
		return status, CodeWithPayloadError{StatusCode: code, Reason: status, ContentType: "text/plain"}
	}
	return result, CodeWithPayloadError{StatusCode: code, Reason: status, ContentType: "application/json"}
}

func (s *HTTPServer) AgentRegisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.ServiceDefinition
	// Fixup the type decode of TTL or Interval if a check if provided.

	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	if err := decodeBody(req.Body, &args); err != nil {
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

	var token string
	s.parseToken(req, &token)

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &args.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	// Get the node service.
	ns := args.NodeService()
	if ns.Weights != nil {
		if err := structs.ValidateWeights(ns.Weights); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, fmt.Errorf("Invalid Weights: %v", err))
			return nil, nil
		}
	}
	if err := structs.ValidateServiceMetadata(ns.Kind, ns.Meta, false); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, fmt.Errorf("Invalid Service Meta: %v", err))
		return nil, nil
	}

	// Run validation. This is the same validation that would happen on
	// the catalog endpoint so it helps ensure the sync will work properly.
	if err := ns.Validate(); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, err.Error())
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

	// Verify the sidecar check types
	if args.Connect != nil && args.Connect.SidecarService != nil {
		chkTypes, err := args.Connect.SidecarService.CheckTypes()
		if err != nil {
			return nil, &BadRequestError{
				Reason: fmt.Sprintf("Invalid check in sidecar_service: %v", err),
			}
		}
		for _, check := range chkTypes {
			if check.Status != "" && !structs.ValidStatus(check.Status) {
				return nil, &BadRequestError{
					Reason: "Status for checks must 'passing', 'warning', 'critical'",
				}
			}
		}
	}

	// Get the provided token, if any, and vet against any ACL policies.
	if err := s.agent.vetServiceRegisterWithAuthorizer(authz, ns); err != nil {
		return nil, err
	}

	// See if we have a sidecar to register too
	sidecar, sidecarChecks, sidecarToken, err := s.agent.sidecarServiceFromNodeService(ns, token)
	if err != nil {
		return nil, &BadRequestError{
			Reason: fmt.Sprintf("Invalid SidecarService: %s", err)}
	}
	if sidecar != nil {
		// Make sure we are allowed to register the sidecar using the token
		// specified (might be specific to sidecar or the same one as the overall
		// request).
		if err := s.agent.vetServiceRegister(sidecarToken, sidecar); err != nil {
			return nil, err
		}
		// We parsed the sidecar registration, now remove it from the NodeService
		// for the actual service since it's done it's job and we don't want to
		// persist it in the actual state/catalog. SidecarService is meant to be a
		// registration syntax sugar so don't propagate it any further.
		ns.Connect.SidecarService = nil
	}

	// Add the service.
	replaceExistingChecks := false

	query := req.URL.Query()
	if len(query["replace-existing-checks"]) > 0 && (query.Get("replace-existing-checks") == "" || query.Get("replace-existing-checks") == "true") {
		replaceExistingChecks = true
	}

	if replaceExistingChecks {
		if err := s.agent.AddServiceAndReplaceChecks(ns, chkTypes, true, token, ConfigSourceRemote); err != nil {
			return nil, err
		}
	} else {
		if err := s.agent.AddService(ns, chkTypes, true, token, ConfigSourceRemote); err != nil {
			return nil, err
		}
	}
	// Add sidecar.
	if sidecar != nil {
		if replaceExistingChecks {
			if err := s.agent.AddServiceAndReplaceChecks(sidecar, sidecarChecks, true, sidecarToken, ConfigSourceRemote); err != nil {
				return nil, err
			}
		} else {
			if err := s.agent.AddService(sidecar, sidecarChecks, true, sidecarToken, ConfigSourceRemote); err != nil {
				return nil, err
			}
		}
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentDeregisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	sid := structs.NewServiceID(strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/"), nil)

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)

	if err := s.parseEntMetaNoWildcard(req, &sid.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &sid.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	sid.Normalize()

	if err := s.agent.vetServiceUpdateWithAuthorizer(authz, sid); err != nil {
		return nil, err
	}

	if err := s.agent.RemoveService(sid); err != nil {
		return nil, err
	}

	s.syncChanges()
	return nil, nil
}

func (s *HTTPServer) AgentServiceMaintenance(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Ensure we have a service ID
	sid := structs.NewServiceID(strings.TrimPrefix(req.URL.Path, "/v1/agent/service/maintenance/"), nil)

	if sid.ID == "" {
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

	if err := s.parseEntMetaNoWildcard(req, &sid.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.resolveTokenAndDefaultMeta(token, &sid.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	sid.Normalize()

	if err := s.agent.vetServiceUpdateWithAuthorizer(authz, sid); err != nil {
		return nil, err
	}

	if enable {
		reason := params.Get("reason")
		if err = s.agent.EnableServiceMaintenance(sid, reason, token); err != nil {
			resp.WriteHeader(http.StatusNotFound)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
	} else {
		if err = s.agent.DisableServiceMaintenance(sid); err != nil {
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
	if rule != nil && rule.NodeWrite(s.agent.config.NodeName, nil) != acl.Allow {
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
	if rule != nil && rule.AgentRead(s.agent.config.NodeName, nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	// Get the provided loglevel.
	logLevel := req.URL.Query().Get("loglevel")
	if logLevel == "" {
		logLevel = "INFO"
	}

	var logJSON bool
	if _, ok := req.URL.Query()["logjson"]; ok {
		logJSON = true
	}

	if !logging.ValidateLogLevel(logLevel) {
		return nil, BadRequestError{
			Reason: fmt.Sprintf("Unknown log level: %s", logLevel),
		}
	}

	flusher, ok := resp.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("Streaming not supported")
	}

	monitor := monitor.New(monitor.Config{
		BufferSize: 512,
		Logger:     s.agent.logger,
		LoggerOptions: &hclog.LoggerOptions{
			Level:      logging.LevelFromString(logLevel),
			JSONFormat: logJSON,
		},
	})
	logsCh := monitor.Start()

	// Send header so client can start streaming body
	resp.WriteHeader(http.StatusOK)

	// 0 byte write is needed before the Flush call so that if we are using
	// a gzip stream it will go ahead and write out the HTTP response header
	resp.Write([]byte(""))
	flusher.Flush()

	// Stream logs until the connection is closed.
	for {
		select {
		case <-req.Context().Done():
			droppedCount := monitor.Stop()
			if droppedCount > 0 {
				s.agent.logger.Warn("Dropped logs during monitor request", "dropped_count", droppedCount)
			}
			return nil, nil
		case log := <-logsCh:
			fmt.Fprint(resp, string(log))
			flusher.Flush()
		}
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
	if rule != nil && rule.AgentWrite(s.agent.config.NodeName, nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	// The body is just the token, but it's in a JSON object so we can add
	// fields to this later if needed.
	var args api.AgentToken
	if err := decodeBody(req.Body, &args); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	if s.agent.config.ACLEnableTokenPersistence {
		// we hold the lock around updating the internal token store
		// as well as persisting the tokens because we don't want to write
		// into the store to have something else wipe it out before we can
		// persist everything (like an agent config reload). The token store
		// lock is only held for those operations so other go routines that
		// just need to read some token out of the store will not be impacted
		// any more than they would be without token persistence.
		s.agent.persistedTokensLock.Lock()
		defer s.agent.persistedTokensLock.Unlock()
	}

	// Figure out the target token.
	target := strings.TrimPrefix(req.URL.Path, "/v1/agent/token/")
	triggerAntiEntropySync := false
	switch target {
	case "acl_token", "default":
		changed := s.agent.tokens.UpdateUserToken(args.Token, token_store.TokenSourceAPI)
		if changed {
			triggerAntiEntropySync = true
		}

	case "acl_agent_token", "agent":
		changed := s.agent.tokens.UpdateAgentToken(args.Token, token_store.TokenSourceAPI)
		if changed {
			triggerAntiEntropySync = true
		}

	case "acl_agent_master_token", "agent_master":
		s.agent.tokens.UpdateAgentMasterToken(args.Token, token_store.TokenSourceAPI)

	case "acl_replication_token", "replication":
		s.agent.tokens.UpdateReplicationToken(args.Token, token_store.TokenSourceAPI)

	default:
		resp.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(resp, "Token %q is unknown", target)
		return nil, nil
	}

	if triggerAntiEntropySync {
		s.agent.sync.SyncFull.Trigger()
	}

	if s.agent.config.ACLEnableTokenPersistence {
		tokens := persistedTokens{}

		if tok, source := s.agent.tokens.UserTokenAndSource(); tok != "" && source == token_store.TokenSourceAPI {
			tokens.Default = tok
		}

		if tok, source := s.agent.tokens.AgentTokenAndSource(); tok != "" && source == token_store.TokenSourceAPI {
			tokens.Agent = tok
		}

		if tok, source := s.agent.tokens.AgentMasterTokenAndSource(); tok != "" && source == token_store.TokenSourceAPI {
			tokens.AgentMaster = tok
		}

		if tok, source := s.agent.tokens.ReplicationTokenAndSource(); tok != "" && source == token_store.TokenSourceAPI {
			tokens.Replication = tok
		}

		data, err := json.Marshal(tokens)
		if err != nil {
			s.agent.logger.Warn("failed to persist tokens", "error", err)
			return nil, fmt.Errorf("Failed to marshal tokens for persistence: %v", err)
		}

		if err := file.WriteAtomicWithPerms(filepath.Join(s.agent.config.DataDir, tokensPath), data, 0700, 0600); err != nil {
			s.agent.logger.Warn("failed to persist tokens", "error", err)
			return nil, fmt.Errorf("Failed to persist tokens - %v", err)
		}
	}

	s.agent.logger.Info("Updated agent's ACL token", "token", target)
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
	// Get the service name. Note that this is the name of the service,
	// not the ID of the service instance.
	serviceName := strings.TrimPrefix(req.URL.Path, "/v1/agent/connect/ca/leaf/")

	args := cachetype.ConnectCALeafRequest{
		Service: serviceName, // Need name not ID
	}
	var qOpts structs.QueryOptions

	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Store DC in the ConnectCALeafRequest but query opts separately
	if done := s.parse(resp, req, &args.Datacenter, &qOpts); done {
		return nil, nil
	}
	args.MinQueryIndex = qOpts.MinQueryIndex
	args.MaxQueryTime = qOpts.MaxQueryTime
	args.Token = qOpts.Token

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

	var authReq structs.ConnectAuthorizeRequest

	if err := s.parseEntMetaNoWildcard(req, &authReq.EnterpriseMeta); err != nil {
		return nil, err
	}

	if err := decodeBody(req.Body, &authReq); err != nil {
		return nil, BadRequestError{fmt.Sprintf("Request decode failed: %v", err)}
	}

	authz, reason, cacheMeta, err := s.agent.ConnectAuthorize(token, &authReq)
	if err != nil {
		return nil, err
	}
	setCacheMeta(resp, cacheMeta)

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

// AgentHost
//
// GET /v1/agent/host
//
// Retrieves information about resources available and in-use for the
// host the agent is running on such as CPU, memory, and disk usage. Requires
// a operator:read ACL token.
func (s *HTTPServer) AgentHost(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	rule, err := s.agent.resolveToken(token)
	if err != nil {
		return nil, err
	}

	if rule != nil && rule.OperatorRead(nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	return debug.CollectHostInfo(), nil
}
