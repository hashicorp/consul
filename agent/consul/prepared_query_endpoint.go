package consul

import (
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
	"github.com/hashicorp/consul/logging"
)

var PreparedQuerySummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"prepared-query", "apply"},
		Help: "Measures the time it takes to apply a prepared query update.",
	},
	{
		Name: []string{"prepared-query", "explain"},
		Help: "Measures the time it takes to process a prepared query explain request.",
	},
	{
		Name: []string{"prepared-query", "execute"},
		Help: "Measures the time it takes to process a prepared query execute request.",
	},
	{
		Name: []string{"prepared-query", "execute_remote"},
		Help: "Measures the time it takes to process a prepared query execute request that was forwarded to another datacenter.",
	},
}

// PreparedQuery manages the prepared query endpoint.
type PreparedQuery struct {
	srv    *Server
	logger hclog.Logger
}

// Apply is used to apply a modifying request to the data store. This should
// only be used for operations that modify the data. The ID of the session is
// returned in the reply.
func (p *PreparedQuery) Apply(args *structs.PreparedQueryRequest, reply *string) (err error) {
	if done, err := p.srv.ForwardRPC("PreparedQuery.Apply", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"prepared-query", "apply"}, time.Now())

	// Validate the ID. We must create new IDs before applying to the Raft
	// log since it's not deterministic.
	if args.Op == structs.PreparedQueryCreate {
		if args.Query.ID != "" {
			return fmt.Errorf("ID must be empty when creating a new prepared query")
		}

		// We are relying on the fact that UUIDs are random and unlikely
		// to collide since this isn't inside a write transaction.
		state := p.srv.fsm.State()
		for {
			if args.Query.ID, err = uuid.GenerateUUID(); err != nil {
				return fmt.Errorf("UUID generation for prepared query failed: %v", err)
			}
			_, query, err := state.PreparedQueryGet(nil, args.Query.ID)
			if err != nil {
				return fmt.Errorf("Prepared query lookup failed: %v", err)
			}
			if query == nil {
				break
			}
		}
	}
	*reply = args.Query.ID

	// Get the ACL token for the request for the checks below.
	authz, err := p.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	// If prefix ACLs apply to the incoming query, then do an ACL check. We
	// need to make sure they have write access for whatever they are
	// proposing.
	if prefix, ok := args.Query.GetACLPrefix(); ok {
		if err := authz.ToAllowAuthorizer().PreparedQueryWriteAllowed(prefix, nil); err != nil {
			p.logger.Warn("Operation on prepared query denied due to ACLs", "query", args.Query.ID)
			return err
		}
	}

	// This is the second part of the check above. If they are referencing
	// an existing query then make sure it exists and that they have write
	// access to whatever they are changing, if prefix ACLs apply to it.
	if args.Op != structs.PreparedQueryCreate {
		state := p.srv.fsm.State()
		_, query, err := state.PreparedQueryGet(nil, args.Query.ID)
		if err != nil {
			return fmt.Errorf("Prepared Query lookup failed: %v", err)
		}
		if query == nil {
			return fmt.Errorf("Cannot modify non-existent prepared query: '%s'", args.Query.ID)
		}

		if prefix, ok := query.GetACLPrefix(); ok {
			if err := authz.ToAllowAuthorizer().PreparedQueryWriteAllowed(prefix, nil); err != nil {
				p.logger.Warn("Operation on prepared query denied due to ACLs", "query", args.Query.ID)
				return err
			}
		}
	}

	// Parse the query and prep it for the state store.
	switch args.Op {
	case structs.PreparedQueryCreate, structs.PreparedQueryUpdate:
		if err := parseQuery(args.Query); err != nil {
			return fmt.Errorf("Invalid prepared query: %v", err)
		}

	case structs.PreparedQueryDelete:
		// Nothing else to verify here, just do the delete (we only look
		// at the ID field for this op).

	default:
		return fmt.Errorf("Unknown prepared query operation: %s", args.Op)
	}

	// Commit the query to the state store.
	_, err = p.srv.raftApply(structs.PreparedQueryRequestType, args)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}
	return nil
}

