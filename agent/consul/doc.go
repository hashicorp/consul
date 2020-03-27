// Package consul Consul HTTP API.
//
// The main interface to Consul is a RESTful HTTP API.
// The API can perform basic CRUD operations on nodes, services, checks, configuration, and more.
//
// Authentication
// When authentication is enabled, a Consul token should be provided to API requests using the
// X-Consul-Token header or with the Bearer scheme in the authorization header.
// This reduces the probability of the token accidentally getting logged or exposed.
// When using authentication, clients should communicate via TLS.
// If you don’t provide a token in the request, then the agent default token will be used.
//
// Terms Of Service:
//
// there are no TOS at this moment, use at your own risk we take no responsibility
//
//     Schemes: http, https
//     Host: localhost:8500
//     BasePath: /v1
//     Version: 1.0
//     License: MPL "https://github.com/hashicorp/consul/blob/master/LICENSE
//     Contact: Security HashiCorp <security@hashicorp.com> https://www.consul.io/
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Security:
//     - x_consul_token:
//
//     SecurityDefinitions:
//     x_consul_token:
//          type: apiKey
//          name: X-Consul-Token
//          in: header
//
// swagger:meta
package consul
