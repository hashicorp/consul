package agent

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logging"
)

// ServiceSummary is used to summarize a service
type ServiceSummary struct {
	Kind                structs.ServiceKind `json:",omitempty"`
	Name                string
	Datacenter          string
	Tags                []string
	Nodes               []string
	ExternalSources     []string
	externalSourceSet   map[string]struct{} // internal to track uniqueness
	checks              map[string]*structs.HealthCheck
	InstanceCount       int
	ChecksPassing       int
	ChecksWarning       int
	ChecksCritical      int
	GatewayConfig       GatewayConfig
	TransparentProxy    bool
	transparentProxySet bool
	ConnectNative       bool

	PeerName string `json:",omitempty"`

	acl.EnterpriseMeta
}

func (s *ServiceSummary) LessThan(other *ServiceSummary) bool {
	if s.EnterpriseMeta.LessThan(&other.EnterpriseMeta) {
		return true
	}
	return s.Name < other.Name
}

type GatewayConfig struct {
	AssociatedServiceCount int      `json:",omitempty"`
	Addresses              []string `json:",omitempty"`

	// internal to track uniqueness
	addressesSet map[string]struct{}
}

type ServiceListingSummary struct {
	ServiceSummary

	ConnectedWithProxy   bool
	ConnectedWithGateway bool
}

type ServiceTopologySummary struct {
	ServiceSummary

	Source    string
	Intention structs.IntentionDecisionSummary
}

type ServiceTopology struct {
	Protocol         string
	TransparentProxy bool
	Upstreams        []*ServiceTopologySummary
	Downstreams      []*ServiceTopologySummary
	FilteredByACLs   bool
}

// UINodes is used to list the nodes in a given datacenter. We return a
// NodeDump which provides overview information for all the nodes
func (s *HTTPHandlers) UINodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Parse arguments
	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	s.parseFilter(req, &args.Filter)

	// Make the RPC request
	var out structs.IndexedNodeDump
	defer setMeta(resp, &out.QueryMeta)
RPC:
	if err := s.agent.RPC(req.Context(), "Internal.NodeDump", &args, &out); err != nil {
		// Retry the request allowing stale data if no leader
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !args.AllowStale {
			args.AllowStale = true
			goto RPC
		}
		return nil, err
	}

	// Use empty list instead of nil
	for _, info := range out.Dump {
		if info.Services == nil {
			info.Services = make([]*structs.NodeService, 0)
		}
		if info.Checks == nil {
			info.Checks = make([]*structs.HealthCheck, 0)
		}
	}
	if out.Dump == nil {
		out.Dump = make(structs.NodeDump, 0)
	}

	// Use empty list instead of nil
	for _, info := range out.ImportedDump {
		if info.Services == nil {
			info.Services = make([]*structs.NodeService, 0)
		}
		if info.Checks == nil {
			info.Checks = make([]*structs.HealthCheck, 0)
		}
	}

	return append(out.Dump, out.ImportedDump...), nil
}

// UINodeInfo is used to get info on a single node in a given datacenter. We return a
// NodeInfo which provides overview information for the node
func (s *HTTPHandlers) UINodeInfo(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Parse arguments
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Verify we have some DC, or use the default
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/internal/ui/node/")
	if args.Node == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing node name"}
	}

	if peer := req.URL.Query().Get("peer"); peer != "" {
		args.PeerName = peer
	}

	// Make the RPC request
	var out structs.IndexedNodeDump
	defer setMeta(resp, &out.QueryMeta)
RPC:
	if err := s.agent.RPC(req.Context(), "Internal.NodeInfo", &args, &out); err != nil {
		// Retry the request allowing stale data if no leader
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !args.AllowStale {
			args.AllowStale = true
			goto RPC
		}
		return nil, err
	}

	// Return only the first entry
	if len(out.Dump) > 0 {
		info := out.Dump[0]
		if info.Services == nil {
			info.Services = make([]*structs.NodeService, 0)
		}
		if info.Checks == nil {
			info.Checks = make([]*structs.HealthCheck, 0)
		}
		return info, nil
	}

	resp.WriteHeader(http.StatusNotFound)
	return nil, nil
}

