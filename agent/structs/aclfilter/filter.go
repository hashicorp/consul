// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package aclfilter

import (
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// RedactedToken is shown in structures with embedded tokens when they
	// are not allowed to be displayed.
	RedactedToken = "<hidden>"
)

// Filter is used to filter results based on ACL rules.
type Filter struct {
	authorizer acl.Authorizer
	logger     hclog.Logger
}

// New constructs a Filter with the given authorizer.
func New(authorizer acl.Authorizer, logger hclog.Logger) *Filter {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}
	return &Filter{authorizer, logger}
}

// Filter the given subject in-place.
func (f *Filter) Filter(subject any) {
	switch v := subject.(type) {
	case *structs.CheckServiceNodes:
		f.filterCheckServiceNodes(v)

	case *structs.IndexedCheckServiceNodes:
		v.QueryMeta.ResultsFilteredByACLs = f.filterCheckServiceNodes(&v.Nodes)

	case *structs.PreparedQueryExecuteResponse:
		v.QueryMeta.ResultsFilteredByACLs = f.filterCheckServiceNodes(&v.Nodes)

	case *structs.IndexedServiceTopology:
		filtered := f.filterServiceTopology(v.ServiceTopology)
		if filtered {
			v.FilteredByACLs = true
			v.QueryMeta.ResultsFilteredByACLs = true
		}

	case *structs.DatacenterIndexedCheckServiceNodes:
		v.QueryMeta.ResultsFilteredByACLs = f.filterDatacenterCheckServiceNodes(&v.DatacenterNodes)

	case *structs.IndexedCoordinates:
		v.QueryMeta.ResultsFilteredByACLs = f.filterCoordinates(&v.Coordinates)

	case *structs.IndexedHealthChecks:
		v.QueryMeta.ResultsFilteredByACLs = f.filterHealthChecks(&v.HealthChecks)

	case *structs.IndexedIntentions:
		v.QueryMeta.ResultsFilteredByACLs = f.filterIntentions(&v.Intentions)

	case *structs.IntentionQueryMatch:
		f.filterIntentionMatch(v)

	case *structs.IndexedNodeDump:
		if f.filterNodeDump(&v.Dump) {
			v.QueryMeta.ResultsFilteredByACLs = true
		}
		if f.filterNodeDump(&v.ImportedDump) {
			v.QueryMeta.ResultsFilteredByACLs = true
		}

	case *structs.IndexedServiceDump:
		v.QueryMeta.ResultsFilteredByACLs = f.filterServiceDump(&v.Dump)

	case *structs.IndexedNodes:
		v.QueryMeta.ResultsFilteredByACLs = f.filterNodes(&v.Nodes)

	case *structs.IndexedNodeServices:
		v.QueryMeta.ResultsFilteredByACLs = f.filterNodeServices(&v.NodeServices)

	case *structs.IndexedNodeServiceList:
		v.QueryMeta.ResultsFilteredByACLs = f.filterNodeServiceList(&v.NodeServices)

	case *structs.IndexedServiceNodes:
		v.QueryMeta.ResultsFilteredByACLs = f.filterServiceNodes(&v.ServiceNodes)

	case *structs.IndexedServices:
		v.QueryMeta.ResultsFilteredByACLs = f.filterServices(v.Services, &v.EnterpriseMeta)

	case *structs.IndexedSessions:
		v.QueryMeta.ResultsFilteredByACLs = f.filterSessions(&v.Sessions)

	case *structs.IndexedPreparedQueries:
		v.QueryMeta.ResultsFilteredByACLs = f.filterPreparedQueries(&v.Queries)

	case **structs.PreparedQuery:
		f.redactPreparedQueryTokens(v)

	case *structs.ACLTokens:
		f.filterTokens(v)
	case **structs.ACLToken:
		f.filterToken(v)
	case *[]*structs.ACLTokenListStub:
		f.filterTokenStubs(v)
	case **structs.ACLTokenListStub:
		f.filterTokenStub(v)

	case *structs.ACLPolicies:
		f.filterPolicies(v)
	case **structs.ACLPolicy:
		f.filterPolicy(v)

	case *structs.ACLRoles:
		f.filterRoles(v)
	case **structs.ACLRole:
		f.filterRole(v)

	case *structs.ACLBindingRules:
		f.filterBindingRules(v)
	case **structs.ACLBindingRule:
		f.filterBindingRule(v)

	case *structs.ACLAuthMethods:
		f.filterAuthMethods(v)
	case **structs.ACLAuthMethod:
		f.filterAuthMethod(v)

	case *structs.IndexedServiceList:
		v.QueryMeta.ResultsFilteredByACLs = f.filterServiceList(&v.Services)

	case *structs.IndexedExportedServiceList:
		for peer, peerServices := range v.Services {
			v.QueryMeta.ResultsFilteredByACLs = f.filterServiceList(&peerServices)
			if len(peerServices) == 0 {
				delete(v.Services, peer)
			} else {
				v.Services[peer] = peerServices
			}
		}

	case *structs.IndexedGatewayServices:
		v.QueryMeta.ResultsFilteredByACLs = f.filterGatewayServices(&v.Services)

	case *structs.IndexedNodesWithGateways:
		if f.filterCheckServiceNodes(&v.Nodes) {
			v.QueryMeta.ResultsFilteredByACLs = true
		}
		if f.filterGatewayServices(&v.Gateways) {
			v.QueryMeta.ResultsFilteredByACLs = true
		}
		if f.filterCheckServiceNodes(&v.ImportedNodes) {
			v.QueryMeta.ResultsFilteredByACLs = true
		}

	default:
		panic(fmt.Errorf("Unhandled type passed to ACL filter: %T %#v", subject, subject))
	}
}

