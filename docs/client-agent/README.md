# Client Agent

- agent/cache
- [agent/local](https://github.com/hashicorp/consul/tree/main/agent/local)
- anti-entropy sync in [agent/ae](https://github.com/hashicorp/consul/tree/main/agent/ae) powering the [Anti-Entropy Sync Back](https://developer.hashicorp.com/docs/internals/anti-entropy.html) process to the Consul servers.

Applications on client nodes use their local agent in client mode to [register services](https://developer.hashicorp.com/api/agent.html) and to discover other services or interact with the key/value store. 
