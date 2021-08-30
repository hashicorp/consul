# ACL

This section is a work in progress.

The ACL subsystem is responsible for authenticating and authorizing access to Consul
operations ([HTTP API], and [RPC]). 

[HTTP API]: ../http-api
[RPC]: ../rpc

## ACL Entities

There are many entities in the ACL subsystem. The diagram below shows the relationship
between the entities.

![Entity Relationship Diagram](./erd.svg)

<sup>[source](./erd.mmd)</sup>

ACL Tokens are at the center of the ACL system. Tokens are associated with a set of
Policies, and Roles.

AuthMethods, which consist of BindingRules, are a mechanism for creating ACL Tokens from
policies stored in external systems (ex: kubernetes, JWT, or OIDC).

Roles are a set of policies associated with a named role, and ServiceIdentity and
NodeIdentity are policy templates that are associated with a specific service or node and
can be rendered into a full policy.

Each Policy contains a set of rules. Each rule relates to a specific resource, and
includes an AccessLevel (read, write, list or deny).

An ACL Token can be resolved into an Authorizer. The Authorizer is what is used by the
[HTTP API], and [RPC] endpoints to determine if an operation is allowed or forbidden (the
enforcement decision).
