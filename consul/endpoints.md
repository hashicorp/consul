# Consul RPC Endpoints

Consul provides a few high-level services, each of which exposes
methods. The services exposed are:

* Raft : Used to manipulate Raft from non-leader nodes
* Status : Used to query status information
* Catalog: Used to register, deregister, and query service information
* Health: Used to notify of health checks and changes to health

## Raft Service

The Raft service is used to manipulate the Raft controls on the Leader
node. It is only for internal use. It exposes the following methods:

* Apply : Used to execute a command against the FSM
* AddPeer: Used to add a peer to the group
* RemovePeer: Used to remove a peer from the group

## Status Service

The status service is used to query for various status information
from the Consul service. It exposes the following methods:

* Ping : Used to test connectivity
* Leader : Used to get the address of the leader

## Catalog Service

The catalog service is used to manage service discovery and registration.
Nodes can register the services they provide, and deregister them later.
The service exposes the following methods:

* Register : Registers a node, and potentially a node service
* Deregister : Deregisters a node, and potentially a node service

* ListDatacenters: List the known datacenters
* ListServices : Lists the available services
* ListNodes : Lists the available nodes
* ServiceNodes: Returns the nodes that are part of a service
* NodeServices: Returns the services that a node is registered for

## Health Service

The health service is used to manage health checking. Nodes have system
health checks, as well as application health checks. This service is used to
query health information, as well as for nodes to publish changes.

* CheckPass : Used to inform that a check has passed
* CheckWarn : Used to inform that a check is warning
* CheckFail : Used to inform that a check has failed
* RemoveCheck : Used to remove a health check

* CheckInState : Gets the checks that in a given state
* NodeChecks: Gets the checks a given node has

