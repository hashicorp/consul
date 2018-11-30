package agent

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
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
	"github.com/hashicorp/consul/agent/debug"
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
			fmt.Fprint(resp, "Prometheus is not enabled since its retention time is not positive")
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
		weights := api.AgentWeights{Passing: 1, Warning: 1}
		if s.Weights != nil {
			if s.Weights.Passing > 0 {
				weights.Passing = s.Weights.Passing
			}
			weights.Warning = s.Weights.Warning
		}
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
			Weights:           weights,
		}

		if as.Tags == nil {
			as.Tags = []string{}
		}
		if as.Meta == nil {
			as.Meta = map[string]string{}
		}
		// Attach Unmanaged Proxy config if exists
		if s.Kind == structs.ServiceKindConnectProxy {
			as.Proxy = s.Proxy.ToAPI()
			// DEPRECATED (ProxyDestination) - remove this when removing ProxyDestination
			// Also set the deprecated ProxyDestination
			as.ProxyDestination = as.Proxy.DestinationServiceName
		}

		// Attach Connect configs if they exist. We use the actual proxy state since
		// that may have had defaults filled in compared to the config that was
		// provided with the service as stored in the NodeService here.
		if proxy, ok := proxies[id+"-proxy"]; ok {
			as.Connect = &api.AgentServiceConnect{
				Proxy: &api.AgentServiceConnectProxy{
					ExecMode:  api.ProxyExecMode(proxy.Proxy.ExecMode.String()),
					Command:   proxy.Proxy.Command,
					Config:    proxy.Proxy.Config,
					Upstreams: proxy.Proxy.Upstreams.ToAPI(),
				},
			}
		} else if s.Connect.Native {
			as.Connect = &api.AgentServiceConnect{
				Native: true,
			}
		}
		agentSvcs[id] = as
	}

	return agentSvcs, nil
}

