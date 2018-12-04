# Production Ready Checklist

## [Reference Diagram](link back to guide) 

##### Infrastructure architecture. 
- [] Server platform and resources needed. 

##### Possible open [ports](https://www.consul.io/docs/agent/options.html#ports).
- [] DNS server
- [] HTTP API
- [] HTTPS API 
- [] gRPC API
- [] Serf LAN port
- [] Serf WAN port
- []Server RPC address
- [] Inclusive minimum port number to use for automatically assigned sidecar service registrations. 
- [] Sidecar_max_port

## Deployment

##### Consul servers.
- [] Read the [release notes](https://www.consul.io/docs/upgrade-specific.html) for the Consul version.
- [] [Consul binary](https://www.consul.io/downloads.html) has been distributed to all servers.
- [] Customize the [server configuration file](https://www.consul.io/docs/agent/options.html) or files. 
- [] [Autopilot](link to learn) is configured or disabled. 
- [] [TLS enabled](link to learn) for RPC communication
- [] [Gossip encryption](link to learn) configured
- [][Telemetry](link to learn) configured.

##### Consul clients. 
- [] Consul binary has been distributed to all clients.
- [] The configuration file has been customized.
- [] TLS enabled for RPC communication
- [] Gossip encryption configured
- [] [External Service Monitor](https://www.consul.io/docs/guides/external.html) has been deployed to nodes that cannot run a Consul client. 

## Networking  

##### [DNS caching](link to learn) has been configured. 
- [] Stale reads have been configured in the agent configuration file.
- [] Negative response caching have been configured in the agent configuration file.
- [] TTL values have been configured in the agent configuration file.

##### [DNS forwarding](https://www.consul.io/docs/guides/forwarding.html) has been setup.
- [] BIND, dnsmasq, Unbound, systemd-resolved, or iptables has been configured.

## Security

##### Encryption of communication.
- [] TLS: RPC encryption for both incoming and outgoing communication.
- [] Gossip Encryption. Both incoming and outgoing communication. 

##### [ACLs](https://www.consul.io/docs/guides/acl.html) have been enabled and configured. 
- [] Tokens have been created for all agents and services. 

##### [Certificate Authority](link to learn) setup. 
- [] Agent certificates have been created and distributed to all agents.

## Monitoring

- [] Telemetry has been enabled.
- [] API configured. New user and token have been created. 

## Failure Recovery
- [] [Backups](link to learn) are being periodically captured. 
- [] [Outage recovery](link to learn) plan has been outlined.

