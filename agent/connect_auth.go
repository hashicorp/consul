package agent

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

// ConnectAuthorize implements the core authorization logic for Connect. It's in
// a separate agent method here because we need to re-use this both in our own
// HTTP API authz endpoint and in the gRPX xDS/ext_authz API for envoy.
//
// The ACL token and the auth request are provided and the auth decision (true
// means authorized) and reason string are returned.
//
// If the request input is invalid the error returned will be a BadRequestError,
// if the token doesn't grant necessary access then an acl.ErrPermissionDenied
// error is returned, otherwise error indicates an unexpected server failure. If
// access is denied, no error is returned but the first return value is false.
func (a *Agent) ConnectAuthorize(token string,
	req *structs.ConnectAuthorizeRequest) (authz bool, reason string, m *cache.ResultMeta, err error) {

	// Helper to make the error cases read better without resorting to named
	// returns which get messy and prone to mistakes in a method this long.
	returnErr := func(err error) (bool, string, *cache.ResultMeta, error) {
		return false, "", nil, err
	}

	if req == nil {
		return returnErr(BadRequestError{"Invalid request"})
	}

	// We need to have a target to check intentions
	if req.Target == "" {
		return returnErr(BadRequestError{"Target service must be specified"})
	}

	// Parse the certificate URI from the client ID
	uri, err := connect.ParseCertURIFromString(req.ClientCertURI)
	if err != nil {
		return returnErr(BadRequestError{"ClientCertURI not a valid Connect identifier"})
	}

	uriService, ok := uri.(*connect.SpiffeIDService)
	if !ok {
		return returnErr(BadRequestError{"ClientCertURI not a valid Service identifier"})
	}

	// We need to verify service:write permissions for the given token.
	// We do this manually here since the RPC request below only verifies
	// service:read.
	rule, err := a.resolveToken(token)
	if err != nil {
		return returnErr(err)
	}
	if rule != nil && !rule.ServiceWrite(req.Target, nil) {
		return returnErr(acl.ErrPermissionDenied)
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
					Namespace: structs.IntentionDefaultNamespace,
					Name:      req.Target,
				},
			},
		},
		QueryOptions: structs.QueryOptions{Token: token},
	}

	raw, meta, err := a.cache.Get(cachetype.IntentionMatchName, args)
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

	// Test the authorization for each match
	for _, ixn := range reply.Matches[0] {
		if auth, ok := uriService.Authorize(ixn); ok {
			reason = fmt.Sprintf("Matched intention: %s", ixn.String())
			return auth, reason, &meta, nil
		}
	}

	// No match, we need to determine the default behavior. We do this by
	// specifying the anonymous token, which will get the default behavior. The
	// default behavior if ACLs are disabled is to allow connections to mimic the
	// behavior of Consul itself: everything is allowed if ACLs are disabled.
	rule, err = a.resolveToken("")
	if err != nil {
		return returnErr(err)
	}
	if rule == nil {
		// ACLs not enabled at all, the default is allow all.
		return true, "ACLs disabled, access is allowed by default", &meta, nil
	}
	reason = "Default behavior configured by ACLs"
	return rule.IntentionDefaultAllow(), reason, &meta, nil
}