// GET /v1/agent/service/:service_id
//
// Returns the service definition for a single local services and allows
// blocking watch using hash-based blocking.
func (s *HTTPServer) AgentService(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the proxy ID. Note that this is the ID of a proxy's service instance.
	id := strings.TrimPrefix(req.URL.Path, "/v1/agent/service/")

	// DEPRECATED(managed-proxies) - remove this whole hack.
	//
	// Support managed proxies until they are removed entirely. Since built-in
	// proxy will now use this endpoint, in order to not break managed proxies in
	// the interim until they are removed, we need to mirror the default-setting
	// behaviour they had. Rather than thread that through this whole method as
	// special cases that need to be unwound later (and duplicate logic in the
	// proxy config endpoint) just defer to that and then translater the response.
	if managedProxy := s.agent.State.Proxy(id); managedProxy != nil {
		// This is for a managed proxy, use the old endpoint's behaviour
		req.URL.Path = "/v1/agent/connect/proxy/" + id
		obj, err := s.AgentConnectProxyConfig(resp, req)
		if err != nil {
			return obj, err
		}
		proxyCfg, ok := obj.(*api.ConnectProxyConfig)
		if !ok {
			return nil, errors.New("internal error")
		}
		// These are all set by defaults so type checks are just sanity checks that
		// should never fail.
		port, ok := proxyCfg.Config["bind_port"].(int)
		if !ok || port < 1 {
			return nil, errors.New("invalid proxy config")
		}
		addr, ok := proxyCfg.Config["bind_address"].(string)
		if !ok || addr == "" {
			return nil, errors.New("invalid proxy config")
		}
		localAddr, ok := proxyCfg.Config["local_service_address"].(string)
		if !ok || localAddr == "" {
			return nil, errors.New("invalid proxy config")
		}
		// Old local_service_address was a host:port
		localAddress, localPortRaw, err := net.SplitHostPort(localAddr)
		if err != nil {
			return nil, err
		}
		localPort, err := strconv.Atoi(localPortRaw)
		if err != nil {
			return nil, err
		}

		reply := &api.AgentService{
			Kind:        api.ServiceKindConnectProxy,
			ID:          proxyCfg.ProxyServiceID,
			Service:     managedProxy.Proxy.ProxyService.Service,
			Port:        port,
			Address:     addr,
			ContentHash: proxyCfg.ContentHash,
			Proxy: &api.AgentServiceConnectProxyConfig{
				DestinationServiceName: proxyCfg.TargetServiceName,
				DestinationServiceID:   proxyCfg.TargetServiceID,
				LocalServiceAddress:    localAddress,
				LocalServicePort:       localPort,
				Config:                 proxyCfg.Config,
				Upstreams:              proxyCfg.Upstreams,
			},
		}
		return reply, nil
	}

	// Maybe block
	var queryOpts structs.QueryOptions
	if parseWait(resp, req, &queryOpts) {
		// parseWait returns an error itself
		return nil, nil
	}

	// Parse the token
	var token string
	s.parseToken(req, &token)

	// Parse hash specially. Eventually this should happen in parseWait and end up
	// in QueryOptions but I didn't want to make very general changes right away.
	hash := req.URL.Query().Get("hash")

	return s.agentLocalBlockingQuery(resp, hash, &queryOpts,
		func(ws memdb.WatchSet) (string, interface{}, error) {

			svcState := s.agent.State.ServiceState(id)
			if svcState == nil {
				resp.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(resp, "unknown proxy service ID: %s", id)
				return "", nil, nil
			}

			svc := svcState.Service

			// Setup watch on the service
			ws.Add(svcState.WatchCh)

			// Check ACLs.
			rule, err := s.agent.resolveToken(token)
			if err != nil {
				return "", nil, err
			}
			if rule != nil && !rule.ServiceRead(svc.Service) {
				return "", nil, acl.ErrPermissionDenied
			}

			var connect *api.AgentServiceConnect
			var proxy *api.AgentServiceConnectProxyConfig

			if svc.Connect.Native {
				connect = &api.AgentServiceConnect{
					Native: svc.Connect.Native,
				}
			}

			if svc.Kind == structs.ServiceKindConnectProxy {
				proxy = svc.Proxy.ToAPI()
			}

			var weights api.AgentWeights
			if svc.Weights != nil {
				err := mapstructure.Decode(svc.Weights, &weights)
				if err != nil {
					return "", nil, err
				}
			}

			// Calculate the content hash over the response, minus the hash field
			reply := &api.AgentService{
				Kind:              api.ServiceKind(svc.Kind),
				ID:                svc.ID,
				Service:           svc.Service,
				Tags:              svc.Tags,
				Meta:              svc.Meta,
				Port:              svc.Port,
				Address:           svc.Address,
				EnableTagOverride: svc.EnableTagOverride,
				Weights:           weights,
				Proxy:             proxy,
				Connect:           connect,
			}

			rawHash, err := hashstructure.Hash(reply, nil)
			if err != nil {
				return "", nil, err
			}

			// Include the ContentHash in the response body
			reply.ContentHash = fmt.Sprintf("%x", rawHash)

			return reply.ContentHash, reply, nil
		})
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
	if err := s.agent.AddCheck(health, chkType, true, token, ConfigSourceRemote); err != nil {
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
			// Managed Proxy Config
			"exec_mode": "ExecMode",
			// Proxy Upstreams
			"destination_name":      "DestinationName",
			"destination_type":      "DestinationType",
			"destination_namespace": "DestinationNamespace",
			"local_bind_port":       "LocalBindPort",
			"local_bind_address":    "LocalBindAddress",
			// Proxy Config
			"destination_service_name": "DestinationServiceName",
			"destination_service_id":   "DestinationServiceID",
			"local_service_port":       "LocalServicePort",
			"local_service_address":    "LocalServiceAddress",
			// SidecarService
			"sidecar_service": "SidecarService",

			// DON'T Recurse into these opaque config maps or we might mangle user's
			// keys. Note empty canonical is a special sentinel to prevent recursion.
			"Meta": "",
			// upstreams is an array but this prevents recursion into config field of
			// any item in the array.
			"Proxy.Config":                   "",
			"Proxy.Upstreams.Config":         "",
			"Connect.Proxy.Config":           "",
			"Connect.Proxy.Upstreams.Config": "",

			// Same exceptions as above, but for a nested sidecar_service note we use
			// the canonical form SidecarService since that is translated by the time
			// the lookup here happens. Note that sidecar service doesn't support
			// managed proxies (connect.proxy).
			"Connect.SidecarService.Meta":                   "",
			"Connect.SidecarService.Proxy.Config":           "",
			"Connect.SidecarService.Proxy.Upstreams.config": "",
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
	if ns.Weights != nil {
		if err := structs.ValidateWeights(ns.Weights); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, fmt.Errorf("Invalid Weights: %v", err))
			return nil, nil
		}
	}
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
	var token string
	s.parseToken(req, &token)
	if err := s.agent.vetServiceRegister(token, ns); err != nil {
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
	if err := s.agent.AddService(ns, chkTypes, true, token, ConfigSourceRemote); err != nil {
		return nil, err
	}
	// Add proxy (which will add proxy service so do it before we trigger sync)
	if proxy != nil {
		if err := s.agent.AddProxy(proxy, true, false, "", ConfigSourceRemote); err != nil {
			return nil, err
		}
	}
	// Add sidecar.
	if sidecar != nil {
		if err := s.agent.AddService(sidecar, sidecarChecks, true, sidecarToken, ConfigSourceRemote); err != nil {
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

	case "connect_replication_token":
		s.agent.tokens.UpdateConnectReplicationToken(args.Token)

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
	// Don't resolve a proxy token to a real token that will be
	// done with a call to verifyProxyToken later along with
	// other security relevant checks.
	if done := s.parseWithoutResolvingProxyToken(resp, req, &args.Datacenter, &qOpts); done {
		return nil, nil
	}
	args.MinQueryIndex = qOpts.MinQueryIndex

	// Verify the proxy token. This will check both the local proxy token
	// as well as the ACL if the token isn't local. The checks done in
	// verifyProxyToken are still relevant because a leaf cert can be cached
	// verifying the proxy token matches the service id or that a real
	// acl token still is valid and has ServiceWrite is necessary or
	// that cached cert is potentially unprotected.
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

	// Parse the token - don't resolve a proxy token to a real token
	// that will be done with a call to verifyProxyToken later along with
	// other security relevant checks.
	var token string
	s.parseTokenWithoutResolvingProxyToken(req, &token)

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

			// Validate the ACL token - because this endpoint uses data local to a single
			// agent, this function is responsible for all enforcement regarding
			// protection of the configuration. verifyProxyToken will match the proxies
			// token to the correct service or in the case of being provide a real ACL
			// token it will ensure that the requester has ServiceWrite privileges
			// for this service.
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
				Upstreams:         proxy.Proxy.Upstreams.ToAPI(),
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
		// the callback to load the new value. If agent sync is paused it means
		// local state is currently being bulk-edited e.g. config reload. In this
		// case it's likely that local state just got unloaded and may or may not be
		// reloaded yet. Wait a short amount of time for Sync to resume to ride out
		// typical config reloads.
		if syncPauseCh := s.agent.syncPausedCh(); syncPauseCh != nil {
			select {
			case <-syncPauseCh:
			case <-timeout.C:
			}
		}
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

	if rule != nil && !rule.OperatorRead() {
		return nil, acl.ErrPermissionDenied
	}

	return debug.CollectHostInfo(), nil
}