// allowNode is used to determine if a node is accessible for an ACL.
func (f *Filter) allowNode(node string, ent *acl.AuthorizerContext) bool {
	return f.authorizer.NodeRead(node, ent) == acl.Allow
}

// allowNode is used to determine if the gateway and service are accessible for an ACL
func (f *Filter) allowGateway(gs *structs.GatewayService) bool {
	var authzContext acl.AuthorizerContext

	// Need read on service and gateway. Gateway may have different EnterpriseMeta so we fill authzContext twice
	gs.Gateway.FillAuthzContext(&authzContext)
	if !f.allowService(gs.Gateway.Name, &authzContext) {
		return false
	}

	gs.Service.FillAuthzContext(&authzContext)
	if !f.allowService(gs.Service.Name, &authzContext) {
		return false
	}
	return true
}

// allowService is used to determine if a service is accessible for an ACL.
func (f *Filter) allowService(service string, ent *acl.AuthorizerContext) bool {
	if service == "" {
		return true
	}

	return f.authorizer.ServiceRead(service, ent) == acl.Allow
}

// allowSession is used to determine if a session for a node is accessible for
// an ACL.
func (f *Filter) allowSession(node string, ent *acl.AuthorizerContext) bool {
	return f.authorizer.SessionRead(node, ent) == acl.Allow
}

// filterHealthChecks is used to filter a set of health checks down based on
// the configured ACL rules for a token. Returns true if any elements were
// removed.
func (f *Filter) filterHealthChecks(checks *structs.HealthChecks) bool {
	hc := *checks
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(hc); i++ {
		check := hc[i]
		check.FillAuthzContext(&authzContext)
		if f.allowNode(check.Node, &authzContext) && f.allowService(check.ServiceName, &authzContext) {
			continue
		}

		f.logger.Debug("dropping check from result due to ACLs", "check", check.CheckID)
		removed = true
		hc = append(hc[:i], hc[i+1:]...)
		i--
	}
	*checks = hc
	return removed
}

// filterServices is used to filter a set of services based on ACLs. Returns
// true if any elements were removed.
func (f *Filter) filterServices(services structs.Services, entMeta *acl.EnterpriseMeta) bool {
	var authzContext acl.AuthorizerContext
	entMeta.FillAuthzContext(&authzContext)

	var removed bool

	for svc := range services {
		if f.allowService(svc, &authzContext) {
			continue
		}
		f.logger.Debug("dropping service from result due to ACLs", "service", svc)
		removed = true
		delete(services, svc)
	}

	return removed
}

// filterServiceNodes is used to filter a set of nodes for a given service
// based on the configured ACL rules. Returns true if any elements were removed.
func (f *Filter) filterServiceNodes(nodes *structs.ServiceNodes) bool {
	sn := *nodes
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(sn); i++ {
		node := sn[i]

		node.FillAuthzContext(&authzContext)
		if f.allowNode(node.Node, &authzContext) && f.allowService(node.ServiceName, &authzContext) {
			continue
		}
		removed = true
		node.CompoundServiceID()
		f.logger.Debug("dropping service node from result due to ACLs",
			"node", structs.NodeNameString(node.Node, &node.EnterpriseMeta),
			"service", node.CompoundServiceID())
		sn = append(sn[:i], sn[i+1:]...)
		i--
	}
	*nodes = sn
	return removed
}