// parseQuery makes sure the entries of a query are valid for a create or
// update operation. Some of the fields are not checked or are partially
// checked, as noted in the comments below. This also updates all the parsed
// fields of the query.
func parseQuery(query *structs.PreparedQuery) error {
	// We skip a few fields:
	// - ID is checked outside this fn.
	// - Name is optional with no restrictions, except for uniqueness which
	//   is checked for integrity during the transaction. We also make sure
	//   names do not overlap with IDs, which is also checked during the
	//   transaction. Otherwise, people could "steal" queries that they don't
	//   have proper ACL rights to change.
	// - Template is checked during the transaction since that's where we
	//   compile it.

	// Anonymous queries require a session or need to be part of a template.
	if query.Name == "" && query.Template.Type == "" && query.Session == "" {
		return fmt.Errorf("Must be bound to a session")
	}

	// Token is checked when the query is executed, but we do make sure the
	// user hasn't accidentally pasted-in the special redacted token name,
	// which if we allowed in would be super hard to debug and understand.
	if query.Token == aclfilter.RedactedToken {
		return fmt.Errorf("Bad Token '%s', it looks like a query definition with a redacted token was submitted", query.Token)
	}

	// Parse the service query sub-structure.
	if err := parseService(&query.Service); err != nil {
		return err
	}

	// Parse the DNS options sub-structure.
	if err := parseDNS(&query.DNS); err != nil {
		return err
	}

	return nil
}

// parseService makes sure the entries of a query are valid for a create or
// update operation. Some of the fields are not checked or are partially
// checked, as noted in the comments below. This also updates all the parsed
// fields of the query.
func parseService(svc *structs.ServiceQuery) error {
	// Service is required.
	if svc.Service == "" {
		return fmt.Errorf("Must provide a Service name to query")
	}

	failover := svc.Failover
	// NearestN can be 0 which means "don't fail over by RTT".
	if failover.NearestN < 0 {
		return fmt.Errorf("Bad NearestN '%d', must be >= 0", svc.Failover.NearestN)
	}

	if (failover.NearestN != 0 || len(failover.Datacenters) != 0) && len(failover.Targets) != 0 {
		return fmt.Errorf("Targets cannot be populated with NearestN or Datacenters")
	}

	// Make sure the metadata filters are valid
	if err := structs.ValidateNodeMetadata(svc.NodeMeta, true); err != nil {
		return err
	}

	// We skip a few fields:
	// - There's no validation for Datacenters; we skip any unknown entries
	//   at execution time.
	// - OnlyPassing is just a boolean so doesn't need further validation.
	// - Tags is a free-form list of tags and doesn't need further validation.

	return nil
}

// parseDNS makes sure the entries of a query are valid for a create or
// update operation. This also updates all the parsed fields of the query.
func parseDNS(dns *structs.QueryDNSOptions) error {
	if dns.TTL != "" {
		ttl, err := time.ParseDuration(dns.TTL)
		if err != nil {
			return fmt.Errorf("Bad DNS TTL '%s': %v", dns.TTL, err)
		}

		if ttl < 0 {
			return fmt.Errorf("DNS TTL '%d', must be >=0", ttl)
		}
	}

	return nil
}

// Get returns a single prepared query by ID.
func (p *PreparedQuery) Get(args *structs.PreparedQuerySpecificRequest,
	reply *structs.IndexedPreparedQueries) error {
	if done, err := p.srv.ForwardRPC("PreparedQuery.Get", args, reply); done {
		return err
	}

	return p.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, query, err := state.PreparedQueryGet(ws, args.QueryID)
			if err != nil {
				return err
			}
			if query == nil {
				return structs.ErrQueryNotFound
			}

			// If no prefix ACL applies to this query, then they are
			// always allowed to see it if they have the ID. We still
			// have to filter the remaining object for tokens.
			reply.Index = index
			reply.Queries = structs.PreparedQueries{query}
			if _, ok := query.GetACLPrefix(); !ok {
				return p.srv.filterACL(args.Token, &reply.Queries[0])
			}

			// Otherwise, attempt to filter it the usual way.
			if err := p.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			// Since this is a GET of a specific query, if ACLs have
			// prevented us from returning something that exists,
			// then alert the user with a permission denied error.
			if len(reply.Queries) == 0 {
				p.logger.Warn("Request to get prepared query denied due to ACLs", "query", args.QueryID)
				return acl.ErrPermissionDenied
			}

			return nil
		})
}

