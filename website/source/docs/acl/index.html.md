---
layout: "docs"
page_title: "ACL Guides"
sidebar_current: "docs-acl-index"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. Select the following guide for your use case.
---

# ACL Documentation and Guides

Consul uses Access Control Lists (ACLs) to secure the UI, API, CLI, service
communications, and agent communications. At the core, ACLs operate by grouping
rules into policies, then associating one or more policies with a token.

The following documentation and guides will help you understand and implement
ACLs.

## ACL Documentation

### ACL System

Consul provides an optional Access Control List (ACL) system which can be used
to control access to data and APIs. The ACL system is a Capability-based system
that relies on tokens which can have fine grained rules applied to them. The
[ACL System documentation](/docs/acl/acl-system.html) details the functionality
of Consul ACLs.

### ACL Rules

A core part of the ACL system is the rule language, which is used to describe
the policy that must be enforced. Read the ACL rules
[documentation](/docs/acl/acl-rules.html) to learn about rule specifications. 

### ACL Auth Methods

An auth method is a component in Consul that performs authentication against a
trusted external party to authorize the creation of an ACL tokens usable within
the local datacenter. Read the ACL auth method
[documentation](/docs/acl/acl-auth-methods.html) to learn more about how they
work and why you may want to use them.

### ACL Legacy System

The ACL system in Consul 1.3.1 and older is now called legacy. For information
on bootstrapping the legacy system, ACL rules, and a general ACL system
overview, read the legacy [documentation](/docs/acl/acl-legacy.html).

### ACL Migration

[The migration documentation](/docs/acl/acl-migrate-tokens.html) details how to
upgrade existing legacy tokens after upgrading to 1.4.0. It will briefly
describe what changed, and then walk through the high-level migration process
options, finally giving some specific examples of migration strategies. The new
ACL system has improvements for the security and management of ACL tokens and
policies.

## Learn ACL Guide

~> Note: the following guide is located on HashiCorp Learn. By selecting it,
you will be directed to a new site.

### Securing Consul with ACLs

In this guide, you will learn how to secure the UI, API, CLI, service
communications, and agent communications with ACLs. When securing your cluster
you should configure the ACLs first. The ACL documentation introduces basic
concepts and syntax for the ACL system, and we recommend that you read it
before you begin [this
guide](https://learn.hashicorp.com/consul/security-networking/production-acls).