// UICatalogOverview is used to get a high-level overview of the health of nodes, services,
// and checks in the datacenter.
func (s *HTTPHandlers) UICatalogOverview(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Parse arguments
	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Make the RPC request
	var out structs.CatalogSummary
	if err := s.agent.RPC(req.Context(), "Internal.CatalogOverview", &args, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// UIServices is used to list the services in a given datacenter. We return a
// ServiceSummary which provides overview information for the service
func (s *HTTPHandlers) UIServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Parse arguments
	args := structs.ServiceDumpRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if peer := req.URL.Query().Get("peer"); peer != "" {
		args.PeerName = peer
	}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	s.parseFilter(req, &args.Filter)

	// Make the RPC request
	var out structs.IndexedNodesWithGateways
	defer setMeta(resp, &out.QueryMeta)
RPC:
	if err := s.agent.RPC(req.Context(), "Internal.ServiceDump", &args, &out); err != nil {
		// Retry the request allowing stale data if no leader
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !args.AllowStale {
			args.AllowStale = true
			goto RPC
		}
		return nil, err
	}

	// Store the names of the gateways associated with each service
	var (
		serviceGateways   = make(map[structs.PeeredServiceName][]structs.PeeredServiceName)
		numLinkedServices = make(map[structs.PeeredServiceName]int)
	)
	for _, gs := range out.Gateways {
		psn := structs.PeeredServiceName{Peer: structs.DefaultPeerKeyword, ServiceName: gs.Service}
		gpsn := structs.PeeredServiceName{Peer: structs.DefaultPeerKeyword, ServiceName: gs.Gateway}
		serviceGateways[psn] = append(serviceGateways[psn], gpsn)
		numLinkedServices[gpsn] += 1
	}

	summaries, hasProxy := summarizeServices(append(out.Nodes, out.ImportedNodes...).ToServiceDump(), nil, "")
	sorted := prepSummaryOutput(summaries, false)

	// Ensure at least a zero length slice
	result := make([]*ServiceListingSummary, 0)
	for _, svc := range sorted {
		sum := ServiceListingSummary{ServiceSummary: *svc}

		sn := structs.NewServiceName(svc.Name, &svc.EnterpriseMeta)
		psn := structs.PeeredServiceName{Peer: svc.PeerName, ServiceName: sn}
		if hasProxy[psn] {
			sum.ConnectedWithProxy = true
		}

		// Verify that at least one of the gateways linked by config entry has an instance registered in the catalog
		for _, gw := range serviceGateways[psn] {
			if s := summaries[gw]; s != nil && sum.InstanceCount > 0 {
				sum.ConnectedWithGateway = true
			}
		}
		sum.GatewayConfig.AssociatedServiceCount = numLinkedServices[psn]

		result = append(result, &sum)
	}
	return result, nil
}

// UIGatewayServices is used to query all the nodes for services associated with a gateway along with their gateway config
func (s *HTTPHandlers) UIGatewayServicesNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Parse arguments
	args := structs.ServiceSpecificRequest{}
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/internal/ui/gateway-services-nodes/")
	if args.ServiceName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing gateway name"}
	}

	// Make the RPC request
	var out structs.IndexedServiceDump
	defer setMeta(resp, &out.QueryMeta)
RPC:
	if err := s.agent.RPC(req.Context(), "Internal.GatewayServiceDump", &args, &out); err != nil {
		// Retry the request allowing stale data if no leader
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !args.AllowStale {
			args.AllowStale = true
			goto RPC
		}
		return nil, err
	}

	summaries, _ := summarizeServices(out.Dump, s.agent.config, args.Datacenter)

	prepped := prepSummaryOutput(summaries, false)
	if prepped == nil {
		prepped = make([]*ServiceSummary, 0)
	}
	return prepped, nil
}

// UIServiceTopology returns the list of upstreams and downstreams for a Connect enabled service.
//   - Downstreams are services that list the given service as an upstream
//   - Upstreams are the upstreams defined in the given service's proxy registrations
func (s *HTTPHandlers) UIServiceTopology(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Parse arguments
	args := structs.ServiceSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/internal/ui/service-topology/")
	if args.ServiceName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing service name"}
	}

	kind, ok := req.URL.Query()["kind"]
	if !ok {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing service kind"}
	}
	args.ServiceKind = structs.ServiceKind(kind[0])

	switch args.ServiceKind {
	case structs.ServiceKindTypical, structs.ServiceKindIngressGateway:
		// allowed
	default:
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Unsupported service kind %q", args.ServiceKind)}
	}

	// Make the RPC request
	var out structs.IndexedServiceTopology
	defer setMeta(resp, &out.QueryMeta)