// filterNodeServices is used to filter services on a given node base on ACLs.
// Returns true if any elements were removed
func (f *Filter) filterNodeServices(services **structs.NodeServices) bool {
	if *services == nil {
		return false
	}

	var authzContext acl.AuthorizerContext
	(*services).Node.FillAuthzContext(&authzContext)
	if !f.allowNode((*services).Node.Node, &authzContext) {
		*services = nil
		return true
	}

	var removed bool
	for svcName, svc := range (*services).Services {
		svc.FillAuthzContext(&authzContext)

		if f.allowNode((*services).Node.Node, &authzContext) && f.allowService(svcName, &authzContext) {
			continue
		}
		f.logger.Debug("dropping service from result due to ACLs", "service", svc.CompoundServiceID())
		removed = true
		delete((*services).Services, svcName)
	}

	return removed
}

// filterNodeServices is used to filter services on a given node base on ACLs.
// Returns true if any elements were removed.
func (f *Filter) filterNodeServiceList(services *structs.NodeServiceList) bool {
	if services.Node == nil {
		return false
	}

	var authzContext acl.AuthorizerContext
	services.Node.FillAuthzContext(&authzContext)
	if !f.allowNode(services.Node.Node, &authzContext) {
		*services = structs.NodeServiceList{}
		return true
	}

	var removed bool
	svcs := services.Services
	for i := 0; i < len(svcs); i++ {
		svc := svcs[i]
		svc.FillAuthzContext(&authzContext)

		if f.allowService(svc.Service, &authzContext) {
			continue
		}

		f.logger.Debug("dropping service from result due to ACLs", "service", svc.CompoundServiceID())
		svcs = append(svcs[:i], svcs[i+1:]...)
		i--
		removed = true
	}
	services.Services = svcs

	return removed
}

// filterCheckServiceNodes is used to filter nodes based on ACL rules. Returns
// true if any elements were removed.
func (f *Filter) filterCheckServiceNodes(nodes *structs.CheckServiceNodes) bool {
	csn := *nodes
	var removed bool

	for i := 0; i < len(csn); i++ {
		node := csn[i]
		if node.CanRead(f.authorizer) == acl.Allow {
			continue
		}
		f.logger.Debug("dropping check service node from result due to ACLs",
			"node", structs.NodeNameString(node.Node.Node, node.Node.GetEnterpriseMeta()),
			"service", node.Service.CompoundServiceID())
		removed = true
		csn = append(csn[:i], csn[i+1:]...)
		i--
	}
	*nodes = csn
	return removed
}

// filterServiceTopology is used to filter upstreams/downstreams based on ACL rules.
// this filter is unlike others in that it also returns whether the result was filtered by ACLs
func (f *Filter) filterServiceTopology(topology *structs.ServiceTopology) bool {
	filteredUpstreams := f.filterCheckServiceNodes(&topology.Upstreams)
	filteredDownstreams := f.filterCheckServiceNodes(&topology.Downstreams)
	return filteredUpstreams || filteredDownstreams
}

// filterDatacenterCheckServiceNodes is used to filter nodes based on ACL rules.
// Returns true if any elements are removed.
func (f *Filter) filterDatacenterCheckServiceNodes(datacenterNodes *map[string]structs.CheckServiceNodes) bool {
	dn := *datacenterNodes
	out := make(map[string]structs.CheckServiceNodes)
	var removed bool
	for dc := range dn {
		nodes := dn[dc]
		if f.filterCheckServiceNodes(&nodes) {
			removed = true
		}
		if len(nodes) > 0 {
			out[dc] = nodes
		}
	}
	*datacenterNodes = out
	return removed
}

// filterSessions is used to filter a set of sessions based on ACLs. Returns
// true if any elements were removed.
func (f *Filter) filterSessions(sessions *structs.Sessions) bool {
	s := *sessions

	var removed bool
	for i := 0; i < len(s); i++ {
		session := s[i]

		var entCtx acl.AuthorizerContext
		session.FillAuthzContext(&entCtx)

		if f.allowSession(session.Node, &entCtx) {
			continue
		}
		removed = true
		f.logger.Debug("dropping session from result due to ACLs", "session", session.ID)
		s = append(s[:i], s[i+1:]...)
		i--
	}
	*sessions = s
	return removed
}

