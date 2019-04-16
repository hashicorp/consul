---
name: 'Appendix: Production Checklist'
content_length: 6
id: /day-1-operations/production-checklist
layout: content_layout
products_used:
  - Consul
description: Production readiness checklist.
level: Advanced
---

Below is a checklist that can help you deploy your first datacenter. This checklist is not an exhaustive list and you may need to add additional tasks depending on your environment.

## Infrastructure Planning

- Review the [reference diagram](/consul/advanced/day-1-operations/reference-architecture#infrastructure-diagram)
- Review the infrastructure [requirements](/consul/advanced/day-1-operations/reference-architecture#infrastructure-requirements).

### Ports

Refer to the [API documentation](https://www.consul.io/docs/agent/options.html#ports) for specific port numbers or alternate configuration options.

- DNS server
- HTTP API
- HTTPS API
- gRPC API
- Serf LAN port
- Serf WAN port
- Server RPC address
- Inclusive minimum port number to use for automatically assigned sidecar service registrations.
- Sidecar_max_port

## Deployment

### Consul Servers

- Read the [release notes](https://www.consul.io/docs/upgrade-specific.html) for the Consul version.
- [Consul binary](https://www.consul.io/downloads.html) has been distributed to all servers.
- Customize the [server configuration file](https://www.consul.io/docs/agent/options.html) or files.
- [Autopilot](/consul/day-2-operations/advanced-operations/autopilot) is configured or disabled.
- [TLS enabled](/consul/advanced/day-1-operations/agent-encryption#tls-encryption-for-rpc) for RPC communication
- [Gossip encryption](/consul/advanced/day-1-operations/agent-encryption#gossip-encryption) configured
- [Telemetry](/consul/advanced/day-1-operations/monitoring#enable-telemetry) configured.

### Consul Clients

- Consul binary has been distributed to all clients.
- The configuration file has been customized.
- TLS enabled for RPC communication
- Gossip encryption configured
- [External Service Monitor](https://www.consul.io/docs/guides/external.html) has been deployed to nodes that cannot run a Consul client.

## Networking

### Configure DNS Caching

Refer to the [DNS caching guide](/consul/day-2-operations/advanced-operations/dns-caching) for step by step instructions and considerations around DNS performance.

- Stale reads have been configured in the agent configuration file.
- Negative response caching have been configured in the agent configuration file.
- TTL values have been configured in the agent configuration file.

### Setup DNS Forwarding

Refer to the [DNS forwarding](https://www.consul.io/docs/guides/forwarding.html) guide for instructions on integrating Consul with system DNS.

- BIND, dnsmasq, Unbound, systemd-resolved, or iptables has been configured.

## Security

### Encryption of Communication

- TLS: RPC encryption for both incoming and outgoing communication.
- Gossip Encryption. Both incoming and outgoing communication.

### Enable ACLs

Refer to the [ACL guide](/consul/advanced/day-1-operations/acl-guide) for instructions on setting up access control lists.

- Tokens have been created for all agents and services.

### Setup a Certificate Authority

Refer to the [Certificate guide](/consul/advanced/day-1-operations/certificates) for instructions on setting up a certificate authority.

- Agent certificates have been created and distributed to all agents.

## Monitoring

- Telemetry has been enabled.
- API has been configured. New user and token have been created.

## Failure Recovery

- [Backups](/consul/advanced/day-1-operations/backup) are being periodically captured.
- [Outage recovery](/consul/day-2-operations/advanced-operations/outage) plan has been outlined.
