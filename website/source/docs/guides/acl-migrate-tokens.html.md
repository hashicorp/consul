---
layout: "docs"
page_title: "ACL Token Migration"
sidebar_current: "docs-guides-acl-migrate-tokens"
description: |-
   Consul 1.4.0 introduces a new ACL system with backward incompatible changes.
   This guide documents how to upgrade the "legacy" tokens after upgrading to
   1.4.0.
---

# ACL Token Migration

Consul 1.4.0 introduces a new ACL system with backward incompatible changes.
This guide documents how to upgrade the "legacy" tokens after upgrading to
1.4.0.

Since the policy syntax changed to be more precise and flexible to manage, it's
necessary to manually translate old tokens into new ones to take advantage of
the new ACL system features. Tooling is provided to help automate this and this
guide describes the overall process.

This document will briefly describe [what changed](#what-changed), and then walk
through the [high-level migration process options](#migration-process), finally
giving some [specific examples](#migration-examples) of migration strategies.

## What Changed

The [ACL guide](/docs/guides/acl.html) and [legacy ACL
guide](/docs/guides/acl-legacy.html) describes the new and old systems in
detail. Below is a summary of the changes that need to be considered when
migrating legacy tokens to the new system.

### Token and Policy Separation

You can use a single policy in the new system for all tokens that share access
rules. For example, all tokens created using the clone endpoint in the legacy
system can be represented with a single policy and a set of tokens that map to
that policy.

### Rule Syntax Changes

The most significant change is that rules with
selectors _no longer prefix match by default_. In the legacy systems the
rules:

```
node "foo" { policy = "write" }
service "foo" { policy = "write" }
key "foo" { policy = "write" }
```

would grant access to all services or keys _prefixed_ with foo. In the new
system the same syntax will only perform _exact_ match on the whole key or
service name.

In general, exact match is what most operators intended most of the time so the
same policy can be kept, however if you rely on prefix match behavior then using
the same syntax will break behavior.

Prefix matching can be expressed in the new ACL system explicitly, making the
following rules in the new system exactly the same as the rules above in the
old:

```
node_prefix "foo" { policy = "write" }
service_prefix "foo" { policy = "write" }
key_prefix "foo" { policy = "write" }
```

## Migration Process

The high-level process for migrating a legacy token is as follows:

 1. Create a new policy or policies that grant the required access
 2. Update the existing token to use those policies

### Pre-requisites

This process assumes that the 1.4.0 upgrade is complete including all Legacy
ACLs having their accessor IDs populated. This might take up to several minutes
after the servers upgrade in the primary datacenter. You can tell if this is the
case by using `consul acl token list` and checking that no tokens exist with a
blank `AccessorID`.

In addition, it is assumed that all clients that might _create_ ACL tokens (e.g. Vault and Nomad) have been updates

### Creating Policies

There are several ways to create new policies. The simplest and most automatic
is to create one new policy for every existing token. This may result in a lot
of policies that are logical duplicates and make managing policies harder later
though. This approach can be easily accomplished using the
`consul acl policy create` command with `-from-token` options.

An alternative is to create one policy for each logically distinct token. While
it's easy enough to do this by hashing the policy contents and only keeping
distinct hashes, it's hard to extract a meaningful name for the policy that
expresses it's intent. To make this easier, there is a CLI and API tool that can
translate a legacy ACL policy into a new ACL policy that is exactly equivalent.
See [`consul acl translate-rules`]().

The final option is to manually inspect all the existing tokens and define named
policies that describe what the policy is intended to be used for. In this case
you may also want to modify the policy for example switching to exact-match
rather than prefix match on resources if that is all that is actually required.
In this case existing ACL token rules can be inspected using the `consul acl
token read -id <accessor_id>` command.

### Updating Existing Tokens

Once you have one or more policies that adequately express the rules needed for
a legacy token, you can update the token via the CLI or API to use those
policies.

After updating, the token is no longer considered "legacy" and will have all the
properties of a new token, however it keeps it's `SecretID` (the secret part of
the token used in API calls etc.) so clients already using that token will
continue to work. It is assumed that the policies attached continue to grant the
necessary access for existing clients but this is up to the operator to ensure.

#### Update via API

Use the [`PUT /v1/acl/token/:AccessorID`](/api/acl/tokens.html#update-a-token)
endpoint. Specifically, ensure that the `Rules` field is omitted or the empty
string. Empty `Rules` is the signal that this is now treated as a new token.

#### Update via CLI

Use the [`consul acl token update`]() command to update the token. Specifically
you need to use `-upgrade-legacy` which will ensure that legacy rules are
removed as well as the new policies added.

## Migration Examples

These examples show specifically how you can achieve a few possible migration
strategies as discussed above.

### Simple Policy Mapping

This strategy uses the CLI to create a new policy for every existing legacy
token with exactly equivalent rules. It's easy to automate and clients will see
no change in behavior for their tokens, but it does leave you with a lot of
potentially identical policies to manage or clean up later.

#### Create Policies

You can get the AccessorID of every legacy token from the API. For example,
using `curl` and `jq` in bash:

```sh
$ LEGACY_IDS=$(curl -sH "X-Consul-Token: $CONSUL_HTTP_TOKEN" \
   'localhost:8500/v1/acl/tokens' | jq -r '.[] | select (.Legacy) | .AccessorID')
$ echo "$LEGACY_IDS"
621cbd12-dde7-de06-9be0-e28d067b5b7f
65cecc86-eb5b-ced5-92dc-f861cf7636fe
ba464aa8-d857-3d26-472c-4d49c3bdae72
```

To create a policy for each one we can use something like:

```sh
for id in $LEGACY_IDS; do \
  consul acl policy create -name "migrated-$id" -from-token $id \
    -description "Migrated from legacy ACL token"; \
done
```

Each policy now has an identical set of rules to the original token. You can
inspect these:

```sh
$ consul acl policy read -name migrated-621cbd12-dde7-de06-9be0-e28d067b5b7f
ID:           573d84bd-8b08-3061-e391-d2602e1b4947
Name:         migrated-621cbd12-dde7-de06-9be0-e28d067b5b7f
Description:  Migrated from legacy ACL token
Datacenters:
Rules:
service_prefix "" {
  policy = "write"
}
```

Notice how the policy here is `service_prefix` and not `service` since the old
ACL syntax was an implicit prefix match. This ensures any clients relying on
prefix matching behavior will still work.

#### Update Tokens

With the policies creates as above, we can automatically upgrade all legacy
tokens.

```sh
for id in $LEGACY_IDS; do \
  consul acl token update -id $id -policy-name "migrated-$id" -upgrade-legacy; \
done
```

The update is now complete, all legacy tokens are now new tokens with identical
secrets and enforcement rules.