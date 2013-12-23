# Agent HTTP API

The Consul agent is capable of running an HTTP server that
exposes various API's in a RESTful manner. These API's can
be used to both query the service catalog, as well as to
register new services.

The URLs are also versioned to allow for changes in the API.
The current URLs supported are:

* /v1/catalog/register : Registers a new service
* /v1/catalog/deregister : Deregisters a service or node
* /v1/catalog/datacenters : Lists known datacenters
* /v1/catalog/nodes : Lists nodes in a given DC
* /v1/catalog/services : Lists services in a given DC
* /v1/catalog/service/<service>/ : Lists the nodes in a given service
* /v1/catalog/node/<node>/ : Lists the services provided by a node

* Health system (future):
* /v1/health/register : Registers a new health check
* /v1/health/deregister : Deregisters a health check
* /v1/health/pass: Pass a health check
* /v1/health/warn: Warn on a health check
* /v1/health/fail: Fail a health check
* /v1/health/node/<node>: Returns the health info of a node
* /v1/health/service/<service>: Returns the health info of a service
* /v1/health/query/<state>: Returns the checks in a given state

* /v1/status/leader : Returns the current Raft leader
* /v1/status/peers : Returns the current Raft peer set

* /v1/agent/services : Returns the services local agent is attempting to register
* /v1/agent/health: Returns the health info from the local agent (future)
* /v1/agent/members : Returns the members as seen by the local serf agent
* /v1/agent/join : Instructs the local agent to join a node
* /v1/agent/members-wan: Returns the consul servers as seen by the wan serf (must be a server)
* /v1/agent/join-wan : Instructs the local agent to join a remote consul server (must be a server)

