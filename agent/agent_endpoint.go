package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/debug"
	"github.com/hashicorp/consul/agent/structs"
	token_store "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/logging/monitor"
	"github.com/hashicorp/consul/types"
)

type Self struct {
	Config      interface{}
	DebugConfig map[string]interface{}
	Coord       *coordinate.Coordinate
	Member      serf.Member
	Stats       map[string]map[string]string
	Meta        map[string]string
	XDS         *XDSSelf `json:"xDS,omitempty"`
}

type XDSSelf struct {
	SupportedProxies map[string][]string
	Port             int
}

func (s *HTTPHandlers) AgentSelf(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentRead(s.agent.config.NodeName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	var cs lib.CoordinateSet
	if !s.agent.config.DisableCoordinates {
		var err error
		if cs, err = s.agent.GetLANCoordinate(); err != nil {
			return nil, err
		}
	}

	var xds *XDSSelf
	if s.agent.grpcServer != nil {
		xds = &XDSSelf{
			SupportedProxies: map[string][]string{
				"envoy": proxysupport.EnvoyVersions,
			},
			Port: s.agent.config.GRPCPort,
		}
	}

	config := struct {
		Datacenter        string
		PrimaryDatacenter string
		NodeName          string
		NodeID            string
		Partition         string `json:",omitempty"`
		Revision          string
		Server            bool
		Version           string
	}{
		Datacenter:        s.agent.config.Datacenter,
		PrimaryDatacenter: s.agent.config.PrimaryDatacenter,
		NodeName:          s.agent.config.NodeName,
		NodeID:            string(s.agent.config.NodeID),
		Partition:         s.agent.config.PartitionOrEmpty(),
		Revision:          s.agent.config.Revision,
		Server:            s.agent.config.ServerMode,
		Version:           s.agent.config.Version,
	}
	return Self{
		Config:      config,
		DebugConfig: s.agent.config.Sanitized(),
		Coord:       cs[s.agent.config.SegmentName],
		Member:      s.agent.AgentLocalMember(),
		Stats:       s.agent.Stats(),
		Meta:        s.agent.State.Metadata(),
		XDS:         xds,
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

func (s *HTTPHandlers) AgentMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentRead(s.agent.config.NodeName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}
	if enablePrometheusOutput(req) {
		if s.agent.config.Telemetry.PrometheusOpts.Expiration < 1 {
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
	return s.agent.baseDeps.MetricsHandler.DisplayMetrics(resp, req)
}

func (s *HTTPHandlers) AgentMetricsStream(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentRead(s.agent.config.NodeName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	flusher, ok := resp.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	resp.WriteHeader(http.StatusOK)

	// 0 byte write is needed before the Flush call so that if we are using
	// a gzip stream it will go ahead and write out the HTTP response header
	resp.Write([]byte(""))
	flusher.Flush()

	enc := metricsEncoder{
		logger:  s.agent.logger,
		encoder: json.NewEncoder(resp),
		flusher: flusher,
	}
	enc.encoder.SetIndent("", "    ")
	s.agent.baseDeps.MetricsHandler.Stream(req.Context(), enc)
	return nil, nil
}

type metricsEncoder struct {
	logger  hclog.Logger
	encoder *json.Encoder
	flusher http.Flusher
}

func (m metricsEncoder) Encode(summary interface{}) error {
	if err := m.encoder.Encode(summary); err != nil {
		m.logger.Error("failed to encode metrics summary", "error", err)
		return err
	}
	m.flusher.Flush()
	return nil
}

func (s *HTTPHandlers) AgentReload(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentWrite(s.agent.config.NodeName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	return nil, s.agent.ReloadConfig()
}

func buildAgentService(s *structs.NodeService, dc string) api.AgentService {
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
		SocketPath:        s.SocketPath,
		TaggedAddresses:   taggedAddrs,
		EnableTagOverride: s.EnableTagOverride,
		CreateIndex:       s.CreateIndex,
		ModifyIndex:       s.ModifyIndex,
		Weights:           weights,
		Datacenter:        dc,
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

func (s *HTTPHandlers) AgentServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	// TODO(partitions): should this default to the agent's partition?
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	var filterExpression string
	s.parseFilter(req, &filterExpression)

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &entMeta, nil)
	if err != nil {
		return nil, err
	}

	if !s.validateRequestPartition(resp, &entMeta) {
		return nil, nil
	}

	// NOTE: we're explicitly fetching things in the requested partition and
	// namespace here.
	services := s.agent.State.Services(&entMeta)
	if err := s.agent.filterServicesWithAuthorizer(authz, &services); err != nil {
		return nil, err
	}

	// Convert into api.AgentService since that includes Connect config but so far
	// NodeService doesn't need to internally. They are otherwise identical since
	// that is the struct used in client for reading the one we output here
	// anyway.
	agentSvcs := make(map[string]*api.AgentService)

	dc := s.agent.config.Datacenter

	// Use empty list instead of nil
	for id, s := range services {
		agentService := buildAgentService(s, dc)
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
func (s *HTTPHandlers) AgentService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

	// TODO(partitions): should this default to the agent's partition?
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	// need to resolve to default the meta
	_, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &entMeta, nil)
	if err != nil {
		return nil, err
	}

	// Parse hash specially. Eventually this should happen in parseWait and end up
	// in QueryOptions but I didn't want to make very general changes right away.
	hash := req.URL.Query().Get("hash")

	sid := structs.NewServiceID(id, &entMeta)

	if !s.validateRequestPartition(resp, &entMeta) {
		return nil, nil
	}

	dc := s.agent.config.Datacenter

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
			authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
			if err != nil {
				return "", nil, err
			}
			var authzContext acl.AuthorizerContext
			svc.FillAuthzContext(&authzContext)
			if authz.ServiceRead(svc.Service, &authzContext) != acl.Allow {
				return "", nil, acl.ErrPermissionDenied
			}

			// Calculate the content hash over the response, minus the hash field
			aSvc := buildAgentService(svc, dc)
			reply := &aSvc

			// TODO(partitions): do we need to do anything here?
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

func (s *HTTPHandlers) AgentChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any.
	var token string
	s.parseToken(req, &token)

	// TODO(partitions): should this default to the agent's partition?
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &entMeta, nil)
	if err != nil {
		return nil, err
	}

	if !s.validateRequestPartition(resp, &entMeta) {
		return nil, nil
	}

	var filterExpression string
	s.parseFilter(req, &filterExpression)
	filter, err := bexpr.CreateFilter(filterExpression, nil, nil)
	if err != nil {
		return nil, err
	}

	// NOTE(partitions): this works because nodes exist in ONE partition
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

func (s *HTTPHandlers) AgentMembers(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

	// Get the request partition and default to that of the agent.
	entMeta := s.agent.AgentEnterpriseMeta()
	if err := s.parseEntMetaPartition(req, entMeta); err != nil {
		return nil, err
	}

	var members []serf.Member
	if wan {
		members = s.agent.WANMembers()
	} else {
		filter := consul.LANMemberFilter{
			Partition: entMeta.PartitionOrDefault(),
		}
		if segment == api.AllSegments {
			// Older 'consul members' calls will default to adding segment=_all
			// so we only choose to use that request argument in the case where
			// the partition is also the default and ignore it the rest of the time.
			if structs.IsDefaultPartition(filter.Partition) {
				filter.AllSegments = true
			}
		} else {
			filter.Segment = segment
		}
		var err error
		members, err = s.agent.delegate.LANMembers(filter)
		if err != nil {
			return nil, err
		}
	}
	if err := s.agent.filterMembers(token, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (s *HTTPHandlers) AgentJoin(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentWrite(s.agent.config.NodeName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	// Get the request partition and default to that of the agent.
	entMeta := s.agent.AgentEnterpriseMeta()
	if err := s.parseEntMetaPartition(req, entMeta); err != nil {
		return nil, err
	}

	// Check if the WAN is being queried
	wan := false
	if other := req.URL.Query().Get("wan"); other != "" {
		wan = true
		// TODO(partitions) : block wan join
	}

	// Get the address
	addr := strings.TrimPrefix(req.URL.Path, "/v1/agent/join/")

	if wan {
		if s.agent.config.ConnectMeshGatewayWANFederationEnabled {
			return nil, fmt.Errorf("WAN join is disabled when wan federation via mesh gateways is enabled")
		}
		_, err = s.agent.JoinWAN([]string{addr})
	} else {
		_, err = s.agent.JoinLAN([]string{addr}, entMeta)
	}
	return nil, err
}

func (s *HTTPHandlers) AgentLeave(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentWrite(s.agent.config.NodeName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	if err := s.agent.Leave(); err != nil {
		return nil, err
	}
	return nil, s.agent.ShutdownAgent()
}

func (s *HTTPHandlers) AgentForceLeave(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}
	// TODO(partitions): should this be possible in a partition?
	if authz.OperatorWrite(nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	// Get the request partition and default to that of the agent.
	entMeta := s.agent.AgentEnterpriseMeta()
	if err := s.parseEntMetaPartition(req, entMeta); err != nil {
		return nil, err
	}

	// Check the value of the prune query
	_, prune := req.URL.Query()["prune"]

	addr := strings.TrimPrefix(req.URL.Path, "/v1/agent/force-leave/")
	return nil, s.agent.ForceLeave(addr, prune, entMeta)
}

// syncChanges is a helper function which wraps a blocking call to sync
// services and checks to the server. If the operation fails, we only
// only warn because the write did succeed and anti-entropy will sync later.
func (s *HTTPHandlers) syncChanges() {
	if err := s.agent.State.SyncChanges(); err != nil {
		s.agent.logger.Error("failed to sync changes", "error", err)
	}
}

func (s *HTTPHandlers) AgentRegisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var token string
	s.parseToken(req, &token)

	// TODO(partitions): should this default to the agent's partition?
	var args structs.CheckDefinition
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	if err := decodeBody(req.Body, &args); err != nil {
		return nil, BadRequestError{fmt.Sprintf("Request decode failed: %v", err)}
	}

	// Verify the check has a name.
	if args.Name == "" {
		return nil, BadRequestError{"Missing check name"}
	}

	if args.Status != "" && !structs.ValidStatus(args.Status) {
		return nil, BadRequestError{"Bad check status"}
	}

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &args.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	if !s.validateRequestPartition(resp, &args.EnterpriseMeta) {
		return nil, nil
	}

	// Construct the health check.
	health := args.HealthCheck(s.agent.config.NodeName)

	// Verify the check type.
	chkType := args.CheckType()
	err = chkType.Validate()
	if err != nil {
		return nil, BadRequestError{fmt.Sprintf("Invalid check: %v", err)}
	}

	// Store the type of check based on the definition
	health.Type = chkType.Type()

	if health.ServiceID != "" {
		cid := health.CompoundServiceID()
		// fixup the service name so that vetCheckRegister requires the right ACLs
		service := s.agent.State.Service(cid)
		if service != nil {
			health.ServiceName = service.Service
		} else {
			return nil, NotFoundError{fmt.Sprintf("ServiceID %q does not exist", cid.String())}
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

func (s *HTTPHandlers) AgentDeregisterCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := structs.NewCheckID(types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/deregister/")), nil)

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)

	// TODO(partitions): should this default to the agent's partition?
	if err := s.parseEntMetaNoWildcard(req, &checkID.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &checkID.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	checkID.Normalize()

	if !s.validateRequestPartition(resp, &checkID.EnterpriseMeta) {
		return nil, nil
	}

	if err := s.agent.vetCheckUpdateWithAuthorizer(authz, checkID); err != nil {
		return nil, err
	}

	if err := s.agent.RemoveCheck(checkID, true); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPHandlers) AgentCheckPass(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/pass/"))
	note := req.URL.Query().Get("note")
	return s.agentCheckUpdate(resp, req, checkID, api.HealthPassing, note)
}

func (s *HTTPHandlers) AgentCheckWarn(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	checkID := types.CheckID(strings.TrimPrefix(req.URL.Path, "/v1/agent/check/warn/"))
	note := req.URL.Query().Get("note")

	return s.agentCheckUpdate(resp, req, checkID, api.HealthWarning, note)

}

func (s *HTTPHandlers) AgentCheckFail(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
func (s *HTTPHandlers) AgentCheckUpdate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

func (s *HTTPHandlers) agentCheckUpdate(resp http.ResponseWriter, req *http.Request, checkID types.CheckID, status string, output string) (interface{}, error) {
	cid := structs.NewCheckID(checkID, nil)

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)

	// TODO(partitions): should this default to the agent's partition?
	if err := s.parseEntMetaNoWildcard(req, &cid.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &cid.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	cid.Normalize()

	if err := s.agent.vetCheckUpdateWithAuthorizer(authz, cid); err != nil {
		return nil, err
	}

	if !s.validateRequestPartition(resp, &cid.EnterpriseMeta) {
		return nil, nil
	}

	if err := s.agent.updateTTLCheck(cid, status, output); err != nil {
		return nil, err
	}
	s.syncChanges()
	return nil, nil
}

// agentHealthService Returns Health for a given service ID
func agentHealthService(serviceID structs.ServiceID, s *HTTPHandlers) (int, string, api.HealthChecks) {
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
func (s *HTTPHandlers) AgentHealthServiceByID(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Pull out the service id (service id since there may be several instance of the same service on this host)
	serviceID := strings.TrimPrefix(req.URL.Path, "/v1/agent/health/service/id/")
	if serviceID == "" {
		return nil, &BadRequestError{Reason: "Missing serviceID"}
	}

	// TODO(partitions): should this default to the agent's partition?
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	var token string
	s.parseToken(req, &token)

	// need to resolve to default the meta
	var authzContext acl.AuthorizerContext
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &entMeta, &authzContext)
	if err != nil {
		return nil, err
	}

	if !s.validateRequestPartition(resp, &entMeta) {
		return nil, nil
	}

	sid := structs.NewServiceID(serviceID, &entMeta)

	dc := s.agent.config.Datacenter

	if service := s.agent.State.Service(sid); service != nil {
		if authz.ServiceRead(service.Service, &authzContext) != acl.Allow {
			return nil, acl.ErrPermissionDenied
		}
		code, status, healthChecks := agentHealthService(sid, s)
		if returnTextPlain(req) {
			return status, CodeWithPayloadError{StatusCode: code, Reason: status, ContentType: "text/plain"}
		}
		serviceInfo := buildAgentService(service, dc)
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
func (s *HTTPHandlers) AgentHealthServiceByName(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Pull out the service name
	serviceName := strings.TrimPrefix(req.URL.Path, "/v1/agent/health/service/name/")
	if serviceName == "" {
		return nil, &BadRequestError{Reason: "Missing service Name"}
	}

	// TODO(partitions): should this default to the agent's partition?
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	var token string
	s.parseToken(req, &token)

	// need to resolve to default the meta
	var authzContext acl.AuthorizerContext
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &entMeta, &authzContext)
	if err != nil {
		return nil, err
	}

	if authz.ServiceRead(serviceName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	if !s.validateRequestPartition(resp, &entMeta) {
		return nil, nil
	}

	dc := s.agent.config.Datacenter

	code := http.StatusNotFound
	status := fmt.Sprintf("ServiceName %s Not Found", serviceName)

	services := s.agent.State.ServicesByName(structs.NewServiceName(serviceName, &entMeta))
	result := make([]api.AgentServiceChecksInfo, 0, 16)
	for _, service := range services {
		sid := structs.NewServiceID(service.ID, &entMeta)

		scode, sstatus, healthChecks := agentHealthService(sid, s)
		serviceInfo := buildAgentService(service, dc)
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
	if returnTextPlain(req) {
		return status, CodeWithPayloadError{StatusCode: code, Reason: status, ContentType: "text/plain"}
	}
	return result, CodeWithPayloadError{StatusCode: code, Reason: status, ContentType: "application/json"}
}

func (s *HTTPHandlers) AgentRegisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.ServiceDefinition
	// Fixup the type decode of TTL or Interval if a check if provided.

	// TODO(partitions): should this default to the agent's partition?
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

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &args.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	if !s.validateRequestPartition(resp, &args.EnterpriseMeta) {
		return nil, nil
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
		if err := sidecar.Validate(); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
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

	addReq := AddServiceRequest{
		Service:               ns,
		chkTypes:              chkTypes,
		persist:               true,
		token:                 token,
		Source:                ConfigSourceRemote,
		replaceExistingChecks: replaceExistingChecks,
	}
	if err := s.agent.AddService(addReq); err != nil {
		return nil, err
	}

	if sidecar != nil {
		addReq := AddServiceRequest{
			Service:               sidecar,
			chkTypes:              sidecarChecks,
			persist:               true,
			token:                 sidecarToken,
			Source:                ConfigSourceRemote,
			replaceExistingChecks: replaceExistingChecks,
		}
		if err := s.agent.AddService(addReq); err != nil {
			return nil, err
		}
	}
	s.syncChanges()
	return nil, nil
}

func (s *HTTPHandlers) AgentDeregisterService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	sid := structs.NewServiceID(strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/"), nil)

	// Get the provided token, if any, and vet against any ACL policies.
	var token string
	s.parseToken(req, &token)

	// TODO(partitions): should this default to the agent's partition?
	if err := s.parseEntMetaNoWildcard(req, &sid.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &sid.EnterpriseMeta, nil)
	if err != nil {
		return nil, err
	}

	sid.Normalize()

	if !s.validateRequestPartition(resp, &sid.EnterpriseMeta) {
		return nil, nil
	}

	if err := s.agent.vetServiceUpdateWithAuthorizer(authz, sid); err != nil {
		return nil, err
	}

	if err := s.agent.RemoveService(sid); err != nil {
		return nil, err
	}

	s.syncChanges()
	return nil, nil
}

func (s *HTTPHandlers) AgentServiceMaintenance(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

	// TODO(partitions): should this default to the agent's partition?
	if err := s.parseEntMetaNoWildcard(req, &sid.EnterpriseMeta); err != nil {
		return nil, err
	}

	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &sid.EnterpriseMeta, nil)
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

func (s *HTTPHandlers) AgentNodeMaintenance(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.NodeWrite(s.agent.config.NodeName, &authzContext) != acl.Allow {
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

func (s *HTTPHandlers) AgentMonitor(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentRead(s.agent.config.NodeName, &authzContext) != acl.Allow {
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
	const flushDelay = 200 * time.Millisecond
	flushTicker := time.NewTicker(flushDelay)
	defer flushTicker.Stop()

	// Stream logs until the connection is closed.
	for {
		select {
		case <-req.Context().Done():
			droppedCount := monitor.Stop()
			if droppedCount > 0 {
				s.agent.logger.Warn("Dropped logs during monitor request", "dropped_count", droppedCount)
			}
			flusher.Flush()
			return nil, nil

		case log := <-logsCh:
			fmt.Fprint(resp, string(log))

		case <-flushTicker.C:
			flusher.Flush()
		}
	}
}

func (s *HTTPHandlers) AgentToken(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkACLDisabled(resp, req) {
		return nil, nil
	}

	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// Authorize using the agent's own enterprise meta, not the token.
	var authzContext acl.AuthorizerContext
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if authz.AgentWrite(s.agent.config.NodeName, &authzContext) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	// The body is just the token, but it's in a JSON object so we can add
	// fields to this later if needed.
	var args api.AgentToken
	if err := decodeBody(req.Body, &args); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Request decode failed: %v", err)}
	}

	// Figure out the target token.
	target := strings.TrimPrefix(req.URL.Path, "/v1/agent/token/")

	err = s.agent.tokens.WithPersistenceLock(func() error {
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
			return NotFoundError{Reason: fmt.Sprintf("Token %q is unknown", target)}
		}

		// TODO: is it safe to move this out of WithPersistenceLock?
		if triggerAntiEntropySync {
			s.agent.sync.SyncFull.Trigger()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.agent.logger.Info("Updated agent's ACL token", "token", target)
	return nil, nil
}

// AgentConnectCARoots returns the trusted CA roots.
func (s *HTTPHandlers) AgentConnectCARoots(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	raw, m, err := s.agent.cache.Get(req.Context(), cachetype.ConnectCARootName, &args)
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
func (s *HTTPHandlers) AgentConnectCALeafCert(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the service name. Note that this is the name of the service,
	// not the ID of the service instance.
	serviceName := strings.TrimPrefix(req.URL.Path, "/v1/agent/connect/ca/leaf/")

	args := cachetype.ConnectCALeafRequest{
		Service: serviceName, // Need name not ID
	}
	var qOpts structs.QueryOptions

	// TODO(partitions): should this default to the agent's partition?
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

	if !s.validateRequestPartition(resp, &args.EnterpriseMeta) {
		return nil, nil
	}

	raw, m, err := s.agent.cache.Get(req.Context(), cachetype.ConnectCALeafName, &args)
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
// NOTE: This endpoint treats any L7 intentions as DENY.
//
// Note: when this logic changes, consider if the Intention.Check RPC method
// also needs to be updated.
func (s *HTTPHandlers) AgentConnectAuthorize(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the token
	var token string
	s.parseToken(req, &token)

	var authReq structs.ConnectAuthorizeRequest

	// TODO(partitions): should this default to the agent's partition?
	if err := s.parseEntMetaNoWildcard(req, &authReq.EnterpriseMeta); err != nil {
		return nil, err
	}

	if err := decodeBody(req.Body, &authReq); err != nil {
		return nil, BadRequestError{fmt.Sprintf("Request decode failed: %v", err)}
	}

	if !s.validateRequestPartition(resp, &authReq.EnterpriseMeta) {
		return nil, nil
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
func (s *HTTPHandlers) AgentHost(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Fetch the ACL token, if any, and enforce agent policy.
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	// TODO(partitions): should this be possible in a partition?
	if authz.OperatorRead(nil) != acl.Allow {
		return nil, acl.ErrPermissionDenied
	}

	return debug.CollectHostInfo(), nil
}