RPC:
	if err := s.agent.RPC(req.Context(), "Internal.ServiceTopology", &args, &out); err != nil {
		// Retry the request allowing stale data if no leader
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !args.AllowStale {
			args.AllowStale = true
			goto RPC
		}
		return nil, err
	}

	upstreams, _ := summarizeServices(out.ServiceTopology.Upstreams.ToServiceDump(), nil, "")
	downstreams, _ := summarizeServices(out.ServiceTopology.Downstreams.ToServiceDump(), nil, "")

	var (
		upstreamResp   = make([]*ServiceTopologySummary, 0)
		downstreamResp = make([]*ServiceTopologySummary, 0)
	)

	// Sort and attach intention data for upstreams and downstreams
	sortedUpstreams := prepSummaryOutput(upstreams, true)
	for _, svc := range sortedUpstreams {
		sn := structs.NewServiceName(svc.Name, &svc.EnterpriseMeta)
		sum := ServiceTopologySummary{
			ServiceSummary: *svc,
			Intention:      out.ServiceTopology.UpstreamDecisions[sn.String()],
			Source:         out.ServiceTopology.UpstreamSources[sn.String()],
		}
		upstreamResp = append(upstreamResp, &sum)
	}
	for k, v := range out.ServiceTopology.UpstreamSources {
		if v == structs.TopologySourceRoutingConfig {
			sn := structs.ServiceNameFromString(k)
			sum := ServiceTopologySummary{
				ServiceSummary: ServiceSummary{
					Datacenter:     args.Datacenter,
					Name:           sn.Name,
					EnterpriseMeta: sn.EnterpriseMeta,
				},
				Intention: out.ServiceTopology.UpstreamDecisions[sn.String()],
				Source:    out.ServiceTopology.UpstreamSources[sn.String()],
			}
			upstreamResp = append(upstreamResp, &sum)
		}
	}

	sortedDownstreams := prepSummaryOutput(downstreams, true)
	for _, svc := range sortedDownstreams {
		sn := structs.NewServiceName(svc.Name, &svc.EnterpriseMeta)
		sum := ServiceTopologySummary{
			ServiceSummary: *svc,
			Intention:      out.ServiceTopology.DownstreamDecisions[sn.String()],
			Source:         out.ServiceTopology.DownstreamSources[sn.String()],
		}
		downstreamResp = append(downstreamResp, &sum)
	}

	topo := ServiceTopology{
		TransparentProxy: out.ServiceTopology.TransparentProxy,
		Protocol:         out.ServiceTopology.MetricsProtocol,
		Upstreams:        upstreamResp,
		Downstreams:      downstreamResp,
		FilteredByACLs:   out.FilteredByACLs,
	}
	return topo, nil
}