// List returns all the prepared queries.
func (p *PreparedQuery) List(args *structs.DCSpecificRequest, reply *structs.IndexedPreparedQueries) error {
	if done, err := p.srv.ForwardRPC("PreparedQuery.List", args, reply); done {
		return err
	}

	return p.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, queries, err := state.PreparedQueryList(ws)
			if err != nil {
				return err
			}

			reply.Index, reply.Queries = index, queries
			return p.srv.filterACL(args.Token, reply)
		})
}

// Explain resolves a prepared query and returns the (possibly rendered template)
// to the caller. This is useful for letting operators figure out which query is
// picking up a given name. We can also add additional info about how the query
// will be executed here.
func (p *PreparedQuery) Explain(args *structs.PreparedQueryExecuteRequest,
	reply *structs.PreparedQueryExplainResponse) error {
	if done, err := p.srv.ForwardRPC("PreparedQuery.Explain", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"prepared-query", "explain"}, time.Now())

	// We have to do this ourselves since we are not doing a blocking RPC.
	p.srv.setQueryMeta(&reply.QueryMeta, args.Token)
	if args.RequireConsistent {
		if err := p.srv.consistentRead(); err != nil {
			return err
		}
	}

	// Try to locate the query.
	state := p.srv.fsm.State()
	_, query, err := state.PreparedQueryResolve(args.QueryIDOrName, args.Agent)
	if err != nil {
		return err
	}
	if query == nil {
		return structs.ErrQueryNotFound
	}

	// Place the query into a list so we can run the standard ACL filter on
	// it.
	queries := &structs.IndexedPreparedQueries{
		Queries: structs.PreparedQueries{query},
	}
	if err := p.srv.filterACL(args.Token, queries); err != nil {
		return err
	}

	// If the query was filtered out, return an error.
	if len(queries.Queries) == 0 {
		p.logger.Warn("Explain on prepared query denied due to ACLs", "query", query.ID)
		return acl.ErrPermissionDenied
	}

	reply.Query = *(queries.Queries[0])
	return nil
}

