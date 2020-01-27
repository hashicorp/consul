---
layout: "docs"
page_title: "ACL Auth Methods"
sidebar_current: "docs-acl-auth-methods"
description: |-
  An auth method is a component in Consul that performs authentication against a trusted external party to authorize the creation of an ACL tokens usable within the local datacenter.
---

-> **1.5.0+:**  This guide only applies in Consul versions 1.5.0 and newer.

# ACL Auth Methods

An auth method is a component in Consul that performs authentication against a
trusted external party to authorize the creation of an ACL tokens usable within
the local datacenter.

The only supported type of auth method in Consul 1.5.0 is
[`kubernetes`](/docs/acl/auth-methods/kubernetes.html).

## Overview

Without an auth method a trusted operator is critically involved in the
creation and secure introduction of each ACL token to every application that
needs one, while ensuring that the policies assigned to these tokens follow the
principle of least-privilege.

When running in environments such as a public cloud or when supervised by a
cluster scheduler, applications may already have access to uniquely identifying
credentials that were delivered securely by the platform. Consul auth method
integrations allow for these credentials to be used to create ACL tokens with
properly-scoped policies without additional operator intervention.

In Consul 1.5.0 the focus is around simplifying the creation of tokens with the
privileges necessary to participate in a [Connect](/docs/connect/index.html)
service mesh with minimal operator intervention.

## Operator Configuration

An operator needs to configure each auth method that is to be trusted by
using the API or command line before they can be used by applications.

* **Authentication** - One or more **auth methods** should be configured with
  details about how to authenticate application credentials. Successful
  validation of application credentials will return a set of trusted identity
  attributes (such as a username). These can be managed with the `consul acl
  auth-method` subcommands or the corresponding [API
  endpoints](/api/acl/auth-methods.html).  The specific details of
  configuration are type dependent and described in their own documentation
  pages.

* **Authorization** - One or more **binding rules** must be configured to define
  how to translate trusted identity attributes from each auth method into
  privileges assigned to the ACL token that is created. These can be managed
  with the `consul acl binding-rule` subcommands or the corresponding [API
  endpoints](/api/acl/binding-rules.html).

-> **Note** - To configure auth methods in any connected secondary datacenter,
[ACL token replication](/docs/agent/options.html#acl_enable_token_replication)
must be enabled.  Auth methods require the ability to create local tokens which
is restricted to the primary datacenter and any secondary datacenters with ACL
token replication enabled.

## Binding Rules

Binding rules allow an operator to express a systematic way of automatically
linking [roles](/docs/acl/acl-system.html#acl-roles) and [service
identities](/docs/acl/acl-system.html#acl-service-identities) to newly created
tokens without operator intervention.

Successful authentication with an auth method returns a set of trusted
identity attributes corresponding to the authenticated identity.  Those
attributes are matched against all configured binding rules for that auth
method to determine what privileges to grant the the Consul ACL token it will
ultimately create.

Each binding rule is composed of two portions:

- **Selector** - A logical query that must match the trusted identity
  attributes for the binding rule to be applicable to a given login attempt.
  The syntax uses github.com/hashicorp/go-bexpr which is shared with the [API
  filtering feature](/api/features/filtering.html).  For example:
  `"serviceaccount.namespace==default and serviceaccount.name!=vault"`

- **Bind Type and Name** - A binding rule can bind a token to a
  [role](/docs/acl/acl-system.html#acl-roles) or to a [service
  identity](/docs/acl/acl-system.html#acl-service-identities) by name. The name
  can be specified with a plain string or the bind name can be lightly
  templated using [HIL syntax](https://github.com/hashicorp/hil) to interpolate
  the same values that are usable by the `Selector` syntax. For example:
  `"dev-${serviceaccount.name}"`

When multiple binding rules match, then all roles and service identities are
jointly linked to the token created by the login process.

## Overall Login Process

Applications are responsible for exchanging their auth method specific secret
bearer token for a Consul ACL token by using the login process:

![diagram of auth method login](/assets/images/auth-methods.svg)

1. Applications use the `consul login` subcommand or the [login API
   endpoint](/api/acl/acl.html#login-to-auth-method) to authenticate to a
   specific auth method using their local Consul client. Applications provide
   both the name of the auth method and a secret bearer token during login.

2. The Consul client forwards login requests to the leading Consul server.

3. The Consul leader then uses auth method specific mechanisms to validate the
   provided bearer token credentials.
   
4. Successful validation returns trusted identity attributes to the Consul
   leader.

5. The Consul leader consults the configured set of binding rules associated
   with the specified auth method and selects only those rules that match the
   trusted identity attributes.

6. The Consul leader uses the matching binding rules to generate a list of
   roles and service identities and assigns them to a token created exclusively
   in the _local_ datacenter. If none are generated the login attempt fails.
   
7. The relevant `SecretID` and remaining details about the token are returned to
   the originating Consul client.

8. The Consul client returns the token details back to the application.

9. (later) Applications SHOULD use the `consul logout` subcommand or the
   [logout API endpoint](/api/acl/acl.html#logout-from-auth-method) to destroy
   their token when it is no longer required.

For more details about specific auth methods and how to configure them, click
on the name of the auth method type in the sidebar.
