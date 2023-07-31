package consul

import "github.com/hashicorp/go-hclog"

// Operator endpoint is used to perform low-level operator tasks for Consul.
type Operator struct {
	srv    *Server
	logger hclog.Logger
}
