---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-acl"
description: |-
  The ACL options allows a number of sub-keys to be set which controls the ACL system.
---


# ACL 

This object allows a number of sub-keys to be set which controls the ACL system. To learn more about the ACL system, read the reference documentation. 

ACLs must be configured in the agent's configuration file, there are no command-line options. 

## ACL Options


* <a name="acl"></a><a href="#acl">`acl`</a> - This object allows a number
    of sub-keys to be set which controls the ACL system. Configuring the
    ACL system within the ACL stanza was added in Consul 1.4.0

    The following sub-keys are available:

     * <a name="acl_enabled"></a><a href="#acl_enabled">`enabled`</a> - Enables ACLs.

     * <a name="acl_policy_ttl"></a><a href="#acl_policy_ttl">`policy_ttl`</a> - Used to control
     Time-To-Live caching of ACL policies. By default, this is 30 seconds. This setting has a
     major performance impact: reducing it will cause more frequent refreshes while increasing
     it reduces the number of refreshes. However, because the caches are not actively invalidated,
     ACL policy may be stale up to the TTL value.

     * <a name="acl_role_ttl"></a><a href="#acl_role_ttl">`role_ttl`</a> - Used to control
     Time-To-Live caching of ACL roles. By default, this is 30 seconds. This setting has a
     major performance impact: reducing it will cause more frequent refreshes while increasing
     it reduces the number of refreshes. However, because the caches are not actively invalidated,
     ACL role may be stale up to the TTL value.

     * <a name="acl_token_ttl"></a><a href="#acl_token_ttl">`token_ttl`</a> - Used to control
     Time-To-Live caching of ACL tokens. By default, this is 30 seconds. This setting has a
     major performance impact: reducing it will cause more frequent refreshes while increasing
     it reduces the number of refreshes. However, because the caches are not actively invalidated,
     ACL token may be stale up to the TTL value.

     * <a name="acl_down_policy"></a><a href="#acl_down_policy">`down_policy`</a> - Either
     "allow", "deny", "extend-cache" or "async-cache"; "extend-cache" is the default. In the case that a
     policy or token cannot be read from the [`primary_datacenter`](#primary_datacenter) or leader
     node, the down policy is applied. In "allow" mode, all actions are permitted, "deny" restricts
     all operations, and "extend-cache" allows any cached objects to be used, ignoring their TTL
     values. If a non-cached ACL is used, "extend-cache" acts like "deny".
     The value "async-cache" acts the same way as "extend-cache" but performs updates
     asynchronously when ACL is present but its TTL is expired, thus, if latency is bad between
     the primary and secondary datacenters, latency of operations is not impacted.

     * <a name="acl_default_policy"></a><a href="#acl_default_policy">`default_policy`</a> - Either
     "allow" or "deny"; defaults to "allow" but this will be changed in a future major release.
     The default policy controls the behavior of a token when there is no matching rule. In "allow" mode,
     ACLs are a blacklist: any operation not specifically prohibited is allowed. In "deny" mode, ACLs are
     a whitelist: any operation not specifically allowed is blocked. *Note*: this will not take effect until
     you've enabled ACLs.

     * <a name="acl_enable_key_list_policy"></a><a href="#acl_enable_key_list_policy">`enable_key_list_policy`</a> - Either "enabled" or "disabled", defaults to "disabled". When enabled, the `list` permission will be required on the prefix being recursively read from the KV store. Regardless of being enabled, the full set of KV entries under the prefix will be filtered to remove any entries that the request's ACL token does not grant at least read permissions. This option is only available in Consul 1.0 and newer.

     * <a name="acl_enable_token_replication"></a><a href="#acl_enable_token_replication">`enable_token_replication`</a> - By
     default secondary Consul datacenters will perform replication of only ACL policies and roles.
     Setting this configuration will will enable ACL token replication and
     allow for the creation of both [local tokens](/api/acl/tokens.html#local)
     and [auth methods](/docs/acl/acl-auth-methods.html) in connected secondary
     datacenters.

     * <a name="acl_enable_token_persistence"></a><a href="#acl_enable_token_persistence">`enable_token_persistence`</a> - Either
    `true` or `false`. When `true` tokens set using the API will be persisted to disk and reloaded when an agent restarts.

     * <a name="acl_tokens"></a><a href="#acl_tokens">`tokens`</a> - This object holds
     all of the configured ACL tokens for the agents usage.

        * <a name="acl_tokens_master"></a><a href="#acl_tokens_master">`master`</a> - Only used
          for servers in the [`primary_datacenter`](#primary_datacenter). This token will be created with management-level
          permissions if it does not exist. It allows operators to bootstrap the ACL system
          with a token Secret ID that is well-known.
          <br/><br/>
          The `master` token is only installed when a server acquires cluster leadership. If
          you would like to install or change the `acl_master_token`, set the new value for `master`
          in the configuration for all servers. Once this is done, restart the current leader to force a
          leader election. If the `master` token is not supplied, then the servers do not create a master
          token. When you provide a value, it should be a UUID. To maintain backwards compatibility
          and an upgrade path this restriction is not currently enforced but will be in a future major
          Consul release.

        * <a name="acl_tokens_default"></a><a href="#acl_tokens_default">`default`</a> - When provided,
        the agent will use this token when making requests to the Consul servers. Clients can
        override this token on a per-request basis by providing the "?token" query parameter.
        When not provided, the empty token, which maps to the 'anonymous' ACL token, is used.

        * <a name="acl_tokens_agent"></a><a href="#acl_tokens_agent">`agent`</a> - Used for clients
        and servers to perform internal operations. If this isn't specified, then the
        <a href="#acl_tokens_default">`default`</a> will be used.
        <br/><br/>
        This token must at least have write access to the node name it will register as in order to set any
        of the node-level information in the catalog such as metadata, or the node's tagged addresses.

        * <a name="acl_tokens_agent_master"></a><a href="#acl_tokens_agent_master">`agent_master`</a> -
        Used to access <a href="/api/agent.html">agent endpoints</a> that require agent read
        or write privileges, or node read privileges, even if Consul servers aren't present to validate
        any tokens. This should only be used by operators during outages, regular ACL tokens should normally
        be used by applications.

        * <a name="acl_tokens_replication"></a><a href="#acl_tokens_replication">`replication`</a> -
        The ACL token used to authorize secondary datacenters with the primary datacenter for replication
        operations. This token is required for servers outside the [`primary_datacenter`](#primary_datacenter) when
        ACLs are enabled. This token may be provided later using the [agent token API](/api/agent.html#update-acl-tokens)
        on each server. This token must have at least "read" permissions on ACL data but if ACL
        token replication is enabled then it must have "write" permissions. This also enables
        Connect replication in Consul Enterprise, for which the token will require both operator
        "write" and intention "read" permissions for replicating CA and Intention data.

* <a name="acl_datacenter"></a><a href="#acl_datacenter">`acl_datacenter`</a> - **This field is
  deprecated in Consul 1.4.0. See the [`primary_datacenter`](#primary_datacenter) field instead.**

    This designates the datacenter which is authoritative for ACL information. It must be provided to enable ACLs. All servers and datacenters must agree on the ACL datacenter. Setting it on the servers is all you need for cluster-level enforcement, but for the APIs to forward properly from the clients,
    it must be set on them too. In Consul 0.8 and later, this also enables agent-level enforcement
    of ACLs. Please see the [ACL Guide](https://learn.hashicorp.com/consul/security-networking/production-acls) for more details.

* <a name="acl_default_policy_legacy"></a><a href="#acl_default_policy_legacy">`acl_default_policy`</a> - **Deprecated in Consul 1.4.0.
  See the [`acl.default_policy`](#acl_default_policy) field instead.** Either
  "allow" or "deny"; defaults to "allow". The default policy controls the behavior of a token when
  there is no matching rule. In "allow" mode, ACLs are a blacklist: any operation not specifically
  prohibited is allowed. In "deny" mode, ACLs are a whitelist: any operation not
  specifically allowed is blocked. *Note*: this will not take effect until you've set `primary_datacenter`
  to enable ACL support.

* <a name="acl_down_policy_legacy"></a><a href="#acl_down_policy_legacy">`acl_down_policy`</a> - **Deprecated in Consul 1.4.0.
  See the [`acl.down_policy`](#acl_down_policy) field instead.**Either
  "allow", "deny", "extend-cache" or "async-cache"; "extend-cache" is the default. In the case that the
  policy for a token cannot be read from the [`primary_datacenter`](#primary_datacenter) or leader
  node, the down policy is applied. In "allow" mode, all actions are permitted, "deny" restricts
  all operations, and "extend-cache" allows any cached ACLs to be used, ignoring their TTL
  values. If a non-cached ACL is used, "extend-cache" acts like "deny".
  The value "async-cache" acts the same way as "extend-cache" but performs updates
  asynchronously when ACL is present but its TTL is expired, thus, if latency is bad between
  ACL authoritative and other datacenters, latency of operations is not impacted.

* <a name="acl_agent_master_token_legacy"></a><a href="#acl_agent_master_token_legacy">`acl_agent_master_token`</a> -
  **Deprecated in Consul 1.4.0. See the [`acl.tokens.agent_master`](#acl_tokens_agent_master) field instead.**
  Used to access <a href="/api/agent.html">agent endpoints</a> that require agent read
  or write privileges, or node read privileges, even if Consul servers aren't present to validate
  any tokens. This should only be used by operators during outages, regular ACL tokens should normally
  be used by applications. This was added in Consul 0.7.2 and is only used when
  <a href="#acl_enforce_version_8">`acl_enforce_version_8`</a> is set to true.

*   <a name="acl_agent_token_legacy"></a><a href="#acl_agent_token_legacy">`acl_agent_token`</a> -
    **Deprecated in Consul 1.4.0. See the [`acl.tokens.agent`](#acl_tokens_agent) field instead.**
    Used for clients and servers to perform internal operations. If this isn't specified, then the
    <a href="#acl_token">`acl_token`</a> will be used. This was added in Consul 0.7.2.

    This token must at least have write access to the node name it will register as in order to set any
    of the node-level information in the catalog such as metadata, or the node's tagged addresses.

* <a name="acl_enforce_version_8"></a><a href="#acl_enforce_version_8">`acl_enforce_version_8`</a> -
  **Deprecated in Consul 1.4.0**
  Used for clients and servers to determine if enforcement should occur for new ACL policies being
  previewed before Consul 0.8. Added in Consul 0.7.2, this defaults to false in versions of
  Consul prior to 0.8, and defaults to true in Consul 0.8 and later. This helps ease the
  transition to the new ACL features by allowing policies to be in place before enforcement begins.


*   <a name="acl_master_token_legacy"></a><a href="#acl_master_token_legacy">`acl_master_token`</a> -
    **Deprecated in Consul 1.4.0. See the [`acl.tokens.master`](#acl_tokens_master) field instead.** Only used
    for servers in the [`primary_datacenter`](#primary_datacenter). This token will be created with management-level
    permissions if it does not exist. It allows operators to bootstrap the ACL system
    with a token ID that is well-known.

    The `acl_master_token` is only installed when a server acquires cluster leadership. If
    you would like to install or change the `acl_master_token`, set the new value for `acl_master_token`
    in the configuration for all servers. Once this is done, restart the current leader to force a
    leader election. If the `acl_master_token` is not supplied, then the servers do not create a master
    token. When you provide a value, it can be any string value. Using a UUID would ensure that it looks
    the same as the other tokens, but isn't strictly necessary.

*   <a name="acl_replication_token_legacy"></a><a href="#acl_replication_token_legacy">`acl_replication_token`</a> -
    **Deprecated in Consul 1.4.0. See the [`acl.tokens.replication`](#acl_tokens_replication) field instead.**
    Only used for servers outside the [`primary_datacenter`](#primary_datacenter) running Consul 0.7 or later.
    When provided, this will enable [ACL replication](https://learn.hashicorp.com/consul/day-2-operations/acl-replication) using this
    ACL replication using this
    token to retrieve and replicate the ACLs to the non-authoritative local datacenter. In Consul 0.9.1
    and later you can enable ACL replication using [`enable_acl_replication`](#enable_acl_replication)
    and then set the token later using the [agent token API](/api/agent.html#update-acl-tokens) on each
    server. If the `acl_replication_token` is set in the config, it will automatically set
    [`enable_acl_replication`](#enable_acl_replication) to true for backward compatibility.

    If there's a partition or other outage affecting the authoritative datacenter, and the
    [`acl_down_policy`](/docs/agent/options.html#acl_down_policy) is set to "extend-cache", tokens not
    in the cache can be resolved during the outage using the replicated set of ACLs.

* <a name="acl_token_legacy"></a><a href="#acl_token_legacy">`acl_token`</a> -
  **Deprecated in Consul 1.4.0. See the [`acl.tokens.default`](#acl_tokens_default) field instead.**
  When provided, the agent will use this
  token when making requests to the Consul servers. Clients can override this token on a per-request
  basis by providing the "?token" query parameter. When not provided, the empty token, which maps to
  the 'anonymous' ACL policy, is used.

* <a name="acl_ttl_legacy"></a><a href="#acl_ttl_legacy">`acl_ttl`</a> -
  **Deprecated in Consul 1.4.0. See the [`acl.token_ttl`](#acl_token_ttl) field instead.**Used to control Time-To-Live caching of ACLs.
  By default, this is 30 seconds. This setting has a major performance impact: reducing it will cause
  more frequent refreshes while increasing it reduces the number of refreshes. However, because the caches
  are not actively invalidated, ACL policy may be stale up to the TTL value.

* <a name="enable_acl_replication"></a><a href="#enable_acl_replication">`enable_acl_replication`</a> When
  set on a Consul server, enables ACL replication without having to set
  the replication token via [`acl_replication_token`](#acl_replication_token). Instead, enable ACL replication
  and then introduce the token using the [agent token API](/api/agent.html#update-acl-tokens) on each server.
  See [`acl_replication_token`](#acl_replication_token) for more details.

* <a name="primary_datacenter"></a><a href="#primary_datacenter">`primary_datacenter`</a> - This
  designates the datacenter which is authoritative for ACL information, intentions and is the root
  Certificate Authority for Connect. It must be provided to enable ACLs. All servers and datacenters
  must agree on the primary datacenter. Setting it on the servers is all you need for cluster-level enforcement, but for the APIs to forward properly from the clients, it must be set on them too. In
  Consul 0.8 and later, this also enables agent-level enforcement of ACLs.