// Execute runs a prepared query and returns the results. This will perform the
// failover logic if no local results are available. This is typically called as
// part of a DNS lookup, or when executing prepared queries from the HTTP API.
func (p *PreparedQuery) Execute(args *structs.PreparedQueryExecuteRequest,
	reply *structs.PreparedQueryExecuteResponse) error {
	if done, err := p.srv.ForwardRPC("PreparedQuery.Execute", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"prepared-query", "execute"}, time.Now())

	// We have to do this ourselves since we are not doing a blocking RPC.
	if args.RequireConsistent {
		if err := p.srv.consistentRead(); err != nil {
			return err
		}
	}

	// Try to locate the query.
	state := p.srv.fsm.State()
	_, query, err := state.PreparedQueryResolve(args.QueryIDOrName, args.Agent)
	if err != nil {
		return err
	}
	if query == nil {
		return structs.ErrQueryNotFound
	}

	// Execute the query for the local DC.
	if err := p.execute(query, reply, args.Connect); err != nil {
		return err
	}

	// If they supplied a token with the query, use that, otherwise use the
	// token passed in with the request.
	token := args.QueryOptions.Token
	if query.Token != "" {
		token = query.Token
	}
	if err := p.srv.filterACL(token, reply); err != nil {
		return err
	}

	// TODO (slackpad) We could add a special case here that will avoid the
	// fail over if we filtered everything due to ACLs. This seems like it
	// might not be worth the code complexity and behavior differences,
	// though, since this is essentially a misconfiguration.

	// We have to do this ourselves since we are not doing a blocking RPC.
	p.srv.setQueryMeta(&reply.QueryMeta, token)

	// Shuffle the results in case coordinates are not available if they
	// requested an RTT sort.
	reply.Nodes.Shuffle()

	// Build the query source. This can be provided by the client, or by
	// the prepared query. Client-specified takes priority.
	qs := args.Source
	if qs.Datacenter == "" {
		qs.Datacenter = args.Agent.Datacenter
	}
	if query.Service.Near != "" && qs.Node == "" {
		qs.Node = query.Service.Near
	}

	// Respect the magic "_agent" flag.
	if qs.Node == "_agent" {
		qs.Node = args.Agent.Node
	} else if qs.Node == "_ip" {
		if args.Source.Ip != "" {
			_, nodes, err := state.Nodes(nil, structs.NodeEnterpriseMetaInDefaultPartition(), structs.TODOPeerKeyword)
			if err != nil {
				return err
			}

			for _, node := range nodes {
				if args.Source.Ip == node.Address {
					qs.Node = node.Node
					break
				}
			}
		} else {
			p.logger.Warn("Prepared Query using near=_ip requires " +
				"the source IP to be set but none was provided. No distance " +
				"sorting will be done.")

		}

		// Either a source IP was given but we couldnt find the associated node
		// or no source ip was given. In both cases we should wipe the Node value
		if qs.Node == "_ip" {
			qs.Node = ""
		}
	}

	// Perform the distance sort
	err = p.srv.sortNodesByDistanceFrom(qs, reply.Nodes)
	if err != nil {
		return err
	}

	// If we applied a distance sort, make sure that the node queried for is in
	// position 0, provided the results are from the same datacenter.
	if qs.Node != "" && reply.Datacenter == qs.Datacenter {
		for i, node := range reply.Nodes {
			if strings.EqualFold(node.Node.Node, qs.Node) {
				reply.Nodes[0], reply.Nodes[i] = reply.Nodes[i], reply.Nodes[0]
				break
			}

			// Put a cap on the depth of the search. The local agent should
			// never be further in than this if distance sorting was applied.
			if i == 9 {
				break
			}
		}
	}

	// Apply the limit if given.
	if args.Limit > 0 && len(reply.Nodes) > args.Limit {
		reply.Nodes = reply.Nodes[:args.Limit]
	}

	// In the happy path where we found some healthy nodes we go with that
	// and bail out. Otherwise, we fail over and try remote DCs, as allowed
	// by the query setup.
	if len(reply.Nodes) == 0 {
		wrapper := &queryServerWrapper{srv: p.srv, executeRemote: p.ExecuteRemote}
		if err := queryFailover(wrapper, query, args, reply); err != nil {
			return err
		}
	}

	return nil
}

// ExecuteRemote is used when a local node doesn't have any instances of a
// service available and needs to probe remote DCs. This sends the full query
// over since the remote side won't have it in its state store, and this doesn't
// do the failover logic since that's already being run on the originating DC.
// We don't want things to fan out further than one level.
func (p *PreparedQuery) ExecuteRemote(args *structs.PreparedQueryExecuteRemoteRequest,
	reply *structs.PreparedQueryExecuteResponse) error {
	if done, err := p.srv.ForwardRPC("PreparedQuery.ExecuteRemote", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"prepared-query", "execute_remote"}, time.Now())

	// We have to do this ourselves since we are not doing a blocking RPC.
	if args.RequireConsistent {
		if err := p.srv.consistentRead(); err != nil {
			return err
		}
	}

	// Run the query locally to see what we can find.
	if err := p.execute(&args.Query, reply, args.Connect); err != nil {
		return err
	}

	// If they supplied a token with the query, use that, otherwise use the
	// token passed in with the request.
	token := args.QueryOptions.Token
	if args.Query.Token != "" {
		token = args.Query.Token
	}
	if err := p.srv.filterACL(token, reply); err != nil {
		return err
	}

	// We have to do this ourselves since we are not doing a blocking RPC.
	p.srv.setQueryMeta(&reply.QueryMeta, token)

	// We don't bother trying to do an RTT sort here since we are by
	// definition in another DC. We just shuffle to make sure that we
	// balance the load across the results.
	reply.Nodes.Shuffle()

	// Apply the limit if given.
	if args.Limit > 0 && len(reply.Nodes) > args.Limit {
		reply.Nodes = reply.Nodes[:args.Limit]
	}

	return nil
}

