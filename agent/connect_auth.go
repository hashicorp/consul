package agent

import (
	"fmt"
	"strings"

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
// means authorised) and reason string are returned.
//
// If the request input is invalide the error returned will be a
// BadRequestError, if the token doesn't grant necessary access then an
// acl.ErrPermissionDenied error is returned, otherwise error indicates an
// unexpected server failure. If access is denied, no error is returned but the
// first return value is false.
func (a *Agent) ConnectAuthorize(token string,
	req *structs.ConnectAuthorizeRequest) (authz bool, reason string, m *cache.ResultMeta, err error) {
	if req == nil {
		err = BadRequestError{"Invalid request"}
		return
	}

	// We need to have a target to check intentions
	if req.Target == "" {
		err = BadRequestError{"Target service must be specified"}
		return
	}

	// Parse the certificate URI from the client ID
	uri, err := connect.ParseCertURIFromString(req.ClientCertURI)
	if err != nil {
		err = BadRequestError{"ClientCertURI not a valid Connect identifier"}
		return
	}

	uriService, ok := uri.(*connect.SpiffeIDService)
	if !ok {
		err = BadRequestError{"ClientCertURI not a valid Service identifier"}
		return
	}

	// We need to verify service:write permissions for the given token.
	// We do this manually here since the RPC request below only verifies
	// service:read.
	rule, err := a.resolveToken(token)
	if err != nil {
		return
	}
	if rule != nil && !rule.ServiceWrite(req.Target, nil) {
		err = acl.ErrPermissionDenied
		return
	}

	// Validate the trust domain matches ours. Later we will support explicit
	// external federation but not built yet.
	rootArgs := &structs.DCSpecificRequest{Datacenter: a.config.Datacenter}
	raw, _, err := a.cache.Get(cachetype.ConnectCARootName, rootArgs)
	if err != nil {
		return
	}

	roots, ok := raw.(*structs.IndexedCARoots)
	if !ok {
		err = fmt.Errorf("internal error: roots response type not correct")
		return
	}
	if roots.TrustDomain == "" {
		err = fmt.Errorf("Connect CA not bootstrapped yet")
		return
	}
	if roots.TrustDomain != strings.ToLower(uriService.Host) {
		authz = false
		reason = fmt.Sprintf("Identity from an external trust domain: %s",
			uriService.Host)
		return
	}

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
	}
	args.Token = token

	raw, meta, err := a.cache.Get(cachetype.IntentionMatchName, args)
	if err != nil {
		return
	}
	m = &meta

	reply, ok := raw.(*structs.IndexedIntentionMatches)
	if !ok {
		err = fmt.Errorf("internal error: response type not correct")
		return
	}
	if len(reply.Matches) != 1 {
		err = fmt.Errorf("Internal error loading matches")
		return
	}

	// Test the authorization for each match
	for _, ixn := range reply.Matches[0] {
		if auth, ok := uriService.Authorize(ixn); ok {
			authz = auth
			reason = fmt.Sprintf("Matched intention: %s", ixn.String())
			return
		}
	}

	// No match, we need to determine the default behavior. We do this by
	// specifying the anonymous token token, which will get that behavior.
	// The default behavior if ACLs are disabled is to allow connections
	// to mimic the behavior of Consul itself: everything is allowed if
	// ACLs are disabled.
	rule, err = a.resolveToken("")
	if err != nil {
		return
	}
	authz = true
	reason = "ACLs disabled, access is allowed by default"
	if rule != nil {
		authz = rule.IntentionDefaultAllow()
		reason = "Default behavior configured by ACLs"
	}
	return
}