// filterCoordinates is used to filter nodes in a coordinate dump based on ACL
// rules. Returns true if any elements were removed.
func (f *Filter) filterCoordinates(coords *structs.Coordinates) bool {
	c := *coords
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(c); i++ {
		c[i].FillAuthzContext(&authzContext)
		node := c[i].Node
		if f.allowNode(node, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node, c[i].GetEnterpriseMeta()))
		removed = true
		c = append(c[:i], c[i+1:]...)
		i--
	}
	*coords = c
	return removed
}

// filterIntentions is used to filter intentions based on ACL rules.
// We prune entries the user doesn't have access to, and we redact any tokens
// if the user doesn't have a management token. Returns true if any elements
// were removed.
func (f *Filter) filterIntentions(ixns *structs.Intentions) bool {
	ret := make(structs.Intentions, 0, len(*ixns))
	var removed bool
	for _, ixn := range *ixns {
		if !ixn.CanRead(f.authorizer) {
			removed = true
			f.logger.Debug("dropping intention from result due to ACLs", "intention", ixn.ID)
			continue
		}

		ret = append(ret, ixn)
	}

	*ixns = ret
	return removed
}

// filterIntentionMatch filters IntentionQueryMatch to only exclude all
// matches when the user doesn't have access to any match.
func (f *Filter) filterIntentionMatch(args *structs.IntentionQueryMatch) {
	var authzContext acl.AuthorizerContext
	authz := f.authorizer.ToAllowAuthorizer()
	for _, entry := range args.Entries {
		entry.FillAuthzContext(&authzContext)
		if prefix := entry.Name; prefix != "" {
			if err := authz.IntentionReadAllowed(prefix, &authzContext); err != nil {
				accessorID := authz.AccessorID
				f.logger.Warn("Operation on intention prefix denied due to ACLs",
					"prefix", prefix,
					"accessorID", acl.AliasIfAnonymousToken(accessorID))
				args.Entries = nil
				return
			}
		}
	}
}

// filterNodeDump is used to filter through all parts of a node dump and
// remove elements the provided ACL token cannot access. Returns true if
// any elements were removed.
func (f *Filter) filterNodeDump(dump *structs.NodeDump) bool {
	nd := *dump

	var authzContext acl.AuthorizerContext
	var removed bool
	for i := 0; i < len(nd); i++ {
		info := nd[i]

		// Filter nodes
		info.FillAuthzContext(&authzContext)
		if node := info.Node; !f.allowNode(node, &authzContext) {
			f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node, info.GetEnterpriseMeta()))
			removed = true
			nd = append(nd[:i], nd[i+1:]...)
			i--
			continue
		}

		// Filter services
		for j := 0; j < len(info.Services); j++ {
			svc := info.Services[j].Service
			info.Services[j].FillAuthzContext(&authzContext)
			if f.allowNode(info.Node, &authzContext) && f.allowService(svc, &authzContext) {
				continue
			}
			f.logger.Debug("dropping service from result due to ACLs", "service", svc)
			removed = true
			info.Services = append(info.Services[:j], info.Services[j+1:]...)
			j--
		}

		// Filter checks
		for j := 0; j < len(info.Checks); j++ {
			chk := info.Checks[j]
			chk.FillAuthzContext(&authzContext)
			if f.allowNode(info.Node, &authzContext) && f.allowService(chk.ServiceName, &authzContext) {
				continue
			}
			f.logger.Debug("dropping check from result due to ACLs", "check", chk.CheckID)
			removed = true
			info.Checks = append(info.Checks[:j], info.Checks[j+1:]...)
			j--
		}
	}
	*dump = nd
	return removed
}

// filterServiceDump is used to filter nodes based on ACL rules. Returns true
// if any elements were removed.
func (f *Filter) filterServiceDump(services *structs.ServiceDump) bool {
	svcs := *services
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(svcs); i++ {
		service := svcs[i]

		if f.allowGateway(service.GatewayService) {
			// ServiceDump might only have gateway config and no node information
			if service.Node == nil {
				continue
			}

			service.Service.FillAuthzContext(&authzContext)
			if f.allowNode(service.Node.Node, &authzContext) {
				continue
			}
		}

		f.logger.Debug("dropping service from result due to ACLs", "service", service.GatewayService.Service)
		removed = true
		svcs = append(svcs[:i], svcs[i+1:]...)
		i--
	}
	*services = svcs
	return removed
}