// execute runs a prepared query in the local DC without any failover. We don't
// apply any sorting options or ACL checks at this level - it should be done up above.
func (p *PreparedQuery) execute(query *structs.PreparedQuery,
	reply *structs.PreparedQueryExecuteResponse,
	forceConnect bool) error {
	state := p.srv.fsm.State()

	// If we're requesting Connect-capable services, then switch the
	// lookup to be the Connect function.
	f := state.CheckServiceNodes
	if query.Service.Connect || forceConnect {
		f = state.CheckConnectServiceNodes
	}

	_, nodes, err := f(nil, query.Service.Service, &query.Service.EnterpriseMeta, query.Service.PeerName)
	if err != nil {
		return err
	}

	// Filter out any unhealthy nodes.
	nodes = nodes.FilterIgnore(query.Service.OnlyPassing,
		query.Service.IgnoreCheckIDs)

	// Apply the node metadata filters, if any.
	if len(query.Service.NodeMeta) > 0 {
		nodes = nodeMetaFilter(query.Service.NodeMeta, nodes)
	}

	// Apply the service metadata filters, if any.
	if len(query.Service.ServiceMeta) > 0 {
		nodes = serviceMetaFilter(query.Service.ServiceMeta, nodes)
	}

	// Apply the tag filters, if any.
	if len(query.Service.Tags) > 0 {
		nodes = tagFilter(query.Service.Tags, nodes)
	}

	// Capture the nodes and pass the DNS information through to the reply.
	reply.Service = query.Service.Service
	reply.EnterpriseMeta = query.Service.EnterpriseMeta
	reply.Nodes = nodes
	reply.DNS = query.DNS

	// Stamp the result with its this datacenter or peer.
	if peerName := query.Service.PeerName; peerName != "" {
		reply.PeerName = peerName
		reply.Datacenter = ""
	} else {
		reply.Datacenter = p.srv.config.Datacenter
	}

	return nil
}

// tagFilter returns a list of nodes who satisfy the given tags. Nodes must have
// ALL the given tags, and NONE of the forbidden tags (prefixed with !). Note
// for performance this modifies the original slice.
func tagFilter(tags []string, nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	// Build up lists of required and disallowed tags.
	must, not := make([]string, 0), make([]string, 0)
	for _, tag := range tags {
		tag = strings.ToLower(tag)
		if strings.HasPrefix(tag, "!") {
			tag = tag[1:]
			not = append(not, tag)
		} else {
			must = append(must, tag)
		}
	}

	n := len(nodes)
	for i := 0; i < n; i++ {
		node := nodes[i]

		// Index the tags so lookups this way are cheaper.
		index := make(map[string]struct{})
		if node.Service != nil {
			for _, tag := range node.Service.Tags {
				tag = strings.ToLower(tag)
				index[tag] = struct{}{}
			}
		}

		// Bail if any of the required tags are missing.
		for _, tag := range must {
			if _, ok := index[tag]; !ok {
				goto DELETE
			}
		}

		// Bail if any of the disallowed tags are present.
		for _, tag := range not {
			if _, ok := index[tag]; ok {
				goto DELETE
			}
		}

		// At this point, the service is ok to leave in the list.
		continue

	DELETE:
		nodes[i], nodes[n-1] = nodes[n-1], structs.CheckServiceNode{}
		n--
		i--
	}
	return nodes[:n]
}

