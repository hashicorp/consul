---
layout: "docs"
page_title: "ACL Auth Methods"
sidebar_current: "docs-acl-auth-methods"
description: |-
  An Auth Method is a component in Consul that performs authentication against a trusted external party to authorize the creation of an appropriately scoped ACL Token usable within the local datacenter.
---

-> **1.5.0+:**  This guide only applies in Consul versions 1.5.0 and later.

# ACL Auth Methods

An Auth Method is a component in Consul that performs authentication against a
trusted external party to authorize the creation of an appropriately scoped ACL
Token usable within the local datacenter.

The only supported type of auth method in Consul 1.5 is
[`kubernetes`](/docs/acl/auth-methods/kubernetes.html).

## Overview

Without auth methods a trusted operator needs to be critically involved in the
creation and secure introduction of each ACL Token to every application that
needs one, while ensuring that the policies assigned to these tokens follow the
principle of least-privilege.

When running in environments such as a public cloud or when supervised by a
cluster scheduler, applications may already have access to uniquely identifying
credentials that were delivered securely by the platform. Consul auth method
integrations allow for these credentials to be used to create ACL Tokens with
properly-scoped policies without additional operator intervention.

In Consul 1.5 the focus is around simplifying the creation of tokens with the
privileges necessary to participate in a [Connect](/docs/connect/index.html)
service mesh with minimal operator intervention.

## Operator Configuration

An operator needs to configure each auth method that is to be trusted by
using the API or command line before they can be used by applications.

* **Authentication** - One or more **auth methods** should be configured with
  details about how to authenticate application credentials. These can be
  managed with the `consul acl auth-method` subcommands or the corresponding
  [API endpoints](/api/acl/auth-methods.html). The specific details of
  configuration are type dependent and described in their own documentation
  pages.

* **Authorization** - One or more **binding rules** must be configured defining
  how to translate trusted identity attributes from each auth method into
  privileges assigned to the ACL Token that is created. These can be managed
  with the `consul acl binding-rule` subcommands or the corresponding [API
  endpoints](/api/acl/binding-rules.html).

## Binding Rules

Once an auth method has been been used to successfully validate a user-provided
secret bearer token and mapped it to a set of trusted identity attributes,
those attributes are matched against all configured Binding Rules for that auth
method.

Binding rules allow an operator to express a systematic way to automatically
[roles](/docs/acl/acl-system.html#acl-roles) and [service
identities](/docs/acl/acl-system.html#acl-service-identities) to newly created
Tokens without operator intervention.

Each binding rule is composed of two portions:

- **Selector** - A logical query that must match the trusted identity
  attributes for the binding rule to be applicable to a given login attempt.
  The syntax uses github.com/hashicorp/go-bexpr which is shared with the [API
  filtering feature](/api/features/filtering.html).  For example:
  `"serviceaccount.namespace==default and serviceaccount.name!=vault"`

- **Bind Type and Name** - A binding rule can bind a token to a
  [role](/docs/acl/acl-system.html#acl-roles) or to a [service
  identity](/docs/acl/acl-system.html#acl-service-identities) by name. The name
  can be specified with a plain string, or the bind name can be lightly
  templated using [HIL syntax](https://github.com/hashicorp/hil) to interpolate
  the same values that are usable by the `Selector` syntax. For example:
  `"dev-${serviceaccount.name}"`

When multiple binding rules match then all roles and service identities are
jointly linked to the token created by the login process.

## Overall Login Process

![diagram of auth method login](/assets/images/auth-methods.svg)

1. Applications use the `consul login` subcommand or the [login API
   endpoint](/api/acl/acl.html#login-to-auth-method) to authenticate to a
   specific auth method using their local Consul Client. Applications provide
   both the name of the auth method and a secret bearer token during login.

2. The Consul Client forwards login requests to the leading Consul Server.

3. The Consul leader uses auth method specific mechanisms to validate the
   provided bearer token credentials.
   
4. Successful validation returns trusted identity attributes to the Consul
   leader.

5. The Consul leader consults the configured set of binding rules associated
   with the chosen auth method and selects only those that match the trusted
   identity attributes.

6. Bound roles and service identites are computed. If none are computed the
   login attempt fails. The bound roles and service identities are assigned to
   a newly-created ACL Token created exclusively in the _local_ datacenter.
   
7. The relevant `SecretID` and remaining details about the token are returned to
   the originating Consul client.

8. The Consul client returns the token details back to the application.

9. (later) Applications SHOULD use the `consul logout` subcommand or the
   [logout API endpoint](/api/acl/acl.html#logout-from-auth-method) to destroy
   their token when it is no longer required.

For more details about specific auth methods and how to configure them, consult
the type-specific docs linked in the sidebar.