func summarizeServices(dump structs.ServiceDump, cfg *config.RuntimeConfig, dc string) (map[structs.PeeredServiceName]*ServiceSummary, map[structs.PeeredServiceName]bool) {
	var (
		summary  = make(map[structs.PeeredServiceName]*ServiceSummary)
		hasProxy = make(map[structs.PeeredServiceName]bool)
	)

	getService := func(psn structs.PeeredServiceName) *ServiceSummary {
		serv, ok := summary[psn]
		if !ok {
			serv = &ServiceSummary{
				Name:           psn.ServiceName.Name,
				EnterpriseMeta: psn.ServiceName.EnterpriseMeta,
				// the other code will increment this unconditionally so we
				// shouldn't initialize it to 1
				InstanceCount: 0,
				PeerName:      psn.Peer,
			}
			summary[psn] = serv
		}
		return serv
	}

	for _, csn := range dump {
		var peerName string
		// all entities will have the same peer name so it is safe to use the node's peer name
		if csn.Node == nil {
			// this can happen for gateway dumps that call this summarize func
			peerName = structs.DefaultPeerKeyword
		} else {
			peerName = csn.Node.PeerName
		}

		if cfg != nil && csn.GatewayService != nil {
			gwsvc := csn.GatewayService

			psn := structs.PeeredServiceName{Peer: peerName, ServiceName: gwsvc.Service}
			sum := getService(psn)
			modifySummaryForGatewayService(cfg, dc, sum, gwsvc)
		}

		// Will happen in cases where we only have the GatewayServices mapping
		if csn.Service == nil {
			continue
		}

		sn := structs.NewServiceName(csn.Service.Service, &csn.Service.EnterpriseMeta)
		psn := structs.PeeredServiceName{Peer: peerName, ServiceName: sn}
		sum := getService(psn)

		svc := csn.Service
		sum.Nodes = append(sum.Nodes, csn.Node.Node)
		sum.Kind = svc.Kind
		sum.Datacenter = csn.Node.Datacenter
		sum.InstanceCount += 1
		sum.ConnectNative = svc.Connect.Native
		if svc.Kind == structs.ServiceKindConnectProxy {
			sn := structs.NewServiceName(svc.Proxy.DestinationServiceName, &svc.EnterpriseMeta)
			psn := structs.PeeredServiceName{Peer: peerName, ServiceName: sn}
			hasProxy[psn] = true

			destination := getService(psn)
			for _, check := range csn.Checks {
				cid := structs.NewCheckID(check.CheckID, &check.EnterpriseMeta)
				uid := structs.UniqueID(csn.Node.Node, cid.String())
				if destination.checks == nil {
					destination.checks = make(map[string]*structs.HealthCheck)
				}
				destination.checks[uid] = check
			}

			// Only consider the target service to be transparent when all its proxy instances are in that mode.
			// This is done because the flag is used to display warnings about proxies needing to enable
			// transparent proxy mode. If ANY instance isn't in the right mode then the warming applies.
			if svc.Proxy.Mode == structs.ProxyModeTransparent && !destination.transparentProxySet {
				destination.TransparentProxy = true
			}
			if svc.Proxy.Mode != structs.ProxyModeTransparent {
				destination.TransparentProxy = false
			}
			destination.transparentProxySet = true
		}
		for _, tag := range svc.Tags {
			found := false
			for _, existing := range sum.Tags {
				if existing == tag {
					found = true
					break
				}
			}
			if !found {
				sum.Tags = append(sum.Tags, tag)
			}
		}

		// If there is an external source, add it to the list of external
		// sources. We only want to add unique sources so there is extra
		// accounting here with an unexported field to maintain the set
		// of sources.
		if len(svc.Meta) > 0 && svc.Meta[structs.MetaExternalSource] != "" {
			source := svc.Meta[structs.MetaExternalSource]
			if sum.externalSourceSet == nil {
				sum.externalSourceSet = make(map[string]struct{})
			}
			if _, ok := sum.externalSourceSet[source]; !ok {
				sum.externalSourceSet[source] = struct{}{}
				sum.ExternalSources = append(sum.ExternalSources, source)
			}
		}

		for _, check := range csn.Checks {
			cid := structs.NewCheckID(check.CheckID, &check.EnterpriseMeta)
			uid := structs.UniqueID(csn.Node.Node, cid.String())
			if sum.checks == nil {
				sum.checks = make(map[string]*structs.HealthCheck)
			}
			sum.checks[uid] = check
		}
	}

	return summary, hasProxy
}

func prepSummaryOutput(summaries map[structs.PeeredServiceName]*ServiceSummary, excludeSidecars bool) []*ServiceSummary {
	var resp []*ServiceSummary
	// Ensure at least a zero length slice
	resp = make([]*ServiceSummary, 0)

	// Collect and sort resp for display
	for _, sum := range summaries {
		sort.Strings(sum.Nodes)
		sort.Strings(sum.Tags)

		for _, chk := range sum.checks {
			switch chk.Status {
			case api.HealthPassing:
				sum.ChecksPassing++
			case api.HealthWarning:
				sum.ChecksWarning++
			case api.HealthCritical:
				sum.ChecksCritical++
			}
		}
		if excludeSidecars && sum.Kind != structs.ServiceKindTypical && sum.Kind != structs.ServiceKindIngressGateway {
			continue
		}
		resp = append(resp, sum)
	}
	sort.Slice(resp, func(i, j int) bool {
		return resp[i].LessThan(resp[j])
	})
	return resp
}