// nodeMetaFilter returns a list of the nodes who satisfy the given metadata filters. Nodes
// must have ALL the given tags.
func nodeMetaFilter(filters map[string]string, nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	var filtered structs.CheckServiceNodes
	for _, node := range nodes {
		if structs.SatisfiesMetaFilters(node.Node.Meta, filters) {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

func serviceMetaFilter(filters map[string]string, nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	var filtered structs.CheckServiceNodes
	for _, node := range nodes {
		if structs.SatisfiesMetaFilters(node.Service.Meta, filters) {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

// queryServer is a wrapper that makes it easier to test the failover logic.
type queryServer interface {
	GetLogger() hclog.Logger
	GetOtherDatacentersByDistance() ([]string, error)
	GetLocalDC() string
	ExecuteRemote(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error
}

// queryServerWrapper applies the queryServer interface to a Server.
type queryServerWrapper struct {
	srv           *Server
	executeRemote func(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error
}

// GetLocalDC returns the name of the local datacenter.
func (q *queryServerWrapper) GetLocalDC() string {
	return q.srv.config.Datacenter
}

// ExecuteRemote calls ExecuteRemote on PreparedQuery.
func (q *queryServerWrapper) ExecuteRemote(args *structs.PreparedQueryExecuteRemoteRequest, reply *structs.PreparedQueryExecuteResponse) error {
	return q.executeRemote(args, reply)
}

// GetLogger returns the server's logger.
func (q *queryServerWrapper) GetLogger() hclog.Logger {
	return q.srv.loggers.Named(logging.PreparedQuery)
}

// GetOtherDatacentersByDistance calls into the server's fn and filters out the
// server's own DC.
func (q *queryServerWrapper) GetOtherDatacentersByDistance() ([]string, error) {
	// TODO (slackpad) - We should cache this result since it's expensive to
	// compute.
	dcs, err := q.srv.router.GetDatacentersByDistance()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, dc := range dcs {
		if dc != q.srv.config.Datacenter {
			result = append(result, dc)
		}
	}
	return result, nil
}

// queryFailover runs an algorithm to determine which DCs to try and then calls
// them to try to locate alternative services.
func queryFailover(q queryServer, query *structs.PreparedQuery,
	args *structs.PreparedQueryExecuteRequest,
	reply *structs.PreparedQueryExecuteResponse) error {

	// Pull the list of other DCs. This is sorted by RTT in case the user
	// has selected that.
	nearest, err := q.GetOtherDatacentersByDistance()
	if err != nil {
		return err
	}

	// This will help us filter unknown DCs supplied by the user.
	known := make(map[string]struct{})
	for _, dc := range nearest {
		known[dc] = struct{}{}
	}

	// Build a candidate list of DCs to try, starting with the nearest N
	// from RTTs.
	var targets []structs.QueryFailoverTarget
	index := make(map[string]struct{})
	if query.Service.Failover.NearestN > 0 {
		for i, dc := range nearest {
			if !(i < query.Service.Failover.NearestN) {
				break
			}

			targets = append(targets, structs.QueryFailoverTarget{Datacenter: dc})
			index[dc] = struct{}{}
		}
	}

	// Then add any DCs explicitly listed that weren't selected above.
	for _, target := range query.Service.Failover.AsTargets() {
		// This will prevent a log of other log spammage if we do not
		// attempt to talk to datacenters we don't know about.
		if dc := target.Datacenter; dc != "" {
			if _, ok := known[dc]; !ok {
				q.GetLogger().Debug("Skipping unknown datacenter in prepared query", "datacenter", dc)
				continue
			}

			// This will make sure we don't re-try something that fails
			// from the NearestN list.
			if _, ok := index[dc]; !ok {
				targets = append(targets, target)
			}
		}

		if target.PeerName != "" {
			targets = append(targets, target)
		}
	}

	// Now try the selected DCs in priority order.
	failovers := 0
	for _, target := range targets {
		// This keeps track of how many iterations we actually run.
		failovers++

		// Be super paranoid and set the nodes slice to nil since it's
		// the same slice we used before. We know there's nothing in
		// there, but the underlying msgpack library has a policy of
		// updating the slice when it's non-nil, and that feels dirty.
		// Let's just set it to nil so there's no way to communicate
		// through this slice across successive RPC calls.
		reply.Nodes = nil

		// Reset PeerName because it may have been set by a previous failover
		// target.
		query.Service.PeerName = target.PeerName
		dc := target.Datacenter
		if target.PeerName != "" {
			dc = q.GetLocalDC()
		}

		// Note that we pass along the limit since may be applied
		// remotely to save bandwidth. We also pass along the consistency
		// mode information and token we were given, so that applies to
		// the remote query as well.
		remote := &structs.PreparedQueryExecuteRemoteRequest{
			Datacenter:   dc,
			Query:        *query,
			Limit:        args.Limit,
			QueryOptions: args.QueryOptions,
			Connect:      args.Connect,
		}

		if err = q.ExecuteRemote(remote, reply); err != nil {
			q.GetLogger().Warn("Failed querying for service in datacenter",
				"service", query.Service.Service,
				"peerName", query.Service.PeerName,
				"datacenter", dc,
				"error", err,
			)
			continue
		}

		// We can stop if we found some nodes.
		if len(reply.Nodes) > 0 {
			break
		}
	}

	// Set this at the end because the response from the remote doesn't have
	// this information.
	reply.Failovers = failovers

	return nil
}
