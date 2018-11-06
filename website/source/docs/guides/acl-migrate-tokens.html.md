---
layout: "docs"
page_title: "ACL Token Migration"
sidebar_current: "docs-guides-acl-migrate-tokens"
description: |-
  Consul 1.4.0 introduces a new ACL system with improvements for the security and
  management of ACL tokens and policies. This guide documents how to upgrade
  existing (now called "legacy") tokens after upgrading to 1.4.0.
---

# ACL Token Migration

Consul 1.4.0 introduces a new ACL system with improvements for the security and
management of ACL tokens and policies. This guide documents how to upgrade
existing (now called "legacy") tokens after upgrading to 1.4.0.

Since the policy syntax changed to be more precise and flexible to manage, it's
necessary to manually translate old tokens into new ones to take advantage of
the new ACL system features. Tooling is provided to help automate this and this
guide describes the overall process.

~> **Note:** **1.4.0 retains full support for "legacy" ACL tokens**
so in-place upgrade is safe and existing tokens will continue to work in the
same way for at least two "major" releases (1.5.x, 1.6.x, etc; note HashiCorp
does not use the SemVer definitions for our products).

This document will briefly describe [what changed](#what-changed), and then walk
through the [high-level migration process options](#migration-process), finally
giving some [specific examples](#migration-examples) of migration strategies.

## New ACL System Differences

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

### API Separation

The "old" API endpoints below continue to work for backwards compatibility but
will continue to create or show only "legacy" tokens that can't take full
advantage of the new ACL system improvements. They are documented fully under
[Legacy Tokens](/api/acl/legacy.html).

- [`PUT /acl/create` - Create Legacy Token](/api/acl/legacy.html#create-acl-token)
- [`PUT /acl/update` - Update Legacy Token](/api/acl/legacy.html#update-acl-token)
- [`PUT /acl/destroy/:uuid` - Delete Legacy Token](/api/acl/legacy.html#delete-acl-token)
- [`GET /acl/info/:uuid` - Read Legacy Token](/api/acl/legacy.html#read-acl-token)
- [`PUT /acl/clone/:uuid` - Clone Legacy Token](/api/acl/legacy.html#clone-acl-token)
- [`GET /acl/list` - List Legacy Tokens](/api/acl/legacy.html#list-acls)

The new ACL system includes new API endpoints to manage
the [ACL System](/api/acl/acl.html), [Tokens](/api/acl/tokens.html)
and [Policies](/api/acl/policies.html).

## Migration Process

While "legacy" tokens will continue to work for several major releases, it's
advisable to plan on migrating existing tokens as soon as is convenient.
Migrating also enables using the new policy management improvements, stricter
policy syntax rules and other features of the new system without
re-issuing all the secrets in use.

The high-level process for migrating a legacy token is as follows:

 1. Create a new policy or policies that grant the required access
 2. Update the existing token to use those policies

### Pre-requisites

This process assumes that the 1.4.0 upgrade is complete including all Legacy
ACLs having their accessor IDs populated. This might take up to several minutes
after the servers upgrade in the primary datacenter. You can tell if this is the
case by using `consul acl token list` and checking that no tokens exist with a
blank `AccessorID`.

In addition, it is assumed that all clients that might _create_ ACL tokens (e.g.
Vault's Consul secrets engine) have been updated to use the [new ACL
APIs](/docs/guides/acl-migrate-tokens.html#api-separation).

Specifically if you are using Vault's Consul secrets engine you need to be
running Vault 1.0.0 or higher, _and_ you must update all roles defined in Vault
to specify a list of policy names rather than an inline policy (which causes
Vault to use the legacy API).

~> **Note:** if you have systems still creating "legacy" tokens with the old
APIs, the migration steps below will still work, however you'll have to keep
re-running them until nothing is creating legacy tokens to unsure all tokens are
migrated.

### Creating Policies

There are several ways to create new policies. The simplest and most automatic
is to create one new policy for every existing token. This may however result in
a lot of policies that are logical duplicates and make managing policies harder.
This approach can be easily accomplished using the [`consul acl policy
create`](/docs/commands/acl/acl-policy.html#create) command with `-from-token`
option.

An alternative is to create one policy for each logically distinct token. While
it's easy enough to do this by hashing the policy contents and only keeping
distinct hashes, it's hard to extract a meaningful name for the policy that
expresses it's intent. To make this easier, there is a CLI and API tool that can
translate a legacy ACL policy into a new ACL policy that is exactly equivalent.
See [`consul acl translate-rules`](/docs/commands/acl/acl-translate-rules.html).

The final option is to manually inspect all the existing tokens and define named
policies that describe what the policy is intended to be used for. In this case
you may also want to modify the policy for example switching to exact-match
rather than prefix match on resources if that is all that is actually required.
In this case existing ACL token rules can be inspected using the [`consul acl
token read -id <accessor_id>`](/docs/commands/acl/acl-token.html#read) command.

### Updating Existing Tokens

Once you have one or more policies that adequately express the rules needed for
a legacy token, you can update the token via the CLI or API to use those
policies.

After updating, the token is no longer considered "legacy" and will have all the
properties of a new token, however it keeps it's `SecretID` (the secret part of
the token used in API calls) so clients already using that token will continue
to work. It is assumed that the policies you attach continue to grant the
necessary access for existing clients; this is up to the operator to ensure.

#### Update via API

Use the [`PUT /v1/acl/token/:AccessorID`](/api/acl/tokens.html#update-a-token)
endpoint. Specifically, ensure that the `Rules` field is omitted or empty. Empty
`Rules` indicates that this is now treated as a new token.

#### Update via CLI

Use the [`consul acl token update`](/docs/commands/acl/acl-token.html#update)
command to update the token. Specifically you need to use `-upgrade-legacy`
which will ensure that legacy rules are removed as well as the new policies
added.

## Migration Examples

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

### Combining Policies

This strategy has more manual elements but results in a cleaner and more
managable set of policies than the more automatic solutions. Note that this is
**just an example** to illustrate one way of migrating policies it may not be
appropriate for some users, and likely shouldn't be carried out ad-hoc in a
terminal as demonstrated in production.

#### Find All Unique Policies

You can get the AccessorID of every legacy token from the API. For example,
using `curl` and `jq` in bash:

```sh
$ LEGACY_IDS=$(curl -sH "X-Consul-Token: $CONSUL_HTTP_TOKEN" \
   'localhost:8500/v1/acl/tokens' | jq -r '.[] | select (.Legacy) | .AccessorID')
$ echo "$LEGACY_IDS"
8b65fdf9-303e-0894-9f87-e71b3273600c
d9deb39b-1b30-e100-b9c5-04aba3f593a1
f2bce42e-cdcc-848d-28ca-cfd0556a22e3
```

Now we want to read the actual policy for each legacy token and de-duplicate
them. We can use the `translate-rules` helper sub-command which will read the
token's policy and return a new ACL policy that is exactly equivalent.

```sh
$ for id in $LEGACY_IDS; do \
  echo "Policy for $id:"
  consul acl translate-rules -token-accessor "$id"; \
done
Policy for 8b65fdf9-303e-0894-9f87-e71b3273600c:
service_prefix "bar" {
  policy = "write"
}
Policy for d9deb39b-1b30-e100-b9c5-04aba3f593a1:
service_prefix "foo" {
  policy = "write"
}
Policy for f2bce42e-cdcc-848d-28ca-cfd0556a22e3:
service_prefix "bar" {
  policy = "write"
}
```

Notice that two policies are the same and one different.

We can change the loop above to take a hash of this policy definition to
de-duplicate the policies into a set of files locally. This example uses command
available on macOS but equivalents for other platforms should be easy to find.

```sh
$ mkdir policies
$ for id in $LEGACY_IDS; do \
  # Fetch the equivalent new policy rules based on the legacy token rules
  NEW_POLICY=$(consul acl translate-rules -token-accessor "$id"); \
  # Sha1 hash the rules
  HASH=$(echo -n "$NEW_POLICY" | shasum | awk '{ print $1 }'); \
  # Write rules to a policy file named with the hash to de-duplicated
  echo "$NEW_POLICY" > policies/$HASH.hcl; \
done
$ tree policies
policies
├── 024ce11f26f59436c518fb31f0999d1400485c17.hcl
└── 501b787c9444fbd62f346ab257eeb27197be2444.hcl
```

### Cleaning Up Policies

You can now manually inspect and potentially edit these policies. For example we
could rename them according to their intended use. In this case we maintain the
hash as it will allow us to match tokens to policies later.

```sh
$ cat policies/024ce11f26f59436c518fb31f0999d1400485c17.hcl
service_prefix "bar" {
  policy = "write"
}
$ mv policies/024ce11f26f59436c518fb31f0999d1400485c17.hcl \
  policies/024ce11f26f59436c518fb31f0999d1400485c17-bar-service.hcl
```

You might also choose to tighten up the rules, for example if you know you never
rely on prefix-matching the service name `foo` you might choose to modify the
policy to use exact match.

```sh
$ cat policies/501b787c9444fbd62f346ab257eeb27197be2444.hcl
service_prefix "foo" {
  policy = "write"
}
$ echo 'service "foo" { policy = "write" }' > policies/501b787c9444fbd62f346ab257eeb27197be2444.hcl
$ mv policies/501b787c9444fbd62f346ab257eeb27197be2444.hcl \
  policies/501b787c9444fbd62f346ab257eeb27197be2444-foo-service.hcl
```

We now have a minimal set of policies to create.

### Creating Policies

```sh
$ for p in $(ls policies | grep ".hcl"); do \
  # Extract the hash part of the file name
  HASH=$(echo "$p" | cut -d - -f 1); \
  # Extract the name suffix without .hcl
  NAME=$(echo "$p" | cut -d - -f 2- | cut -d . -f 1); \
  # Create new policy based on the rules in the file and the name we gave
  consul acl policy create -name $NAME \
    -rules "@policies/$p" \
    -description "Migrated from legacy token"; \
done
ID:           da2a9f9b-4e44-13f8-e308-76ce7a8dcb21
Name:         bar-service
Description:  Migrated from legacy token
Datacenters:
Rules:
service_prefix "bar" {
  policy = "write"
}

ID:           9fbded86-9140-efe4-b661-c8bd07b6c584
Name:         foo-service
Description:  Migrated from legacy token
Datacenters:
Rules:
service "foo" { policy = "write" }

```

### Upgrading Tokens

Finally we can map our existing tokens back to those policies using the policy
file names.

```sh
$ for id in $LEGACY_IDS; do \
  NEW_POLICY=$(consul acl translate-rules -token-accessor "$id"); \
  HASH=$(echo -n "$NEW_POLICY" | shasum | awk '{ print $1 }'); \
  # Lookup the hash->new policy mapping from the policy file names
  POLICY_FILE=$(ls policies | grep "^$HASH"); \
  POLICY_NAME=$(echo "$POLICY_FILE" | cut -d - -f 2- | cut -d . -f 1); \
  echo "==> Mapping token $id to policy $POLICY_NAME"; \
  consul acl token update -id $id -policy-name $POLICY_NAME -upgrade-legacy; \
done
==> Mapping token 8b65fdf9-303e-0894-9f87-e71b3273600c to policy bar-service
Token updated successfully.
AccessorID:   8b65fdf9-303e-0894-9f87-e71b3273600c
SecretID:     3dbb3981-7654-733a-3475-5ce20fc5a7b9
Description:
Local:        false
Create Time:  0001-01-01 00:00:00 +0000 UTC
Policies:
   da2a9f9b-4e44-13f8-e308-76ce7a8dcb21 - bar-service
==> Mapping token d9deb39b-1b30-e100-b9c5-04aba3f593a1 to policy foo-service
Token updated successfully.
AccessorID:   d9deb39b-1b30-e100-b9c5-04aba3f593a1
SecretID:     5f54733b-4c76-eb74-8781-3550c20f4969
Description:
Local:        false
Create Time:  0001-01-01 00:00:00 +0000 UTC
Policies:
   9fbded86-9140-efe4-b661-c8bd07b6c584 - foo-service
==> Mapping token f2bce42e-cdcc-848d-28ca-cfd0556a22e3 to policy bar-service
Token updated successfully.
AccessorID:   f2bce42e-cdcc-848d-28ca-cfd0556a22e3
SecretID:     f3aaa3e2-2c6f-cf3c-1e86-454de728e8ab
Description:
Local:        false
Create Time:  0001-01-01 00:00:00 +0000 UTC
Policies:
   da2a9f9b-4e44-13f8-e308-76ce7a8dcb21 - bar-service
```

At this point all tokes are upgraded and can use new ACL features while
retaining the same secret clients are already using.