// filterNodes is used to filter through all parts of a node list and remove
// elements the provided ACL token cannot access. Returns true if any elements
// were removed.
func (f *Filter) filterNodes(nodes *structs.Nodes) bool {
	n := *nodes

	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(n); i++ {
		n[i].FillAuthzContext(&authzContext)
		node := n[i].Node
		if f.allowNode(node, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node, n[i].GetEnterpriseMeta()))
		removed = true
		n = append(n[:i], n[i+1:]...)
		i--
	}
	*nodes = n
	return removed
}

// redactPreparedQueryTokens will redact any tokens unless the client has a
// management token. This eases the transition to delegated authority over
// prepared queries, since it was easy to capture management tokens in Consul
// 0.6.3 and earlier, and we don't want to willy-nilly show those. This does
// have the limitation of preventing delegated non-management users from seeing
// captured tokens, but they can at least see whether or not a token is set.
func (f *Filter) redactPreparedQueryTokens(query **structs.PreparedQuery) {
	// Management tokens can see everything with no filtering.
	var authzContext acl.AuthorizerContext
	structs.DefaultEnterpriseMetaInDefaultPartition().FillAuthzContext(&authzContext)
	if f.authorizer.ACLWrite(&authzContext) == acl.Allow {
		return
	}

	// Let the user see if there's a blank token, otherwise we need
	// to redact it, since we know they don't have a management
	// token.
	if (*query).Token != "" {
		// Redact the token, using a copy of the query structure
		// since we could be pointed at a live instance from the
		// state store so it's not safe to modify it. Note that
		// this clone will still point to things like underlying
		// arrays in the original, but for modifying just the
		// token it will be safe to use.
		clone := *(*query)
		clone.Token = RedactedToken
		*query = &clone
	}
}

// filterPreparedQueries is used to filter prepared queries based on ACL rules.
// We prune entries the user doesn't have access to, and we redact any tokens
// if the user doesn't have a management token. Returns true if any (named)
// queries were removed - un-named queries are meant to be ephemeral and can
// only be enumerated by a management token
func (f *Filter) filterPreparedQueries(queries *structs.PreparedQueries) bool {
	var authzContext acl.AuthorizerContext
	structs.DefaultEnterpriseMetaInDefaultPartition().FillAuthzContext(&authzContext)
	// Management tokens can see everything with no filtering.
	// TODO  is this check even necessary - this looks like a search replace from
	// the 1.4 ACL rewrite. The global-management token will provide unrestricted query privileges
	// so asking for ACLWrite should be unnecessary.
	if f.authorizer.ACLWrite(&authzContext) == acl.Allow {
		return false
	}

	// Otherwise, we need to see what the token has access to.
	var namedQueriesRemoved bool
	ret := make(structs.PreparedQueries, 0, len(*queries))
	for _, query := range *queries {
		// If no prefix ACL applies to this query then filter it, since
		// we know at this point the user doesn't have a management
		// token, otherwise see what the policy says.
		prefix, hasName := query.GetACLPrefix()
		switch {
		case hasName && f.authorizer.PreparedQueryRead(prefix, &authzContext) != acl.Allow:
			namedQueriesRemoved = true
			fallthrough
		case !hasName:
			f.logger.Debug("dropping prepared query from result due to ACLs", "query", query.ID)
			continue
		}

		// Redact any tokens if necessary. We make a copy of just the
		// pointer so we don't mess with the caller's slice.
		final := query
		f.redactPreparedQueryTokens(&final)
		ret = append(ret, final)
	}
	*queries = ret
	return namedQueriesRemoved
}

func (f *Filter) filterToken(token **structs.ACLToken) {
	var entCtx acl.AuthorizerContext
	if token == nil || *token == nil || f == nil {
		return
	}

	(*token).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*token = nil
	} else if f.authorizer.ACLWrite(&entCtx) != acl.Allow {
		// no write permissions - redact secret
		clone := *(*token)
		clone.SecretID = RedactedToken
		*token = &clone
	}
}

