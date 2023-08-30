package proxysnapshot

import "github.com/hashicorp/consul/acl"

// ProxySnapshot is an abstraction that allows interchangeability between
// Catalog V1 ConfigSnapshot and Catalog V2 ProxyState.
type ProxySnapshot interface {
	AllowEmptyListeners() bool
	AllowEmptyRoutes() bool
	AllowEmptyClusters() bool
	Authorize(authz acl.Authorizer) error
	LoggerName() string
}

// CancelFunc is a type for a returned function that can be called to cancel a
// watch.
type CancelFunc func()
