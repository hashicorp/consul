package consul

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/consul/structs"
)

var (
	// ErrQueryNotFound is returned if the query lookup failed.
	ErrQueryNotFound = errors.New("Query not found")
)

// PreparedQuery manages the prepared query endpoint.
type PreparedQuery struct {
	srv *Server
}

// Apply is used to apply a modifying request to the data store. This should
// only be used for operations that modify the data. The ID of the session is
// returned in the reply.
func (p *PreparedQuery) Apply(args *structs.PreparedQueryRequest, reply *string) (err error) {
	if done, err := p.srv.forward("PreparedQuery.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "prepared-query", "apply"}, time.Now())

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
			args.Query.ID = generateUUID()
			_, query, err := state.PreparedQueryGet(args.Query.ID)
			if err != nil {
				return fmt.Errorf("Prepared query lookup failed: %v", err)
			}
			if query == nil {
				break
			}
		}
	}
	*reply = args.Query.ID

	// Grab the ACL because we need it in several places below.
	acl, err := p.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	// Enforce that any modify operation has the same token used when the
	// query was created, or a management token with sufficient rights.
	if args.Op != structs.PreparedQueryCreate {
		state := p.srv.fsm.State()
		_, query, err := state.PreparedQueryGet(args.Query.ID)
		if err != nil {
			return fmt.Errorf("Prepared Query lookup failed: %v", err)
		}
		if query == nil {
			return fmt.Errorf("Cannot modify non-existent prepared query: '%s'", args.Query.ID)
		}
		if (query.Token != args.Token) && (acl != nil && !acl.QueryModify()) {
			p.srv.logger.Printf("[WARN] consul.prepared_query: Operation on prepared query '%s' denied because ACL didn't match ACL used to create the query, and a management token wasn't supplied", args.Query.ID)
			return permissionDeniedErr
		}
	}

	// Parse the query and prep it for the state store.
	switch args.Op {
	case structs.PreparedQueryCreate, structs.PreparedQueryUpdate:
		if err := parseQuery(&args.Query); err != nil {
			return fmt.Errorf("Invalid prepared query: %v", err)
		}

		if acl != nil && !acl.ServiceRead(args.Query.Service.Service) {
			p.srv.logger.Printf("[WARN] consul.prepared_query: Operation on prepared query for service '%s' denied due to ACLs", args.Query.Service.Service)
			return permissionDeniedErr
		}

	case structs.PreparedQueryDelete:
		// Nothing else to verify here, just do the delete (we only look
		// at the ID field for this op).

	default:
		return fmt.Errorf("Unknown prepared query operation: %s", args.Op)
	}

	// At this point the token has been vetted, so make sure the token that
	// is stored in the state store matches what was supplied.
	args.Query.Token = args.Token

	resp, err := p.srv.raftApply(structs.PreparedQueryRequestType, args)
	if err != nil {
		p.srv.logger.Printf("[ERR] consul.prepared_query: Apply failed %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
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
	// - Session is optional and checked for integrity during the transaction.
	// - Token is checked outside this fn.

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
	// Service is required. We check integrity during the transaction.
	if svc.Service == "" {
		return fmt.Errorf("Must provide a service name to query")
	}

	// NearestN can be 0 which means "don't fail over by RTT".
	if svc.Failover.NearestN < 0 {
		return fmt.Errorf("Bad NearestN '%d', must be >= 0", svc.Failover.NearestN)
	}

	// We skip a few fields:
	// - There's no validation for Datacenters; we skip any unknown entries
	//   at execution time.
	// - OnlyPassing is just a boolean so doesn't need further validation.
	// - Tags is a free-form list of tags and doesn't need further validation.

	// Sort order must be one of the allowed values, or if not given we
	// default to "shuffle" so there's load balancing.
	switch svc.Sort {
	case structs.QueryOrderShuffle:
	case structs.QueryOrderSort:
	case "":
		svc.Sort = structs.QueryOrderShuffle
	default:
		return fmt.Errorf("Bad Sort '%s'", svc.Sort)
	}

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

// Execute runs a prepared query and returns the results. This will perform the
// failover logic if no local results are available. This is typically called as
// part of a DNS lookup, or when executing prepared queries from the HTTP API.
func (p *PreparedQuery) Execute(args *structs.PreparedQueryExecuteRequest,
	reply *structs.PreparedQueryExecuteResponse) error {
	if done, err := p.srv.forward("PreparedQuery.Execute", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "prepared-query", "execute"}, time.Now())

	// We have to do this ourselves since we are not doing a blocking RPC.
	if args.RequireConsistent {
		if err := p.srv.consistentRead(); err != nil {
			return err
		}
	}

	// Try to locate the query.
	state := p.srv.fsm.State()
	_, query, err := state.PreparedQueryLookup(args.QueryIDOrName)
	if err != nil {
		return err
	}
	if query == nil {
		return ErrQueryNotFound
	}

	// Execute the query for the local DC.
	if err := p.execute(query, reply); err != nil {
		return err
	}

	// Shuffle the results in case coordinates are not available if they
	// requested an RTT sort.
	reply.Nodes.Shuffle()
	if query.Service.Sort == structs.QueryOrderSort {
		if err := p.srv.sortNodesByDistanceFrom(args.Source, reply.Nodes); err != nil {
			return err
		}
	}

	// In the happy path where we found some healthy nodes we go with that
	// and bail out. Otherwise, we fail over and try remote DCs, as allowed
	// by the query setup.
	if len(reply.Nodes) == 0 {
		wrapper := &queryServerWrapper{p.srv}
		if err := queryFailover(wrapper, query, args.QueryOptions, reply); err != nil {
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
	if done, err := p.srv.forward("PreparedQuery.ExecuteRemote", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "prepared-query", "execute_remote"}, time.Now())

	// We have to do this ourselves since we are not doing a blocking RPC.
	if args.RequireConsistent {
		if err := p.srv.consistentRead(); err != nil {
			return err
		}
	}

	// Run the query locally to see what we can find.
	if err := p.execute(&args.Query, reply); err != nil {
		return err
	}

	// We don't bother trying to do an RTT sort here since we are by
	// definition in another DC. We just shuffle to make sure that we
	// balance the load across the results.
	reply.Nodes.Shuffle()

	return nil
}

// execute runs a prepared query in the local DC without any failover. We don't
// apply any sorting options at this level - it should be done up above.
func (p *PreparedQuery) execute(query *structs.PreparedQuery,
	reply *structs.PreparedQueryExecuteResponse) error {
	state := p.srv.fsm.State()
	_, nodes, err := state.CheckServiceNodes(query.Service.Service)
	if err != nil {
		return err
	}

	// This is kind of a paranoia ACL check, in case something changed with
	// the token from the time the query was registered. Note that we use
	// the token stored with the query, NOT the passed-in one, which is
	// critical to how queries work (the query becomes a proxy for a lookup
	// using the ACL it was created with).
	if err := p.srv.filterACL(query.Token, nodes); err != nil {
		return err
	}

	// Filter out any unhealthy nodes.
	nodes = nodes.Filter(query.Service.OnlyPassing)

	// Apply the tag filters, if any.
	if len(query.Service.Tags) > 0 {
		nodes = tagFilter(query.Service.Tags, nodes)
	}

	// Capture the nodes and pass the DNS information through to the reply.
	reply.Nodes = nodes
	reply.DNS = query.DNS

	return nil
}

// tagFilter returns a list of nodes who satisfy the given tags. Nodes must have
// ALL the given tags, and none of the forbidden tags (prefixed with !).
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
		for _, tag := range node.Service.Tags {
			tag = strings.ToLower(tag)
			index[tag] = struct{}{}
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

// queryServer is a wrapper that makes it easier to test the failover logic.
type queryServer interface {
	GetOtherDatacentersByDistance() ([]string, error)
	ForwardDC(method, dc string, args interface{}, reply interface{}) error
}

// queryServerWrapper applies the queryServer interface to a Server.
type queryServerWrapper struct {
	srv *Server
}

// ForwardDC calls into the server's RPC forwarder.
func (q *queryServerWrapper) ForwardDC(method, dc string, args interface{}, reply interface{}) error {
	return q.srv.forwardDC(method, dc, args, reply)
}

// GetOtherDatacentersByDistance calls into the server's fn and filters out the
// server's own DC.
func (q *queryServerWrapper) GetOtherDatacentersByDistance() ([]string, error) {
	dcs, err := q.srv.getDatacentersByDistance()
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
	options structs.QueryOptions,
	reply *structs.PreparedQueryExecuteResponse) error {

	// Build a candidate list of DCs, starting with the nearest N from RTTs.
	var dcs []string
	index := make(map[string]struct{})
	if query.Service.Failover.NearestN > 0 {
		nearest, err := q.GetOtherDatacentersByDistance()
		if err != nil {
			return err
		}

		for i, dc := range nearest {
			if !(i < query.Service.Failover.NearestN) {
				break
			}

			dcs = append(dcs, dc)
			index[dc] = struct{}{}
		}
	}

	// Then add any DCs explicitly listed that weren't selected above.
	for _, dc := range query.Service.Failover.Datacenters {
		_, ok := index[dc]
		if !ok {
			dcs = append(dcs, dc)
		}
	}

	// Now try the selected DCs in priority order.
	for _, dc := range dcs {
		remote := &structs.PreparedQueryExecuteRemoteRequest{
			Datacenter:   dc,
			Query:        *query,
			QueryOptions: options,
		}
		if err := q.ForwardDC("PreparedQuery.ExecuteRemote", dc, remote, reply); err != nil {
			return err
		}

		// We can stop if we found some nodes.
		if len(reply.Nodes) > 0 {
			break
		}
	}

	return nil
}