func (f *Filter) filterTokens(tokens *structs.ACLTokens) {
	ret := make(structs.ACLTokens, 0, len(*tokens))
	for _, token := range *tokens {
		final := token
		f.filterToken(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*tokens = ret
}

func (f *Filter) filterTokenStub(token **structs.ACLTokenListStub) {
	var entCtx acl.AuthorizerContext
	if token == nil || *token == nil || f == nil {
		return
	}

	(*token).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		*token = nil
	} else if f.authorizer.ACLWrite(&entCtx) != acl.Allow {
		// no write permissions - redact secret
		clone := *(*token)
		clone.SecretID = RedactedToken
		*token = &clone
	}
}

func (f *Filter) filterTokenStubs(tokens *[]*structs.ACLTokenListStub) {
	ret := make(structs.ACLTokenListStubs, 0, len(*tokens))
	for _, token := range *tokens {
		final := token
		f.filterTokenStub(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*tokens = ret
}

func (f *Filter) filterPolicy(policy **structs.ACLPolicy) {
	var entCtx acl.AuthorizerContext
	if policy == nil || *policy == nil || f == nil {
		return
	}

	(*policy).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*policy = nil
	}
}

func (f *Filter) filterPolicies(policies *structs.ACLPolicies) {
	ret := make(structs.ACLPolicies, 0, len(*policies))
	for _, policy := range *policies {
		final := policy
		f.filterPolicy(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*policies = ret
}

func (f *Filter) filterRole(role **structs.ACLRole) {
	var entCtx acl.AuthorizerContext
	if role == nil || *role == nil || f == nil {
		return
	}

	(*role).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*role = nil
	}
}

func (f *Filter) filterRoles(roles *structs.ACLRoles) {
	ret := make(structs.ACLRoles, 0, len(*roles))
	for _, role := range *roles {
		final := role
		f.filterRole(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*roles = ret
}

func (f *Filter) filterBindingRule(rule **structs.ACLBindingRule) {
	var entCtx acl.AuthorizerContext
	if rule == nil || *rule == nil || f == nil {
		return
	}

	(*rule).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*rule = nil
	}
}

func (f *Filter) filterBindingRules(rules *structs.ACLBindingRules) {
	ret := make(structs.ACLBindingRules, 0, len(*rules))
	for _, rule := range *rules {
		final := rule
		f.filterBindingRule(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*rules = ret
}

func (f *Filter) filterAuthMethod(method **structs.ACLAuthMethod) {
	var entCtx acl.AuthorizerContext
	if method == nil || *method == nil || f == nil {
		return
	}

	(*method).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*method = nil
	}
}

func (f *Filter) filterAuthMethods(methods *structs.ACLAuthMethods) {
	ret := make(structs.ACLAuthMethods, 0, len(*methods))
	for _, method := range *methods {
		final := method
		f.filterAuthMethod(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*methods = ret
}

func (f *Filter) filterServiceList(services *structs.ServiceList) bool {
	ret := make(structs.ServiceList, 0, len(*services))
	var removed bool
	for _, svc := range *services {
		var authzContext acl.AuthorizerContext

		svc.FillAuthzContext(&authzContext)

		if f.authorizer.ServiceRead(svc.Name, &authzContext) != acl.Allow {
			removed = true
			sid := structs.NewServiceID(svc.Name, &svc.EnterpriseMeta)
			f.logger.Debug("dropping service from result due to ACLs", "service", sid.String())
			continue
		}

		ret = append(ret, svc)
	}

	*services = ret
	return removed
}

// filterGatewayServices is used to filter gateway to service mappings based on ACL rules.
// Returns true if any elements were removed.
func (f *Filter) filterGatewayServices(mappings *structs.GatewayServices) bool {
	ret := make(structs.GatewayServices, 0, len(*mappings))
	var removed bool
	for _, s := range *mappings {
		// This filter only checks ServiceRead on the linked service.
		// ServiceRead on the gateway is checked in the GatewayServices endpoint before filtering.
		var authzContext acl.AuthorizerContext
		s.Service.FillAuthzContext(&authzContext)

		if f.authorizer.ServiceRead(s.Service.Name, &authzContext) != acl.Allow {
			f.logger.Debug("dropping service from result due to ACLs", "service", s.Service.String())
			removed = true
			continue
		}
		ret = append(ret, s)
	}
	*mappings = ret
	return removed
}
