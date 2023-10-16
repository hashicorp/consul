package external

import (
	"github.com/hashicorp/go-uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
)

// We tag logs with a unique identifier to ease debugging. In the future this
// should probably be a real Open Telemetry trace ID.
func TraceID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		return ""
	}
	return id
}

type ACLResolver interface {
	ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (resolver.Result, error)
}

// RequireAnyValidACLToken checks that the caller provided a valid ACL token
// without requiring any specific permissions. This is useful for endpoints
// that are used by all/most consumers of our API, such as those called by the
// consul-server-connection-manager library when establishing a new connection.
//
// Note: no token is required if ACLs are disabled.
func RequireAnyValidACLToken(resolver ACLResolver, token string) error {
	authz, err := resolver.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	if id := authz.ACLIdentity; id != nil && id.ID() == structs.ACLTokenAnonymousID {
		return status.Error(codes.Unauthenticated, "An ACL token must be provided (via the `x-consul-token` metadata field) to call this endpoint")
	}

	return nil
}
