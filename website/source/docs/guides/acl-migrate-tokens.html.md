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

This document will briefly describe what changed, and then walk though to tools
and process required to upgrade tokens.

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

TODO: maybe mention DC scoping and Local-only tokens here? Not sure if it's
relevant other than you may want to take advantage of those as you upgrade?

## Rule Translation

Consul 1.4.0 includes some tools to help translate old ACL policy rules to
equivalent new ones to help automate the process of upgrading.

This can be done via the [API]() or [CLI]() commands. A full guided upgrade
process in the UI is coming soon.

The automatic rule translation will produce a policy that is guaranteed
compatible with the old token policy (i.e. uses explicit prefix matches) in many
cases, prefix matches can and should be changed to exact matches if that is all
that is required however the automated translation API prefers compatibility.

## Migration Process

TODO. Not even quite sure what this is yet - pull legacy tokens from old API,
use translate on them somehow, decide how to split policies, create policy for
each distinct input policy, create token for policy?

If that's what we recommend I'm not sure why we'd document it and not just write
a script that does that. Ideally user would hand-curate the existing ones into
minimal new ones without unnecessary prefix matches etc. but I don't now how to
suggest they start on that process really. What's the most important thing here
for release?
