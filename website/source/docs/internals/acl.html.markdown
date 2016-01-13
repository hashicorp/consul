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

Enforcement is always done by the server nodes. All servers must be configured
to provide an [`acl_datacenter`](/docs/agent/options.html#acl_datacenter) which
enables ACL enforcement but also specifies the authoritative datacenter. Consul does not
replicate data cross-WAN and instead relies on [RPC forwarding](/docs/internals/architecture.html)
to support Multi-Datacenter configurations. However, because requests can be made
across datacenter boundaries, ACL tokens must be valid globally. To avoid
replication issues, a single datacenter is considered authoritative and stores
all the tokens.

When a request is made to a server in a non-authoritative datacenter server, it
must be resolved into the appropriate policy. This is done by reading the token
from the authoritative server and caching the result for a configurable
[`acl_ttl`](/docs/agent/options.html#acl_ttl). The implication
of caching is that the cache TTL is an upper bound on the staleness of policy
that is enforced. It is possible to set a zero TTL, but this has adverse
performance impacts, as every request requires refreshing the policy via a
cross-datacenter WAN call.

The Consul ACL system is designed with flexible rules to accommodate for an outage
of the [`acl_datacenter`](/docs/agent/options.html#acl_datacenter) or networking
issues preventing access to it. In this case, it may be impossible for
servers in non-authoritative datacenters to resolve tokens. Consul provides
a number of configurable [`acl_down_policy`](/docs/agent/options.html#acl_down_policy)
choices to tune behavior. It is possible to deny or permit all actions or to ignore
cache TTLs and enter a fail-safe mode. The default is to ignore cache TTLs
for any previously resolved tokens and to deny any uncached tokens.

ACLs can also act in either a whitelist or blacklist mode depending
on the configuration of
[`acl_default_policy`](/docs/agent/options.html#acl_default_policy). If the
default policy is to deny all actions, then token rules can be set to whitelist
specific actions. In the inverse, the allow all default behavior is a blacklist
where rules are used to prohibit actions. By default, Consul will allow all
actions.

### Blacklist mode and `consul exec`

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

### Blacklist mode and Service Discovery

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

### Blacklist mode and Events

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

### Blacklist mode and Keyring Operations

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

### Bootstrapping ACLs

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
that must be enforced. Consul supports ACLs for both [K/Vs](/intro/getting-started/kv.html)
and [services](/intro/getting-started/services.html).

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
  "keyring": "read"
}
```

## Services and Checks with ACLs

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
## Restricting service discovery with ACLs

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
