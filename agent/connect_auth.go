package agent

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

// TODO(rb/intentions): this should move back into the agent endpoint since
// there is no ext_authz implementation anymore.
//
// ConnectAuthorize implements the core authorization logic for Connect. It's in
// a separate agent method here because we need to re-use this both in our own
// HTTP API authz endpoint and in the gRPX xDS/ext_authz API for envoy.
//
// NOTE: This treats any L7 intentions as DENY.
//
// The ACL token and the auth request are provided and the auth decision (true
// means authorized) and reason string are returned.
//
// If the request input is invalid the error returned will be a BadRequest HTTPError,
// if the token doesn't grant necessary access then an acl.ErrPermissionDenied
// error is returned, otherwise error indicates an unexpected server failure. If
// access is denied, no error is returned but the first return value is false.
func (a *Agent) ConnectAuthorize(token string,
	req *structs.ConnectAuthorizeRequest) (allowed bool, reason string, m *cache.ResultMeta, err error) {

	// Helper to make the error cases read better without resorting to named
	// returns which get messy and prone to mistakes in a method this long.
	returnErr := func(err error) (bool, string, *cache.ResultMeta, error) {
		return false, "", nil, err
	}

	if req == nil {
		return returnErr(HTTPError{StatusCode: http.StatusBadRequest, Reason: "Invalid request"})
	}

	// We need to have a target to check intentions
	if req.Target == "" {
		return returnErr(HTTPError{StatusCode: http.StatusBadRequest, Reason: "Target service must be specified"})
	}

	// Parse the certificate URI from the client ID
	uri, err := connect.ParseCertURIFromString(req.ClientCertURI)
	if err != nil {
		return returnErr(HTTPError{StatusCode: http.StatusBadRequest, Reason: "ClientCertURI not a valid Connect identifier"})
	}

	uriService, ok := uri.(*connect.SpiffeIDService)
	if !ok {
		return returnErr(HTTPError{StatusCode: http.StatusBadRequest, Reason: "ClientCertURI not a valid Service identifier"})
	}

	// We need to verify service:write permissions for the given token.
	// We do this manually here since the RPC request below only verifies
	// service:read.
	var authzContext acl.AuthorizerContext
	authz, err := a.delegate.ResolveTokenAndDefaultMeta(token, &req.EnterpriseMeta, &authzContext)
	if err != nil {
		return returnErr(err)
	}

	if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(req.Target, &authzContext); err != nil {
		return returnErr(err)
	}

	if !uriService.MatchesPartition(req.TargetPartition()) {
		reason = fmt.Sprintf("Mismatched partitions: %q != %q",
			uriService.PartitionOrDefault(),
			acl.PartitionOrDefault(req.TargetPartition()))
		return false, reason, nil, nil
	}

	// Note that we DON'T explicitly validate the trust-domain matches ours. See
	// the PR for this change for details.

	// TODO(banks): Implement revocation list checking here.

	// Get the intentions for this target service.
	args := &structs.IntentionQueryRequest{
		Datacenter: a.config.Datacenter,
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: req.TargetNamespace(),
					Partition: req.TargetPartition(),
					Name:      req.Target,
				},
			},
		},
		QueryOptions: structs.QueryOptions{Token: token},
	}

	raw, meta, err := a.cache.Get(context.TODO(), cachetype.IntentionMatchName, args)
	if err != nil {
		return returnErr(err)
	}

	reply, ok := raw.(*structs.IndexedIntentionMatches)
	if !ok {
		return returnErr(fmt.Errorf("internal error: response type not correct"))
	}
	if len(reply.Matches) != 1 {
		return returnErr(fmt.Errorf("Internal error loading matches"))
	}

	// Figure out which source matches this request.
	var ixnMatch *structs.Intention
	for _, ixn := range reply.Matches[0] {
		// We match on the intention source because the uriService is the source of the connection to authorize.
		if _, ok := connect.AuthorizeIntentionTarget(
			uriService.Service, uriService.Namespace, uriService.Partition, ixn, structs.IntentionMatchSource); ok {
			ixnMatch = ixn
			break
		}
	}

	if ixnMatch != nil {
		if len(ixnMatch.Permissions) == 0 {
			// This is an L4 intention.
			reason = fmt.Sprintf("Matched L4 intention: %s", ixnMatch.String())
			auth := ixnMatch.Action == structs.IntentionActionAllow
			return auth, reason, &meta, nil
		}

		// This is an L7 intention, so DENY.
		reason = fmt.Sprintf("Matched L7 intention: %s", ixnMatch.String())
		return false, reason, &meta, nil
	}

	reason = "Default behavior configured by ACLs"
	return authz.IntentionDefaultAllow(nil) == acl.Allow, reason, &meta, nil
}