func modifySummaryForGatewayService(
	cfg *config.RuntimeConfig,
	datacenter string,
	sum *ServiceSummary,
	gwsvc *structs.GatewayService,
) {
	var dnsAddresses []string
	for _, domain := range []string{cfg.DNSDomain, cfg.DNSAltDomain} {
		// If the domain is empty, do not use it to construct a valid DNS
		// address
		if domain == "" {
			continue
		}
		dnsAddresses = append(dnsAddresses, serviceIngressDNSName(
			gwsvc.Service.Name,
			datacenter,
			domain,
			&gwsvc.Service.EnterpriseMeta,
		))
	}

	for _, addr := range gwsvc.Addresses(dnsAddresses) {
		// check for duplicates, a service will have a ServiceInfo struct for
		// every instance that is registered.
		if _, ok := sum.GatewayConfig.addressesSet[addr]; !ok {
			if sum.GatewayConfig.addressesSet == nil {
				sum.GatewayConfig.addressesSet = make(map[string]struct{})
			}
			sum.GatewayConfig.addressesSet[addr] = struct{}{}
			sum.GatewayConfig.Addresses = append(
				sum.GatewayConfig.Addresses, addr,
			)
		}
	}
}

// GET /v1/internal/ui/gateway-intentions/:gateway
func (s *HTTPHandlers) UIGatewayIntentions(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.IntentionQueryRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	// Pull out the service name
	name := strings.TrimPrefix(req.URL.Path, "/v1/internal/ui/gateway-intentions/")
	if name == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing gateway name"}
	}
	args.Match = &structs.IntentionQueryMatch{
		Type: structs.IntentionMatchDestination,
		Entries: []structs.IntentionMatchEntry{
			{
				Namespace: entMeta.NamespaceOrEmpty(),
				Partition: entMeta.PartitionOrDefault(),
				Name:      name,
			},
		},
	}

	var reply structs.IndexedIntentions

	defer setMeta(resp, &reply.QueryMeta)
	if err := s.agent.RPC(req.Context(), "Internal.GatewayIntentions", args, &reply); err != nil {
		return nil, err
	}

	return reply.Intentions, nil
}

