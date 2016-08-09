---
layout: "docs"
page_title: "ACL System"
sidebar_current: "docs-internals-acl"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL system is a Capability-based system that relies on tokens which can have fine grained rules applied to them. It is very similar to AWS IAM in many ways.
---

# ACL System

Consul provides an optional Access Control List (ACL) system which can be used to control
access to data and APIs. The ACL is
[Capability-based](https://en.wikipedia.org/wiki/Capability-based_security), relying
on tokens to which fine grained rules can be applied. It is very similar to
[AWS IAM](http://aws.amazon.com/iam/) in many ways.

## Scope

When the ACL system was launched in Consul 0.4, it was only possible to specify
policies for the KV store.  In Consul 0.5, ACL policies were extended to service
registrations. In Consul 0.6, ACL's were further extended to restrict service
discovery mechanisms, user events, and encryption keyring operations.

## ACL Design

The ACL system is designed to be easy to use, fast to enforce, and flexible to new
policies, all while providing administrative insight.

Every token has an ID, name, type, and rule set. The ID is a randomly generated
UUID, making it unfeasible to guess. The name is opaque to Consul and human readable.
The type is either "client" (meaning the token cannot modify ACL rules) or "management"
(meaning the token is allowed to perform all actions).

The token ID is passed along with each RPC request to the servers. Agents
can be configured with an [`acl_token`](/docs/agent/options.html#acl_token) property
to provide a default token, but the token can also be specified by a client on a
[per-request basis](/docs/agent/http.html). ACLs were added in Consul 0.4, meaning
prior versions do not provide a token. This is handled by the special "anonymous"
token. If no token is provided, the rules associated with the anonymous token are
automatically applied: this allows policy to be enforced on legacy clients.

ACLs can also act in either a whitelist or blacklist mode depending
on the configuration of
[`acl_default_policy`](/docs/agent/options.html#acl_default_policy). If the
default policy is to deny all actions, then token rules can be set to whitelist
specific actions. In the inverse, the allow all default behavior is a blacklist
where rules are used to prohibit actions. By default, Consul will allow all
actions.

#### ACL Datacenter

Enforcement is always done by the server nodes. All servers must be configured
to provide an [`acl_datacenter`](/docs/agent/options.html#acl_datacenter) which
enables ACL enforcement but also specifies the authoritative datacenter. Consul
relies on [RPC forwarding](/docs/internals/architecture.html) to support
Multi-Datacenter configurations. However, because requests can be made
across datacenter boundaries, ACL tokens must be valid globally. To avoid
consistency issues, a single datacenter is considered authoritative and stores
the canonical set of tokens.

When a request is made to a server in a non-authoritative datacenter server, it
must be resolved into the appropriate policy. This is done by reading the token
from the authoritative server and caching the result for a configurable
[`acl_ttl`](/docs/agent/options.html#acl_ttl). The implication
of caching is that the cache TTL is an upper bound on the staleness of policy
that is enforced. It is possible to set a zero TTL, but this has adverse
performance impacts, as every request requires refreshing the policy via a
cross-datacenter WAN RPC call.

#### Outages and ACL Replication

The Consul ACL system is designed with flexible rules to accommodate for an outage
of the [`acl_datacenter`](/docs/agent/options.html#acl_datacenter) or networking
issues preventing access to it. In this case, it may be impossible for
servers in non-authoritative datacenters to resolve tokens. Consul provides
a number of configurable [`acl_down_policy`](/docs/agent/options.html#acl_down_policy)
choices to tune behavior. It is possible to deny or permit all actions or to ignore
cache TTLs and enter a fail-safe mode. The default is to ignore cache TTLs
for any previously resolved tokens and to deny any uncached tokens.

<a name="replication"></a>
Consul 0.7 added an ACL Replication capability that can allow non-authoritative
datacenter servers to resolve even uncached tokens. This is enabled by setting an
[`acl_replication_token`](/docs/agent/options.html#acl_replication_token) in the
configuration on the servers in the non-authoritative datacenters. With replication
enabled, the servers will maintain a replica of the authoritative datacenter's full
set of ACLs on the non-authoritative servers.

Replication occurs with a background process that looks for new ACLs approximately
every 30 seconds. Replicated changes are written at a rate that's throttled to
100 updates/second, so it may take several minutes to perform the initial sync of
a large set of ACLs.

If there's a partition or other outage affecting the authoritative datacenter,
and the [`acl_down_policy`](/docs/agent/options.html#acl_down_policy)
is set to "extend-cache", tokens will be resolved during the outage using the
replicated set of ACLs. An [ACL replication status](http://localhost:4567/docs/agent/http/acl.html#acl_replication_status)
endpoint is available to monitor the health of the replication process.

Locally-resolved ACLs will be cached using the [`acl_ttl`](/docs/agent/options.html#acl_ttl)
setting of the non-authoritative datacenter, so these entries may persist in the
cache for up to the TTL, even after the authoritative datacenter comes back online.

ACL replication can also be used to migrate ACLs from one datacenter to another
using a process like this:

1. Enable ACL replication in all datacenters to allow continuation of service
during the migration, and to populate the target datacenter. Verify replication
is healthy and caught up to the current ACL index in the target datacenter
using the [ACL replication status](http://localhost:4567/docs/agent/http/acl.html#acl_replication_status)
endpoint.
2. Turn down the old authoritative datacenter servers.
3. Rolling restart the servers in the target datacenter and change the
`acl_datacenter` configuration to itself. This will automatically turn off
replication and will enable the datacenter to start acting as the authoritative
datacenter, using its replicated ACLs from before.
3. Rolling restart the servers in other datacenters and change their `acl_datacenter`
configuration to the target datacenter.

#### Bootstrapping ACLs

Bootstrapping the ACL system is done by providing an initial [`acl_master_token`
configuration](/docs/agent/options.html#acl_master_token) which will be created
as a "management" type token if it does not exist. Note that the [`acl_master_token`
](/docs/agent/options.html#acl_master_token) is only installed when a server acquires
cluster leadership. If you would like to install or change the
[`acl_master_token`](/docs/agent/options.html#acl_master_token), set the new value for
[`acl_master_token`](/docs/agent/options.html#acl_master_token) in the configuration
for all servers. Once this is done, restart the current leader to force a leader election.

## Rule Specification

A core part of the ACL system is a rule language which is used to describe the policy
that must be enforced.

Key policies are defined by coupling a prefix with a policy. The rules are enforced
using a longest-prefix match policy: Consul picks the most specific policy possible. The
policy is either "read", "write", or "deny". A "write" policy implies "read", and there is no
way to specify write-only. If there is no applicable rule, the
[`acl_default_policy`](/docs/agent/options.html#acl_default_policy) is applied.

Service policies are defined by coupling a service name and a policy. The rules are
enforced using an longest-prefix match policy (this was an exact match in 0.5, but changed
in 0.5.1). The default rule, applied to any service that doesn't have a matching policy,
is provided using the empty string. A service policy is either "read", "write", or "deny".
A "write" policy implies "read", and there is no way to specify write-only. If there is no
applicable rule, the [`acl_default_policy`](/docs/agent/options.html#acl_default_policy) is
applied. The "read" policy in a service ACL rule allows restricting access to
the discovery of that service prefix. More information about service discovery
and ACLs can be found [below](#discovery_acls).

The policy for the "consul" service is always "write" as it is managed internally by Consul.

User event policies are defined by coupling an event name prefix with a policy.
The rules are enforced using a longest-prefix match policy. The default rule,
applied to any user event without a matching policy, is provided by an empty
string. An event policy is one of "read", "write", or "deny". Currently, only
the "write" level is enforced during event firing. Events can always be read.

Prepared query policies control access to create, update, and delete prepared
queries. Service policies are used when executing prepared queries. See
[below](#prepared_query_acls) for more details.

We make use of
the [HashiCorp Configuration Language (HCL)](https://github.com/hashicorp/hcl/)
to specify policy. This language is human readable and interoperable
with JSON making it easy to machine-generate.

Specification in the HCL format looks like:

```javascript
# Default all keys to read-only
key "" {
  policy = "read"
}
key "foo/" {
  policy = "write"
}
key "foo/private/" {
  # Deny access to the dir "foo/private"
  policy = "deny"
}

# Default all services to allow registration. Also permits all
# services to be discovered.
service "" {
    policy = "write"
}

# Deny registration access to services prefixed "secure-".
# Discovery of the service is still allowed in read mode.
service "secure-" {
    policy = "read"
}

# Allow firing any user event by default.
event "" {
    policy = "write"
}

# Deny firing events prefixed with "destroy-".
event "destroy-" {
    policy = "deny"
}

# Default prepared queries to read-only.
query "" {
    policy = "read"
}

# Read-only mode for the encryption keyring by default (list only)
keyring = "read"
```

This is equivalent to the following JSON input:

```javascript
{
  "key": {
    "": {
      "policy": "read"
    },
    "foo/": {
      "policy": "write"
    },
    "foo/private": {
      "policy": "deny"
    }
  },
  "service": {
      "": {
          "policy": "write"
      },
      "secure-": {
          "policy": "read"
      }
  },
  "event": {
    "": {
      "policy": "write"
    },
    "destroy-": {
      "policy": "deny"
    }
  },
  "query": {
    "": {
      "policy": "read"
    }
  },
  "keyring": "read"
}
```

## Building ACL Policies

#### Blacklist mode and `consul exec`

If you set [`acl_default_policy`](/docs/agent/options.html#acl_default_policy)
to `deny`, the `anonymous` token won't have permission to read the default
`_rexec` prefix; therefore, Consul agents using the `anonymous` token
won't be able to perform [`consul exec`](/docs/commands/exec.html) actions.

Here's why: the agents need read/write permission to the `_rexec` prefix for
[`consul exec`](/docs/commands/exec.html) to work properly. They use that prefix
as the transport for most data.

You can enable [`consul exec`](/docs/commands/exec.html) from agents that are not
configured with a token by allowing the `anonymous` token to access that prefix.
This can be done by giving this rule to the `anonymous` token:

```javascript
key "_rexec/" {
    policy = "write"
}
```

Alternatively, you can, of course, add an explicit
[`acl_token`](/docs/agent/options.html#acl_token) to each agent, giving it access
to that prefix.

#### Blacklist mode and Service Discovery

If your [`acl_default_policy`](/docs/agent/options.html#acl_default_policy) is
set to `deny`, the `anonymous` token will be unable to read any service
information. This will cause the service discovery mechanisms in the REST API
and the DNS interface to return no results for any service queries. This is
because internally the API's and DNS interface consume the RPC interface, which
will filter results for services the token has no access to.

You can allow all services to be discovered, mimicing the behavior of pre-0.6.0
releases, by configuring this ACL rule for the `anonymous` token:

```
service "" {
    policy = "read"
}
```

Note that the above will allow access for reading service information only. This
level of access allows discovering other services in the system, but is not
enough to allow the agent to sync its services and checks into the global
catalog during [anti-entropy](/docs/internals/anti-entropy.html).

The most secure way of handling service registration and discovery is to run
Consul 0.6+ and issue tokens with explicit access for the services or service
prefixes which are expected to run on each agent.

#### Blacklist mode and Events

Similar to the above, if your
[`acl_default_policy`](/docs/agent/options.html#acl_default_policy) is set to
`deny`, the `anonymous` token will have no access to allow firing user events.
This deviates from pre-0.6.0 builds, where user events were completely
unrestricted.

Events have their own first-class expression in the ACL syntax. To restore
access to user events from arbitrary agents, configure an ACL rule like the
following for the `anonymous` token:

```
event "" {
    policy = "write"
}
```

As always, the more secure way to handle user events is to explicitly grant
access to each API token based on the events they should be able to fire.

#### Blacklist mode and Prepared Queries

After Consul 0.6.3, significant changes were made to ACLs for prepared queries,
including a new `query` ACL policy. See [Prepared Query ACLs](#prepared_query_acls) below for more details.

#### Blacklist mode and Keyring Operations

Consul 0.6 and later supports securing the encryption keyring operations using
ACL's. Encryption is an optional component of the gossip layer. More information
about Consul's keyring operations can be found on the [keyring
command](/docs/commands/keyring.html) documentation page.

If your [`acl_default_policy`](/docs/agent/options.html#acl_default_policy) is
set to `deny`, then the `anonymous` token will not have access to read or write
to the encryption keyring. The keyring policy is yet another first-class citizen
in the ACL syntax. You can configure the anonymous token to have free reign over
the keyring using a policy like the following:

```
keyring = "write"
```

Encryption keyring operations are sensitive and should be properly secured. It
is recommended that instead of configuring a wide-open policy like above, a
per-token policy is applied to maximize security.

#### Services and Checks with ACLs

Consul allows configuring ACL policies which may control access to service and
check registration. In order to successfully register a service or check with
these types of policies in place, a token with sufficient privileges must be
provided to perform the registration into the global catalog. Consul also
performs periodic [anti-entropy](/docs/internals/anti-entropy.html) syncs, which
may require an ACL token to complete. To accommodate this, Consul provides two
methods of configuring ACL tokens to use for registration events:

1. Using the [acl_token](/docs/agent/options.html#acl_token) configuration
   directive. This allows a single token to be configured globally and used
   during all service and check registration operations.
2. Providing an ACL token with service and check definitions at
   registration time. This allows for greater flexibility and enables the use
   of multiple tokens on the same agent. Examples of what this looks like are
   available for both [services](/docs/agent/services.html) and
   [checks](/docs/agent/checks.html). Tokens may also be passed to the
   [HTTP API](/docs/agent/http.html) for operations that require them.

<a name="discovery_acls"></a>
#### Restricting service discovery with ACLs

In Consul 0.6, the ACL system was extended to support restricting read access to
service registrations. This allows tighter access control and limits the ability
of a compromised token to discover other services running in a cluster.

The ACL system permits a user to discover services using the REST API or UI if
the token used during requests has "read"-level access or greater. Consul will
filter out all services which the token has no access to in all API queries,
making it appear as though the restricted services do not exist.

Consul's DNS interface is also affected by restrictions to service
registrations. If the token used by the agent does not have access to a given
service, then the DNS interface will return no records when queried for it.

<a name="prepared_query_acls"></a>
## Prepared Query ACLs

As we've gotten feedback from Consul users, we've evolved how prepared queries
use ACLs. In this section we first cover the current implementation, and then we
follow that with details about what's changed between specific versions of Consul.

#### Managing Prepared Queries

Managing prepared queries includes creating, reading, updating, and deleting
queries. There are a few variations, each of which uses ACLs in one of two
ways: open, protected by unguessable IDs or closed, managed by ACL policies.
These variations are covered here, with examples:

* Static queries with no `Name` defined are not controlled by any ACL policies.
  These types of queries are meant to be ephemeral and not shared to untrusted
  clients, and they are only reachable if the prepared query ID is known. Since
  these IDs are generated using the same random ID scheme as ACL Tokens, it is
  infeasible to guess them. When listing all prepared queries, only a management
  token will be able to see these types, though clients can read instances for
  which they have an ID. An example use for this type is a query built by a
  startup script, tied to a session, and written to a configuration file for a
  process to use via DNS.

* Static queries with a `Name` defined are controlled by the
  [`query`](/docs/internals/acl.html#prepared_query_acls) ACL policy.
  Clients are required to have an ACL token with a prefix sufficient to cover
  the name they are trying to manage, with a longest prefix match providing a
  way to define more specific policies. Clients can list or read queries for
  which they have "read" access based on their prefix, and similar they can
  update any queries for which they have "write" access. An example use for
  this type is a query with a well-known name (eg. `prod-master-customer-db`)
  that is used and known by many clients to provide geo-failover behavior for
  a database.

* [Template queries](https://www.consul.io/docs/agent/http/query.html#templates)
  queries work like static queries with a `Name` defined, except that a catch-all
  template with an empty `Name` requires an ACL token that can write to any query
  prefix.

#### Executing Pepared Queries

When prepared queries are executed via DNS lookups or HTTP requests, the ACL
checks are run against the service being queried, similar to how ACLs work with
other service lookups. There are several ways the ACL token is selected for this
check:

* If an ACL Token was captured when the prepared query was defined, it will be
  used to perform the service lookup. This allows queries to be executed by
  clients with lesser or even no ACL Token, so this should be used with care.

* If no ACL Token was captured, then the client's ACL Token will be used to
  perform the service lookup.

* If no ACL Token was captured and the client has no ACL Token, then the
  anonymous token will be used to perform the service lookup.

In the common case, the ACL Token of the invoker is used
to test the ability to look up a service. If a `Token` was specified when the
prepared query was created, the behavior changes and now the captured
ACL Token set by the definer of the query is used when lookup up a service.

Capturing ACL Tokens is analogous to
[PostgreSQL’s](http://www.postgresql.org/docs/current/static/sql-createfunction.html)
`SECURITY DEFINER` attribute which can be set on functions, and using the client's ACL
Token is similar to the complementary `SECURITY INVOKER` attribute.

<a name="prepared_query_acl_changes"></a>
#### ACL Implementation Changes for Prepared Queries

Prepared queries were originally introduced in Consul 0.6.0, and ACL behavior remained
unchanged through version 0.6.3, but was then changed to allow better management of the
prepared query namespace.

These differences are outlined in the table below:

<table class="table table-bordered table-striped">
  <tr>
    <th>Operation</th>
    <th>Version <= 0.6.3 </th>
    <th>Version > 0.6.3 </th>
  </tr>
  <tr>
    <td>Create static query without `Name`</td>
    <td>The ACL Token used to create the prepared query is checked to make sure it can access the service being queried. This token is captured as the `Token` to use when executing the prepared query.</td>
    <td>No ACL policies are used as long as no `Name` is defined. No `Token` is captured by default unless specifically supplied by the client when creating the query.</td>
  </tr>
  <tr>
    <td>Create static query with `Name`</td>
    <td>The ACL Token used to create the prepared query is checked to make sure it can access the service being queried. This token is captured as the `Token` to use when executing the prepared query.</td>
    <td>The client token's `query` ACL policy is used to determine if the client is allowed to register a query for the given `Name`. No `Token` is captured by default unless specifically supplied by the client when creating the query.</td>
  </tr>
  <tr>
    <td>Manage static query without `Name`</td>
    <td>The ACL Token used to create the query, or a management token must be supplied in order to perform these operations.</td>
    <td>Any client with the ID of the query can perform these operations.</td>
  </tr>
  <tr>
    <td>Manage static query with a `Name`</td>
    <td>The ACL token used to create the query, or a management token must be supplied in order to perform these operations.</td>
    <td>Similar to create, the client token's `query` ACL policy is used to determine if these operations are allowed.</td>
  </tr>
  <tr>
    <td>List queries</td>
    <td>A management token is required to list any queries.</td>
    <td>The client token's `query` ACL policy is used to determine which queries they can see. Only management tokens can see prepared queries without `Name`.</td>
  </tr>
  <tr>
    <td>Execute query</td>
    <td>Since a `Token` is always captured when a query is created, that is used to check access to the service being queried. Any token supplied by the client is ignored.</td>
    <td>The captured token, client's token, or anonymous token is used to filter the results, as described above.</td>
  </tr>
</table>
