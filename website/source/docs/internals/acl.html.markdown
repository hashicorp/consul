---
layout: "docs"
page_title: "ACL System"
sidebar_current: "docs-internals-acl"
---

# ACL System

Consul provides an optional Access Control List (ACL) system which can be used to control
access to data and APIs. The ACL system is an
[Object-Capability system](http://en.wikipedia.org/wiki/Object-capability_model) that relies
on tokens which can have fine grained rules applied to them. It is very similar to
[AWS IAM](http://aws.amazon.com/iam/) in many ways.

## ACL Design

The ACL system is designed to be easy to use, fast to enforce, flexible to new
policies, all while providing administrative insight. It has been modeled on
the AWS IAM system, as well as the more general object-capability model. The system
is modeled around "tokens".

Every token has an ID, name, type and rule set. The ID is a randomly generated
UUID, making it unfeasible to guess. The name is opaque and human readable.
Lastly the type is either "client" meaning it cannot modify ACL rules, and
is restricted by the provided rules, or is "management" and is allowed to
perform all actions.

The token ID is passed along with each RPC request to the servers. Agents
[can be configured](/docs/agent/options.html) with `acl_token` to provide a default token,
but the token can also be specified by a client on a [per-request basis](/docs/agent/http.html).
ACLs are new as of Consul 0.4, meaning versions prior do not provide a token.
This is handled by the special "anonymous" token. Anytime there is no token provided,
the rules defined by that token are automatically applied. This lets policy be enforced
on legacy clients.

Enforcement is always done by the server nodes. All servers must be [configured
to provide](/docs/agent/options.html) an `acl_datacenter`, which enables
ACL enforcement but also specified the authoritative datacenter. Consul does not
replicate data cross-WAN, and instead relies on [RPC forwarding](/docs/internal/architecture.html)
to support Multi-Datacenter configurations. However, because requests can be
made across datacenter boundaries, ACL tokens must be valid globally. To avoid
replication issues, a single datacenter is considered authoritative and stores
all the tokens.

When a request is made to any non-authoritative server with a token, it must
be resolved into the appropriate policy. This is done by reading the token
from the authoritative server and caching a configurable `acl_ttl`. The implication
of caching is that the cache TTL is an upper-bound on the staleness of policy
that is enforced. It is possible to set a zero TTL, but this has adverse
performance impacts, as every request requires refreshing the policy.

Another possible issue is an outage of the `acl_datacenter` or networking
issues preventing access. In this case, it may be impossible for non-authoritative
servers to resolve tokens. Consul provides a number of configurable `acl_down_policy`
choices to tune behavior. It is possible to deny or permit all actions, or to ignore
cache TTLs and enter a fail-safe mode.

ACLs can also act in either a whilelist or blacklist mode depending
on the configuration of `acl_default_policy`. If the default policy is
to deny all actions, then token rules can be set to allow or whitelist
actions. In the inverse, the allow all default behavior is a blacklist,
where rules are used to prohibit actions.

Bootstrapping the ACL system is done by providing an initial `acl_master_token`
[configuration](/docs/agent/options.html), which will be created as a
"management" type token if it does not exist.

## Rule Specification

A core part of the ACL system is a rule language which is used
to describe the policy that must be enforced. We make use of
the [HashiCorp Configuration Language (HCL)](https://github.com/hashicorp/hcl/)
to specify policy. This language is human readable and interoperable
with JSON making it easy to machine generate.

As of Consul 0.4, it is only possible to specify policies for the
KV store. Specification in the HCL format looks like:

    # Default all keys to read-only
    key "" {
        policy = "read"
    }
    key "foo/" {
        policy = "write"
    }
    key "foo/private/" {
        # Deny access to the private dir
        policy = "deny"
    }

This is equivalent to the following JSON input:

    {
        "key": {
            "": {
                "policy": "read",
            },
            "foo/": {
                "policy": "write",
            },
            "foo/private": {
                "policy": "deny",
            }
        }
    }

Key policies provide both a prefix and a policy. The rules are enforced
using a longest-prefix match policy. This means we pick the most specific
policy possible. The policy is either "read", "write" or "deny". A "write"
policy implies "read", and there is no way to specify write-only. If there
is no applicable rule, the `acl_default_policy` is applied.