// UIMetricsProxy handles the /v1/internal/ui/metrics-proxy/ endpoint which, if
// configured, provides a simple read-only HTTP proxy to a single metrics
// backend to expose it to the UI.
func (s *HTTPHandlers) UIMetricsProxy(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Check the UI was enabled at agent startup (note this is not reloadable
	// currently).
	if !s.IsUIEnabled() {
		return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: "UI is not enabled"}
	}

	// Load reloadable proxy config
	cfg, ok := s.metricsProxyCfg.Load().(config.UIMetricsProxy)
	if !ok || cfg.BaseURL == "" {
		// Proxy not configured
		return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: "Metrics proxy is not enabled"}
	}

	// Fetch the ACL token, if provided, but ONLY from headers since other
	// metrics proxies might use a ?token query string parameter for something.
	var token string
	s.parseTokenFromHeaders(req, &token)

	// Clear the token from the headers so we don't end up proxying it.
	s.clearTokenFromHeaders(req)

	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, &entMeta, nil)
	if err != nil {
		return nil, err
	}

	// This endpoint requires wildcard read on all services and all nodes.
	//
	// In enterprise it requires this _in all namespaces_ too.
	//
	// In enterprise it requires this _in all namespaces and partitions_ too.
	var authzContext acl.AuthorizerContext
	wildcardEntMeta := structs.WildcardEnterpriseMetaInPartition(structs.WildcardSpecifier)
	wildcardEntMeta.FillAuthzContext(&authzContext)

	if err := authz.ToAllowAuthorizer().NodeReadAllAllowed(&authzContext); err != nil {
		return nil, err
	}
	if err := authz.ToAllowAuthorizer().ServiceReadAllAllowed(&authzContext); err != nil {
		return nil, err
	}

	log := s.agent.logger.Named(logging.UIMetricsProxy)

	// Construct the new URL from the path and the base path. Note we do this here
	// not in the Director function below because we can handle any errors cleanly
	// here.

	// Replace prefix in the path
	subPath := strings.TrimPrefix(req.URL.Path, "/v1/internal/ui/metrics-proxy")

	// Append that to the BaseURL (which might contain a path prefix component)
	newURL := cfg.BaseURL + subPath

	// Parse it into a new URL
	u, err := url.Parse(newURL)
	if err != nil {
		log.Error("couldn't parse target URL", "base_url", cfg.BaseURL, "path", subPath)
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Invalid path."}
	}

	// Clean the new URL path to prevent path traversal attacks and remove any
	// double slashes etc.
	u.Path = path.Clean(u.Path)

	if len(cfg.PathAllowlist) > 0 {
		// This could be done better with a map, but for the prometheus default
		// integration this list has two items in it, so the straight iteration
		// isn't awful.
		denied := true
		for _, allowedPath := range cfg.PathAllowlist {
			if u.Path == allowedPath {
				denied = false
				break
			}
		}
		if denied {
			log.Error("target URL path is not allowed",
				"base_url", cfg.BaseURL,
				"path", subPath,
				"target_url", u.String(),
				"path_allowlist", cfg.PathAllowlist,
			)
			resp.WriteHeader(http.StatusForbidden)
			return nil, nil
		}
	}

	// Pass through query params
	u.RawQuery = req.URL.RawQuery

	// Validate that the full BaseURL is still a prefix - if there was a path
	// prefix on the BaseURL but an attacker tried to circumvent it with path
	// traversal then the Clean above would have resolve the /../ components back
	// to the actual path which means part of the prefix will now be missing.
	//
	// Note that in practice this is not currently possible since any /../ in the
	// path would have already been resolved by the API server mux and so not even
	// hit this handler. Any /../ that are far enough into the path to hit this
	// handler, can't backtrack far enough to eat into the BaseURL either. But we
	// leave this in anyway in case something changes in the future.
	if !strings.HasPrefix(u.String(), cfg.BaseURL) {
		log.Error("target URL escaped from base path",
			"base_url", cfg.BaseURL,
			"path", subPath,
			"target_url", u.String(),
		)
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Invalid path."}
	}

	// Add any configured headers
	for _, h := range cfg.AddHeaders {
		if strings.ToLower(h.Name) == "host" {
			req.Host = h.Value
		} else {
			req.Header.Set(h.Name, h.Value)
		}
	}

	log.Debug("proxying request", "to", u.String())

	proxy := httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL = u
		},
		Transport: s.proxyTransport,
		ErrorLog: log.StandardLogger(&hclog.StandardLoggerOptions{
			InferLevels: true,
		}),
	}

	proxy.ServeHTTP(resp, req)
	return nil, nil
}

// UIExportedServices is used to list the exported services to a given peer. We return a
// barebones ServiceListingSummary which only contains the name and enterprise meta of a service.
// Currently, the request and response mirror UIServices but the API may change in the future.
func (s *HTTPHandlers) UIExportedServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Parse arguments
	args := structs.ServiceDumpRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if peer := req.URL.Query().Get("peer"); peer != "" {
		args.PeerName = peer
	}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Make the RPC request
	var out structs.IndexedServiceList
	defer setMeta(resp, &out.QueryMeta)
RPC:
	if err := s.agent.RPC(req.Context(), "Internal.ExportedServicesForPeer", &args, &out); err != nil {
		// Retry the request allowing stale data if no leader
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !args.AllowStale {
			args.AllowStale = true
			goto RPC
		}
		return nil, err
	}
	// Ensure at least a zero length slice
	result := make([]*ServiceListingSummary, 0)
	for _, svc := range out.Services {
		// We synthesize a minimal summary for the frontend.
		// The shape of the data may change in the future but
		// currently only the service name is required.
		sum := ServiceListingSummary{
			ServiceSummary: ServiceSummary{
				Name:           svc.Name,
				EnterpriseMeta: svc.EnterpriseMeta,
				Datacenter:     args.Datacenter,
			},
		}
		result = append(result, &sum)
	}
	return result, nil
}
