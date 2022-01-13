## 1.11.2 (January 12, 2022)

FEATURES:

* ingress: allow setting TLS min version and cipher suites in ingress gateway config entries [[GH-11576](https://github.com/hashicorp/consul/issues/11576)]

IMPROVEMENTS:

* api: Return 404 when de-registering a non-existent check [[GH-11950](https://github.com/hashicorp/consul/issues/11950)]
* connect: Add support for connecting to services behind a terminating gateway when using a transparent proxy. [[GH-12049](https://github.com/hashicorp/consul/issues/12049)]
* http: when a user attempts to access the UI but can't because it's disabled, explain this and how to fix it [[GH-11820](https://github.com/hashicorp/consul/issues/11820)]
* ui: Added a notice for non-primary intention creation [[GH-11985](https://github.com/hashicorp/consul/issues/11985)]

BUG FIXES:

* Mutate `NodeService` struct properly to avoid a data race. [[GH-11940](https://github.com/hashicorp/consul/issues/11940)]
* Upgrade to raft `1.3.3` which fixes a bug where a read replica node can trigger a raft election and become a leader. [[GH-11958](https://github.com/hashicorp/consul/issues/11958)]
* cli: Display assigned node identities in output of `consul acl token list`. [[GH-11926](https://github.com/hashicorp/consul/issues/11926)]
* cli: when creating a private key, save the file with mode 0600 so that only the user has read permission. [[GH-11781](https://github.com/hashicorp/consul/issues/11781)]
* config: include all config errors in the error message, previously some could be hidden. [[GH-11918](https://github.com/hashicorp/consul/issues/11918)]
* memberlist: fixes a bug which prevented members from joining a cluster with
large amounts of churn [[GH-253](https://github.com/hashicorp/memberlist/issues/253)] [[GH-12042](https://github.com/hashicorp/consul/issues/12042)]
* snapshot: the `snapshot save` command now saves the snapshot with read permission for only the current user. [[GH-11918](https://github.com/hashicorp/consul/issues/11918)]
* ui: Differentiate between Service Meta and Node Meta when choosing search fields
in Service Instance listings [[GH-11774](https://github.com/hashicorp/consul/issues/11774)]
* ui: Ensure a login buttons appear for some error states, plus text amends [[GH-11892](https://github.com/hashicorp/consul/issues/11892)]
* ui: Ensure partition query parameter is passed through to all OIDC related API
requests [[GH-11979](https://github.com/hashicorp/consul/issues/11979)]
* ui: Fix an issue where attempting to delete a policy from the policy detail page when
attached to a token would result in the delete button disappearing and no
deletion being attempted [[GH-11868](https://github.com/hashicorp/consul/issues/11868)]
* ui: Fixes a bug where proxy service health checks would sometimes not appear
until refresh [[GH-11903](https://github.com/hashicorp/consul/issues/11903)]
* ui: Fixes a bug with URL decoding within KV area [[GH-11931](https://github.com/hashicorp/consul/issues/11931)]
* ui: Fixes a visual issue with some border colors [[GH-11959](https://github.com/hashicorp/consul/issues/11959)]
* ui: Fixes an issue saving intentions when editing per service intentions [[GH-11937](https://github.com/hashicorp/consul/issues/11937)]
* ui: Fixes an issue where once a 403 page is displayed in some circumstances its
diffcult to click back to where you where before receiving a 403 [[GH-11891](https://github.com/hashicorp/consul/issues/11891)]
* ui: Prevent disconnection notice appearing with auth change on certain pages [[GH-11905](https://github.com/hashicorp/consul/issues/11905)]
* ui: Temporarily remove KV pre-flight check for KV list permissions [[GH-11968](https://github.com/hashicorp/consul/issues/11968)]
* windows: Fixes a bug with empty log files when Consul is run as a Windows Service [[GH-11960](https://github.com/hashicorp/consul/issues/11960)]
* xds: fix a deadlock when the snapshot channel already have a snapshot to be consumed. [[GH-11924](https://github.com/hashicorp/consul/issues/11924)]

## 1.11.1 (December 15, 2021)

SECURITY:

* ci: Upgrade golang.org/x/net to address [CVE-2021-44716](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-44716) [[GH-11854](https://github.com/hashicorp/consul/issues/11854)]

FEATURES:

* Admin Partitions (Consul Enterprise only) This version adds admin partitions, a new entity defining administrative and networking boundaries within a Consul deployment. For more information refer to the
 [Admin Partition](https://www.consul.io/docs/enterprise/admin-partitions) documentation. [[GH-11855](https://github.com/hashicorp/consul/issues/11855)]
* networking: **(Enterprise Only)** Make `segment_limit` configurable, cap at 256.

## 1.11.0 (December 14, 2021)

BREAKING CHANGES:

* acl: The legacy ACL system that was deprecated in Consul 1.4.0 has been removed. Before upgrading you should verify that nothing is still using the legacy ACL system. See the [Migrate Legacy ACL Tokens Learn Guide](https://learn.hashicorp.com/tutorials/consul/access-control-token-migration) for more information. [[GH-11232](https://github.com/hashicorp/consul/issues/11232)]
* cli: `consul acl set-agent-token master` has been replaced with `consul acl set-agent-token recovery` [[GH-11669](https://github.com/hashicorp/consul/issues/11669)]

SECURITY:

* namespaces: **(Enterprise only)** Creating or editing namespaces that include default ACL policies or ACL roles now requires `acl:write` permission in the default namespace. This change fixes CVE-2021-41805.
* rpc: authorize raft requests [CVE-2021-37219](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-37219) [[GH-10925](https://github.com/hashicorp/consul/issues/10925)]

FEATURES:

* Admin Partitions (Consul Enterprise only) This version adds admin partitions, a new entity defining administrative and networking boundaries within a Consul deployment. For more information refer to the [Admin Partition](https://www.consul.io/docs/enterprise/admin-partitions) documentation.
* ca: Add a configurable TTL for Connect CA root certificates. The configuration is supported by the Vault and Consul providers. [[GH-11428](https://github.com/hashicorp/consul/issues/11428)]
* ca: Add a configurable TTL to the AWS ACM Private CA provider root certificate. [[GH-11449](https://github.com/hashicorp/consul/issues/11449)]
* health-checks: add support for h2c in http2 ping health checks [[GH-10690](https://github.com/hashicorp/consul/issues/10690)]
* ui: Add UI support to use Vault as an external source for a service [[GH-10769](https://github.com/hashicorp/consul/issues/10769)]
* ui: Adding support of Consul API Gateway as an external source. [[GH-11371](https://github.com/hashicorp/consul/issues/11371)]
* ui: Adds a copy button to each composite row in tokens list page, if Secret ID returns an actual ID [[GH-10735](https://github.com/hashicorp/consul/issues/10735)]
* ui: Adds visible Consul version information [[GH-11803](https://github.com/hashicorp/consul/issues/11803)]
* ui: Topology - New views for scenarios where no dependencies exist or ACLs are disabled [[GH-11280](https://github.com/hashicorp/consul/issues/11280)]

IMPROVEMENTS:

* acl: replication routine to report the last error message. [[GH-10612](https://github.com/hashicorp/consul/issues/10612)]
* agent: add variation of force-leave that exclusively works on the WAN [[GH-11722](https://github.com/hashicorp/consul/issues/11722)]
* api: Enable setting query options on agent health and maintenance endpoints. [[GH-10691](https://github.com/hashicorp/consul/issues/10691)]
* checks: add failures_before_warning setting for interval checks. [[GH-10969](https://github.com/hashicorp/consul/issues/10969)]
* ci: Upgrade to use Go 1.17.5 [[GH-11799](https://github.com/hashicorp/consul/issues/11799)]
* cli: Add `-cas` and `-modify-index` flags to the `consul config delete` command to support Check-And-Set (CAS) deletion of config entries [[GH-11419](https://github.com/hashicorp/consul/issues/11419)]
* config: **(Enterprise Only)** Allow specifying permission mode for audit logs. [[GH-10732](https://github.com/hashicorp/consul/issues/10732)]
* config: Support Check-And-Set (CAS) deletion of config entries [[GH-11419](https://github.com/hashicorp/consul/issues/11419)]
* config: add `dns_config.recursor_strategy` flag to control the order which DNS recursors are queried [[GH-10611](https://github.com/hashicorp/consul/issues/10611)]
* config: warn the user if client_addr is empty because client services won't be listening [[GH-11461](https://github.com/hashicorp/consul/issues/11461)]
* connect/ca: cease including the common name field in generated x509 non-CA certificates [[GH-10424](https://github.com/hashicorp/consul/issues/10424)]
* connect: Add low-level feature to allow an Ingress to retrieve TLS certificates from SDS. [[GH-10903](https://github.com/hashicorp/consul/issues/10903)]
* connect: Consul will now generate a unique virtual IP for each connect-enabled service (this will also differ across namespace/partition in Enterprise). [[GH-11724](https://github.com/hashicorp/consul/issues/11724)]
* connect: Support Vault auth methods for the Connect CA Vault provider. Currently, we support any non-deprecated auth methods
the latest version of Vault supports (v1.8.5), which include AppRole, AliCloud, AWS, Azure, Cloud Foundry, GitHub, Google Cloud,
JWT/OIDC, Kerberos, Kubernetes, LDAP, Oracle Cloud Infrastructure, Okta, Radius, TLS Certificates, and Username & Password. [[GH-11573](https://github.com/hashicorp/consul/issues/11573)]
* connect: Support manipulating HTTP headers in the mesh. [[GH-10613](https://github.com/hashicorp/consul/issues/10613)]
* connect: add Namespace configuration setting for Vault CA provider [[GH-11477](https://github.com/hashicorp/consul/issues/11477)]
* connect: ingress gateways may now enable built-in TLS for a subset of listeners. [[GH-11163](https://github.com/hashicorp/consul/issues/11163)]
* connect: service-resolver subset filters are validated for valid go-bexpr syntax on write [[GH-11293](https://github.com/hashicorp/consul/issues/11293)]
* connect: update supported envoy versions to 1.19.1, 1.18.4, 1.17.4, 1.16.5 [[GH-11115](https://github.com/hashicorp/consul/issues/11115)]
* connect: update supported envoy versions to 1.20.0, 1.19.1, 1.18.4, 1.17.4 [[GH-11277](https://github.com/hashicorp/consul/issues/11277)]
* debug: Add a new /v1/agent/metrics/stream API endpoint for streaming of metrics [[GH-10399](https://github.com/hashicorp/consul/issues/10399)]
* debug: rename cluster capture target to members, to be more consistent with the terms used by the API. [[GH-10804](https://github.com/hashicorp/consul/issues/10804)]
* dns: Added a `virtual` endpoint for querying the assigned virtual IP for a service. [[GH-11725](https://github.com/hashicorp/consul/issues/11725)]
* http: when a URL path is not found, include a message with the 404 status code to help the user understand why (e.g., HTTP API endpoint path not prefixed with /v1/) [[GH-11818](https://github.com/hashicorp/consul/issues/11818)]
* raft: Added a configuration to disable boltdb freelist syncing [[GH-11720](https://github.com/hashicorp/consul/issues/11720)]
* raft: Emit boltdb related performance metrics [[GH-11720](https://github.com/hashicorp/consul/issues/11720)]
* raft: Use bbolt instead of the legacy boltdb implementation [[GH-11720](https://github.com/hashicorp/consul/issues/11720)]
* sdk: Add support for iptable rules that allow DNS lookup redirection to Consul DNS. [[GH-11480](https://github.com/hashicorp/consul/issues/11480)]
* segments: **(Enterprise only)** ensure that the serf_lan_allowed_cidrs applies to network segments [[GH-11495](https://github.com/hashicorp/consul/issues/11495)]
* telemetry: add a new `agent.tls.cert.expiry` metric for tracking when the Agent TLS certificate expires. [[GH-10768](https://github.com/hashicorp/consul/issues/10768)]
* telemetry: add a new `mesh.active-root-ca.expiry` metric for tracking when the root certificate expires. [[GH-9924](https://github.com/hashicorp/consul/issues/9924)]
* types: add TLSVersion and TLSCipherSuite [[GH-11645](https://github.com/hashicorp/consul/issues/11645)]
* ui: Add upstream icons for upstreams and upstream instances [[GH-11556](https://github.com/hashicorp/consul/issues/11556)]
* ui: Add uri guard to prevent future URL encoding issues [[GH-11117](https://github.com/hashicorp/consul/issues/11117)]
* ui: Move the majority of our SASS variables to use native CSS custom
properties [[GH-11200](https://github.com/hashicorp/consul/issues/11200)]
* ui: Removed informational panel from the namespace selector menu when editing
namespaces [[GH-11130](https://github.com/hashicorp/consul/issues/11130)]
* ui: Update UI browser support to 'roughly ~2 years back' [[GH-11505](https://github.com/hashicorp/consul/issues/11505)]
* ui: Update global notification styling [[GH-11577](https://github.com/hashicorp/consul/issues/11577)]
* ui: added copy to clipboard button in code editor toolbars [[GH-11474](https://github.com/hashicorp/consul/issues/11474)]

DEPRECATIONS:

* api: `/v1/agent/token/agent_master` is deprecated and will be removed in a future major release - use `/v1/agent/token/agent_recovery` instead [[GH-11669](https://github.com/hashicorp/consul/issues/11669)]
* config: `acl.tokens.master` has been renamed to `acl.tokens.initial_management`, and `acl.tokens.agent_master` has been renamed to `acl.tokens.agent_recovery` - the old field names are now deprecated and will be removed in a future major release [[GH-11665](https://github.com/hashicorp/consul/issues/11665)]
* tls: With the upgrade to Go 1.17, the ordering of `tls_cipher_suites` will no longer be honored, and `tls_prefer_server_cipher_suites` is now ignored. [[GH-11364](https://github.com/hashicorp/consul/issues/11364)]

BUG FIXES:

* acl: **(Enterprise only)** fix namespace and namespace_prefix policy evaluation when both govern an authz request
* api: Fix default values used for optional fields in autopilot configuration update (POST to `/v1/operator/autopilot/configuration`) [[GH-10558](https://github.com/hashicorp/consul/issues/10558)] [[GH-10559](https://github.com/hashicorp/consul/issues/10559)]
* api: ensure new partition fields are omit empty for compatibility with older versions of consul [[GH-11585](https://github.com/hashicorp/consul/issues/11585)]
* areas: **(Enterprise Only)** Fixes a bug when using Yamux pool ( for servers version 1.7.3 and later), the entire pool was locked while connecting to a remote location, which could potentially take a long time.
* areas: **(Enterprise only)** make the gRPC server tracker network area aware [[GH-11748](https://github.com/hashicorp/consul/issues/11748)]
* ca: fixes a bug that caused non blocking leaf cert queries to return the same cached response regardless of ca rotation or leaf cert expiry [[GH-11693](https://github.com/hashicorp/consul/issues/11693)]
* ca: fixes a bug that caused the SigningKeyID to be wrong in the primary DC, when the Vault provider is used, after a CA config creates a new root. [[GH-11672](https://github.com/hashicorp/consul/issues/11672)]
* ca: fixes a bug that caused the intermediate cert used to sign leaf certs to be missing from the /connect/ca/roots API response when the Vault provider was used. [[GH-11671](https://github.com/hashicorp/consul/issues/11671)]
* check root and intermediate CA expiry before using it to sign a leaf certificate. [[GH-10500](https://github.com/hashicorp/consul/issues/10500)]
* connect/ca: ensure edits to the key type/bits for the connect builtin CA will regenerate the roots [[GH-10330](https://github.com/hashicorp/consul/issues/10330)]
* connect/ca: require new vault mount points when updating the key type/bits for the vault connect CA provider [[GH-10331](https://github.com/hashicorp/consul/issues/10331)]
* connect: fix race causing xDS generation to lock up when discovery chains are tracked for services that are no longer upstreams. [[GH-11826](https://github.com/hashicorp/consul/issues/11826)]
* dns: Fixed an issue where on DNS requests made with .alt_domain response was returned as .domain [[GH-11348](https://github.com/hashicorp/consul/issues/11348)]
* dns: return an empty answer when asked for an addr dns with type other then A and AAAA. [[GH-10401](https://github.com/hashicorp/consul/issues/10401)]
* macos: fixes building with a non-Apple LLVM (such as installed via Homebrew) [[GH-11586](https://github.com/hashicorp/consul/issues/11586)]
* namespaces: **(Enterprise only)** ensure the namespace replicator doesn't replicate deleted namespaces
* proxycfg: ensure all of the watches are canceled if they are cancelable [[GH-11824](https://github.com/hashicorp/consul/issues/11824)]
* snapshot: **(Enterprise only)** fixed a bug where the snapshot agent would ignore the `license_path` setting in config files
* ui: Ensure all types of data get reconciled with the backend data [[GH-11237](https://github.com/hashicorp/consul/issues/11237)]
* ui: Ensure dc selector correctly shows the currently selected dc [[GH-11380](https://github.com/hashicorp/consul/issues/11380)]
* ui: Ensure we check intention permissions for specific services when deciding
whether to show action buttons for per service intention actions [[GH-11409](https://github.com/hashicorp/consul/issues/11409)]
* ui: Ensure we filter tokens by policy when showing which tokens use a certain
policy whilst editing a policy [[GH-11311](https://github.com/hashicorp/consul/issues/11311)]
* ui: Ensure we show a readonly designed page for readonly intentions [[GH-11767](https://github.com/hashicorp/consul/issues/11767)]
* ui: Filter the global intentions list by the currently selected parition rather
than a wildcard [[GH-11475](https://github.com/hashicorp/consul/issues/11475)]
* ui: Fix inline-code brand styling [[GH-11578](https://github.com/hashicorp/consul/issues/11578)]
* ui: Fix visual issue with slight table header overflow [[GH-11670](https://github.com/hashicorp/consul/issues/11670)]
* ui: Fixes an issue where under some circumstances after logging we present the
data loaded previous to you logging in. [[GH-11681](https://github.com/hashicorp/consul/issues/11681)]
* ui: Gracefully recover from non-existant DC errors [[GH-11077](https://github.com/hashicorp/consul/issues/11077)]
* ui: Include `Service.Namespace` into available variables for `dashboard_url_templates` [[GH-11640](https://github.com/hashicorp/consul/issues/11640)]
* ui: Revert to depending on the backend, 'post-user-action', to report
permissions errors rather than using UI capabilities 'pre-user-action' [[GH-11520](https://github.com/hashicorp/consul/issues/11520)]
* ui: Topology - Fix up Default Allow and Permissive Intentions notices [[GH-11216](https://github.com/hashicorp/consul/issues/11216)]
* ui: code editor styling (layout consistency + wide screen support) [[GH-11474](https://github.com/hashicorp/consul/issues/11474)]
* use the MaxQueryTime instead of RPCHoldTimeout for blocking RPC queries
 [[GH-8978](https://github.com/hashicorp/consul/pull/8978)]. [[GH-10299](https://github.com/hashicorp/consul/issues/10299)]
* windows: fixes arm and arm64 builds [[GH-11586](https://github.com/hashicorp/consul/issues/11586)]

NOTES:

* Renamed the `agent_master` field to `agent_recovery` in the `acl-tokens.json` file in which tokens are persisted on-disk (when `acl.enable_token_persistence` is enabled) [[GH-11744](https://github.com/hashicorp/consul/issues/11744)]

## 1.10.4 (November 11, 2021)

SECURITY:

* agent: Use SHA256 instead of MD5 to generate persistence file names. [[GH-11491](https://github.com/hashicorp/consul/issues/11491)]
* namespaces: **(Enterprise only)** Creating or editing namespaces that include default ACL policies or ACL roles now requires `acl:write` permission in the default namespace.  This change fixes CVE-2021-41805.

IMPROVEMENTS:

* ci: Artifact builds will now only run on merges to the release branches or to `main` [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* ci: The Linux packages are now available for all supported Linux architectures including arm, arm64, 386, and amd64 [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* ci: The Linux packaging service configs and pre/post install scripts are now available under  [.release/linux] [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* connect/ca: Return an error when querying roots from uninitialized CA. [[GH-11514](https://github.com/hashicorp/consul/issues/11514)]
* telemetry: Add new metrics for the count of connect service instances and configuration entries. [[GH-11222](https://github.com/hashicorp/consul/issues/11222)]

BUG FIXES:

* acl: fixes the fallback behaviour of down_policy with setting extend-cache/async-cache when the token is not cached. [[GH-11136](https://github.com/hashicorp/consul/issues/11136)]
* api: fixed backwards compatibility issue with AgentService SocketPath field. [[GH-11318](https://github.com/hashicorp/consul/issues/11318)]
* connect/ca: Allow secondary initialization to resume after being deferred due to unreachable or incompatible primary DC servers. [[GH-11514](https://github.com/hashicorp/consul/issues/11514)]
* connect: fix issue with attempting to generate an invalid upstream cluster from UpstreamConfig.Defaults. [[GH-11245](https://github.com/hashicorp/consul/issues/11245)]
* raft: do not trigger an election if not part of the servers list. [[GH-11375](https://github.com/hashicorp/consul/issues/11375)]
* rpc: only attempt to authorize the DNSName in the client cert when verify_incoming_rpc=true [[GH-11255](https://github.com/hashicorp/consul/issues/11255)]
* server: **(Enterprise only)** Ensure that servers leave network segments when leaving other gossip pools
* snapshot: **(Enterprise only)** snapshot agent no longer attempts to refresh its license from the server when a local license is provided (i.e. via config or an environment variable)
* telemetry: Consul Clients no longer emit Autopilot metrics. [[GH-11241](https://github.com/hashicorp/consul/issues/11241)]
* telemetry: fixes a bug with Prometheus consul_autopilot_failure_tolerance metric where 0 is reported instead of NaN on follower servers. [[GH-11399](https://github.com/hashicorp/consul/issues/11399)]
* telemetry: fixes a bug with Prometheus consul_autopilot_healthy metric where 0 is reported instead of NaN on servers. [[GH-11231](https://github.com/hashicorp/consul/issues/11231)]
* ui: **(Enterprise only)** When no namespace is selected, make sure to default to the tokens default namespace when requesting permissions [[GH-11472](https://github.com/hashicorp/consul/issues/11472)]
* ui: Ensure we check intention permissions for specific services when deciding
whether to show action buttons for per service intention actions [[GH-11270](https://github.com/hashicorp/consul/issues/11270)]
* ui: Fixed styling of Role remove dialog on the Token edit page [[GH-11298](https://github.com/hashicorp/consul/issues/11298)]
* xds: fixes a bug where replacing a mesh gateway node used for WAN federation (with another that has a different IP) could leave gateways in the other DC unable to re-establish the connection [[GH-11522](https://github.com/hashicorp/consul/issues/11522)]

BUG FIXES:

* Fixing SOA record to return proper domain when alt domain in use. [[GH-10431]](https://github.com/hashicorp/consul/pull/10431)

## 1.10.3 (September 27, 2021)

FEATURES:

* sso/oidc: **(Enterprise only)** Add support for providing acr_values in OIDC auth flow [[GH-11026](https://github.com/hashicorp/consul/issues/11026)]

IMPROVEMENTS:

* audit-logging: **(Enterprise Only)** Audit logs will now include select HTTP headers in each logs payload. Those headers are: `Forwarded`, `Via`, `X-Forwarded-For`, `X-Forwarded-Host` and `X-Forwarded-Proto`. [[GH-11107](https://github.com/hashicorp/consul/issues/11107)]
* connect: update supported envoy versions to 1.18.4, 1.17.4, 1.16.5 [[GH-10961](https://github.com/hashicorp/consul/issues/10961)]
* telemetry: Add new metrics for the count of KV entries in the Consul store. [[GH-11090](https://github.com/hashicorp/consul/issues/11090)]

BUG FIXES:

* api: Revert early out errors from license APIs to allow v1.10+ clients to
manage licenses on older servers [[GH-10952](https://github.com/hashicorp/consul/issues/10952)]
* connect: Fix upstream listener escape hatch for prepared queries [[GH-11109](https://github.com/hashicorp/consul/issues/11109)]
* grpc: strip local ACL tokens from RPCs during forwarding if crossing datacenters [[GH-11099](https://github.com/hashicorp/consul/issues/11099)]
* tls: consider presented intermediates during server connection tls handshake. [[GH-10964](https://github.com/hashicorp/consul/issues/10964)]
* ui: **(Enterprise Only)** Fix saving intentions with namespaced source/destination [[GH-11095](https://github.com/hashicorp/consul/issues/11095)]
* ui: Don't show a CRD warning for read-only intentions [[GH-11149](https://github.com/hashicorp/consul/issues/11149)]
* ui: Ensure routing-config page blocking queries are cleaned up correctly [[GH-10915](https://github.com/hashicorp/consul/issues/10915)]
* ui: Ignore reported permissions for KV area meaning the KV is always enabled
for both read/write access if the HTTP API allows. [[GH-10916](https://github.com/hashicorp/consul/issues/10916)]
* ui: hide create button for policies/roles/namespace if users token has no write permissions to those areas [[GH-10914](https://github.com/hashicorp/consul/issues/10914)]
* xds: ensure the active streams counters are 64 bit aligned on 32 bit systems [[GH-11085](https://github.com/hashicorp/consul/issues/11085)]
* xds: fixed a bug where Envoy sidecars could enter a state where they failed to receive xds updates from Consul [[GH-10987](https://github.com/hashicorp/consul/issues/10987)]

## 1.10.2 (August 27, 2021)

KNOWN ISSUES:

* tls: The fix for [CVE-2021-37219](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-37219) introduced an issue that could prevent TLS certificate validation when intermediate CA certificates used to sign server certificates are transmitted in the TLS session but are not present in all Consul server's configured CA certificates. This has the effect of preventing Raft RPCs between the affected servers. As a work around until the next patch releases, ensure that all intermediate CA certificates are present in all Consul server configurations prior to using certificates that they have signed.

SECURITY:

* rpc: authorize raft requests [CVE-2021-37219](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-37219) [[GH-10931](https://github.com/hashicorp/consul/issues/10931)]

FEATURES:

* connect: add support for unix domain socket config via API/CLI [[GH-10758](https://github.com/hashicorp/consul/issues/10758)]
* ui: Adding support in Topology view for Routing Configurations [[GH-10872](https://github.com/hashicorp/consul/issues/10872)]
* ui: Create Routing Configurations route and page [[GH-10835](https://github.com/hashicorp/consul/issues/10835)]
* ui: Splitting up the socket mode and socket path in the Upstreams Instance List [[GH-10581](https://github.com/hashicorp/consul/issues/10581)]

IMPROVEMENTS:

* areas: **(Enterprise only)** Add 15s timeout to opening streams over pooled connections.
* areas: **(Enterprise only)** Apply backpressure to area gossip packet ingestion when more than 512 packets are waiting to be ingested.
* areas: **(Enterprise only)** Make implementation of WriteToAddress non-blocking to avoid slowing down memberlist's packetListen routine.
* checks: Add Interval and Timeout to API response. [[GH-10717](https://github.com/hashicorp/consul/issues/10717)]
* ci: make changelog-checker only validate PR number against main base [[GH-10844](https://github.com/hashicorp/consul/issues/10844)]
* ci: upgrade to use Go 1.16.7 [[GH-10856](https://github.com/hashicorp/consul/issues/10856)]
* deps: update to gogo/protobuf v1.3.2 [[GH-10813](https://github.com/hashicorp/consul/issues/10813)]
* proxycfg: log correlation IDs for the proxy configuration snapshot's blocking queries. [[GH-10689](https://github.com/hashicorp/consul/issues/10689)]

BUG FIXES:

* acl: fixes a bug that prevented the default user token from being used to authorize service registration for connect proxies. [[GH-10824](https://github.com/hashicorp/consul/issues/10824)]
* ca: fixed a bug when ca provider fail and provider state is stuck in `INITIALIZING` state. [[GH-10630](https://github.com/hashicorp/consul/issues/10630)]
* ca: report an error when setting the ca config fail because of an index check. [[GH-10657](https://github.com/hashicorp/consul/issues/10657)]
* cli: Ensure the metrics endpoint is accessible when Envoy is configured to use
a non-default admin bind address. [[GH-10757](https://github.com/hashicorp/consul/issues/10757)]
* cli: Fix a bug which prevented initializing a watch when using a namespaced
token. [[GH-10795](https://github.com/hashicorp/consul/issues/10795)]
* cli: Fix broken KV import command on Windows. [[GH-10820](https://github.com/hashicorp/consul/issues/10820)]
* connect: ensure SAN validation for prepared queries validates against all possible prepared query targets [[GH-10873](https://github.com/hashicorp/consul/issues/10873)]
* connect: fix crash that would result from multiple instances of a service resolving service config on a single agent. [[GH-10647](https://github.com/hashicorp/consul/issues/10647)]
* connect: proxy upstreams inherit namespace from service if none are defined. [[GH-10688](https://github.com/hashicorp/consul/issues/10688)]
* dns: fixes a bug with edns truncation where the response could exceed the size limit in some cases. [[GH-10009](https://github.com/hashicorp/consul/issues/10009)]
* grpc: ensure that streaming gRPC requests work over mesh gateway based wan federation [[GH-10838](https://github.com/hashicorp/consul/issues/10838)]
* http: log cancelled requests as such at the INFO level, instead of logging them as errored requests. [[GH-10707](https://github.com/hashicorp/consul/issues/10707)]
* streaming: set the default wait timeout for health queries [[GH-10707](https://github.com/hashicorp/consul/issues/10707)]
* txn: fixes Txn.Apply to properly authorize service registrations. [[GH-10798](https://github.com/hashicorp/consul/issues/10798)]
* ui: Disabling policy form fields from users with 'read' permissions [[GH-10902](https://github.com/hashicorp/consul/issues/10902)]
* ui: Fix Health Checks in K/V form Lock Sessions Info section [[GH-10767](https://github.com/hashicorp/consul/issues/10767)]
* ui: Fix dropdown option duplication in the new intentions form [[GH-10706](https://github.com/hashicorp/consul/issues/10706)]
* ui: Hide all metrics for ingress gateway services [[GH-10858](https://github.com/hashicorp/consul/issues/10858)]
* ui: Properly encode non-URL safe characters in OIDC responses [[GH-10901](https://github.com/hashicorp/consul/issues/10901)]
* ui: fixes a bug with some service failovers not showing the routing tab visualization [[GH-10913](https://github.com/hashicorp/consul/issues/10913)]

## 1.10.1 (July 15, 2021)

KNOWN ISSUES:

* The change to enable streaming by default uncovered an incompatibility between streaming and WAN federation over mesh gateways causing traffic to fall back to attempting a direct WAN connection rather than transiting through the gateways. We currently suggest explicitly setting [`use_streaming_backend=false`](https://www.consul.io/docs/agent/options#use_streaming_backend) if using WAN federation over mesh gateways when upgrading to 1.10.1 and are working to address this issue in a future patch release.

SECURITY:

* xds: ensure envoy verifies the subject alternative name for upstreams [CVE-2021-32574](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-32574) [[GH-10621](https://github.com/hashicorp/consul/issues/10621)]
* xds: ensure single L7 deny intention with default deny policy does not result in allow action [CVE-2021-36213](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-36213) [[GH-10619](https://github.com/hashicorp/consul/issues/10619)]

FEATURES:

* cli: allow running `redirect-traffic` command in a provided Linux namespace. [[GH-10564](https://github.com/hashicorp/consul/issues/10564)]
* sdk: allow applying `iptables` rules in a provided Linux namespace. [[GH-10564](https://github.com/hashicorp/consul/issues/10564)]

IMPROVEMENTS:

* acl: Return secret ID when listing tokens if accessor has `acl:write` [[GH-10546](https://github.com/hashicorp/consul/issues/10546)]
* structs: prevent service-defaults upstream configs from using wildcard names or namespaces [[GH-10475](https://github.com/hashicorp/consul/issues/10475)]
* ui: Move all CSS icons to use standard CSS custom properties rather than SASS variables [[GH-10298](https://github.com/hashicorp/consul/issues/10298)]

DEPRECATIONS:

* connect/ca: remove the `RotationPeriod` field from the Consul CA provider, it was not used for anything. [[GH-10552](https://github.com/hashicorp/consul/issues/10552)]

BUG FIXES:

* agent: fix a panic on 32-bit platforms caused by misaligned struct fields used with sync/atomic. [[GH-10515](https://github.com/hashicorp/consul/issues/10515)]
* ca: Fixed a bug that returned a malformed certificate chain when the certificate did not having a trailing newline. [[GH-10411](https://github.com/hashicorp/consul/issues/10411)]
* checks: fixes the default ServerName used with TLS health checks. [[GH-10490](https://github.com/hashicorp/consul/issues/10490)]
* connect/proxy: fixes logic bug preventing builtin/native proxy from starting upstream listeners [[GH-10486](https://github.com/hashicorp/consul/issues/10486)]
* streaming: fix a bug that was preventing streaming from being enabled. [[GH-10514](https://github.com/hashicorp/consul/issues/10514)]
* ui: **(Enterprise only)** Ensure permissions are checked based on the actively selected namespace [[GH-10608](https://github.com/hashicorp/consul/issues/10608)]
* ui: Ensure in-folder KVs are created in the correct folder [[GH-10569](https://github.com/hashicorp/consul/issues/10569)]
* ui: Fix KV editor syntax highlighting [[GH-10605](https://github.com/hashicorp/consul/issues/10605)]
* ui: Send service name down to Stats to properly call endpoint for Upstreams and Downstreams metrics [[GH-10535](https://github.com/hashicorp/consul/issues/10535)]
* ui: Show ACLs disabled page at Tokens page instead of 403 error when ACLs are disabled [[GH-10604](https://github.com/hashicorp/consul/issues/10604)]
* ui: Use the token's namespace instead of the default namespace when not
specifying a namespace in the URL [[GH-10503](https://github.com/hashicorp/consul/issues/10503)]

## 1.10.0 (June 22, 2021)

BREAKING CHANGES:

* connect: Disallow wildcard as name for service-defaults. [[GH-10069](https://github.com/hashicorp/consul/issues/10069)]
* connect: avoid encoding listener info in ingress and terminating gateway listener stats names. [[GH-10404](https://github.com/hashicorp/consul/issues/10404)]
* licensing: **(Enterprise Only)** Consul Enterprise 1.10 has removed API driven licensing of servers in favor of license loading via configuration. The `PUT` and `DELETE` methods on the `/v1/operator/license` endpoint will now return 405s, the `consul license put` and `consul license reset` CLI commands have been removed and the `LicensePut` and `LicenseReset` methods in the API client have been altered to always return an error. [[GH-10211](https://github.com/hashicorp/consul/issues/10211)]
* licensing: **(Enterprise Only)** Consul Enterprise client agents now require a valid non-anonymous ACL token for retrieving their license from the servers. Additionally client agents rely on the value of the `start_join` and `retry_join` configurations for determining the servers to query for the license. Therefore one must be set to use license auto-retrieval. [[GH-10248](https://github.com/hashicorp/consul/issues/10248)]
* licensing: **(Enterprise Only)** Consul Enterprise has removed support for temporary licensing. All server agents must have a valid license at startup and client agents must have a license at startup or be able to retrieve one from the servers. [[GH-10248](https://github.com/hashicorp/consul/issues/10248)]

FEATURES:

* checks: add H2 ping health checks. [[GH-8431](https://github.com/hashicorp/consul/issues/8431)]
* cli: Add additional flags to the `consul connect redirect-traffic` command to allow excluding inbound and outbound ports,
outbound CIDRs, and additional user IDs from traffic redirection. [[GH-10134](https://github.com/hashicorp/consul/issues/10134)]
* cli: Add new `consul connect redirect-traffic` command for applying traffic redirection rules when Transparent Proxy is enabled. [[GH-9910](https://github.com/hashicorp/consul/issues/9910)]
* cli: Add prefix option to kv import command [[GH-9792](https://github.com/hashicorp/consul/issues/9792)]
* cli: Automatically exclude ports from `envoy_prometheus_bind_addr`, `envoy_stats_bind_addr`, and `ListenerPort` from `Expose` config
from inbound traffic redirection rules if `proxy-id` flag is provided to the `consul connect redirect-traffic` command. [[GH-10134](https://github.com/hashicorp/consul/issues/10134)]
* cli: snapshot inspect command provides KV usage breakdown [[GH-9098](https://github.com/hashicorp/consul/issues/9098)]
* cli: snapshot inspect command supports JSON output [[GH-9006](https://github.com/hashicorp/consul/issues/9006)]
* connect: Add local_request_timeout_ms to allow configuring the Envoy request timeout on local_app [[GH-9554](https://github.com/hashicorp/consul/issues/9554)]
* connect: add support for unix domain sockets addresses for service upstreams and downstreams [[GH-9981](https://github.com/hashicorp/consul/issues/9981)]
* connect: add toggle to globally disable wildcard outbound network access when transparent proxy is enabled [[GH-9973](https://github.com/hashicorp/consul/issues/9973)]
* connect: generate upstream service labels for terminating gateway listener stats. [[GH-10404](https://github.com/hashicorp/consul/issues/10404)]
* sdk: Add new `iptables` package for applying traffic redirection rules with iptables. [[GH-9910](https://github.com/hashicorp/consul/issues/9910)]
* sdk: Allow excluding inbound and outbound ports, outbound CIDRs, and additional user IDs from traffic redirection in the `iptables` package. [[GH-10134](https://github.com/hashicorp/consul/issues/10134)]
* ui: Add Unix Domain Socket support [[GH-10287](https://github.com/hashicorp/consul/issues/10287)]
* ui: Create a collapsible notices component for the Topology tab [[GH-10270](https://github.com/hashicorp/consul/issues/10270)]
* ui: Read-only ACL Auth Methods view [[GH-9617](https://github.com/hashicorp/consul/issues/9617)]
* ui: Transparent Proxy - Service mesh visualization updates [[GH-10002](https://github.com/hashicorp/consul/issues/10002)]
* xds: emit a labeled gauge of connected xDS streams by version [[GH-10243](https://github.com/hashicorp/consul/issues/10243)]
* xds: exclusively support the Incremental xDS protocol when using xDS v3 [[GH-9855](https://github.com/hashicorp/consul/issues/9855)]

IMPROVEMENTS:

* acl: extend the auth-methods list endpoint to include MaxTokenTTL and TokenLocality fields. [[GH-9741](https://github.com/hashicorp/consul/issues/9741)]
* acl: use the presence of a management policy in the state store as a sign that we already migrated to v2 acls [[GH-9505](https://github.com/hashicorp/consul/issues/9505)]
* agent: Save exposed Envoy ports to the agent's state when `Expose.Checks` is true in proxy's configuration. [[GH-10173](https://github.com/hashicorp/consul/issues/10173)]
* api: Add `ExposedPort` to the health check API resource. [[GH-10173](https://github.com/hashicorp/consul/issues/10173)]
* api: Enable setting query options on agent endpoints. [[GH-9903](https://github.com/hashicorp/consul/issues/9903)]
* api: The `Content-Type` header is now always set when a body is present in a request. [[GH-10204](https://github.com/hashicorp/consul/issues/10204)]
* cli: snapshot inspect command can now inspect raw snapshots from a server's data
dir. [[GH-10089](https://github.com/hashicorp/consul/issues/10089)]
* cli: the `consul connect envoy --envoy_statsd_url` flag will now resolve the `$HOST_IP` environment variable, as part of a full url. [[GH-8564](https://github.com/hashicorp/consul/issues/8564)]
* command: Exclude exposed Envoy ports from traffic redirection when providing `-proxy-id` and `Expose.Checks` is set. [[GH-10173](https://github.com/hashicorp/consul/issues/10173)]
* connect: Add support for transparently proxying traffic through Envoy. [experimental] [[GH-9894](https://github.com/hashicorp/consul/issues/9894)]
* connect: Allow per-upstream configuration to be set in service-defaults. [experimental] [[GH-9872](https://github.com/hashicorp/consul/issues/9872)]
* connect: Ensures passthrough tproxy cluster is created even when mesh config doesn't exist. [[GH-10301](https://github.com/hashicorp/consul/issues/10301)]
* connect: Support dialing individual service IP addresses through transparent proxies. [[GH-10329](https://github.com/hashicorp/consul/issues/10329)]
* connect: The builtin connect proxy no longer advertises support for h2 via ALPN. [[GH-4466](https://github.com/hashicorp/consul/issues/4466)]. [[GH-9920](https://github.com/hashicorp/consul/issues/9920)]
* connect: Update the service mesh visualization to account for transparent proxies. [[GH-10016](https://github.com/hashicorp/consul/issues/10016)]
* connect: adds new flags `prometheus-backend-port` and `prometheus-scrape-port` to `consul connect envoy` to support envoy_prometheus_bind_addr pointing to the merged metrics port when using Consul Connect on K8s. [[GH-9768](https://github.com/hashicorp/consul/issues/9768)]
* connect: allow exposing duplicate HTTP paths through a proxy instance. [[GH-10394](https://github.com/hashicorp/consul/issues/10394)]
* connect: rename cluster config entry to mesh. [[GH-10127](https://github.com/hashicorp/consul/issues/10127)]
* connect: restrict transparent proxy mode to only match on the tagged virtual IP address. [[GH-10162](https://github.com/hashicorp/consul/issues/10162)]
* connect: update supported envoy versions to 1.18.2, 1.17.2, 1.16.3, 1.15.4 [[GH-10101](https://github.com/hashicorp/consul/issues/10101)]
* connect: update supported envoy versions to 1.18.3, 1.17.3, 1.16.4, and 1.15.5 [[GH-10231](https://github.com/hashicorp/consul/issues/10231)]
* debug: capture a single stream of logs, and single pprof profile and trace for the whole duration [[GH-10279](https://github.com/hashicorp/consul/issues/10279)]
* grpc: move gRPC INFO logs to be emitted as TRACE logs from Consul [[GH-10395](https://github.com/hashicorp/consul/issues/10395)]
* licensing: **(Enterprise Only)** Consul Enterprise has gained the `consul license inspect` CLI command for inspecting a license without applying it..
* licensing: **(Enterprise Only)** Consul Enterprise has gained the ability to autoload a license via configuration. This can be specified with the `license_path` configuration, the `CONSUL_LICENSE` environment variable or the `CONSUL_LICENSE_PATH` environment variable [[GH-10210](https://github.com/hashicorp/consul/issues/10210)]
* licensing: **(Enterprise Only)** Consul Enterprise has gained the ability update its license via a configuration reload. The same environment variables and configurations will be used to determine the new license. [[GH-10267](https://github.com/hashicorp/consul/issues/10267)]
* monitoring: optimize the monitoring endpoint to avoid losing logs when under high load. [[GH-10368](https://github.com/hashicorp/consul/issues/10368)]
* raft: allow reloading of raft trailing logs and snapshot timing to allow recovery from some [replication failure modes](https://github.com/hashicorp/consul/issues/9609).
telemetry: add metrics and documentation for [monitoring for replication issues](https://consul.io/docs/agent/telemetry#raft-replication-capacity-issues). [[GH-10129](https://github.com/hashicorp/consul/issues/10129)]
* streaming: change `use_streaming_backend` to default to true so that streaming is used by default when it is supported. [[GH-10149](https://github.com/hashicorp/consul/issues/10149)]
* ui: Add 'optional route segments' and move namespaces to use them [[GH-10212](https://github.com/hashicorp/consul/issues/10212)]
* ui: Adding a notice about how TransparentProxy mode affects the Upstreams list at the top of tab view [[GH-10136](https://github.com/hashicorp/consul/issues/10136)]
* ui: Improve loader centering with new side navigation [[GH-10181](https://github.com/hashicorp/consul/issues/10181)]
* ui: Move to a sidebar based main navigation [[GH-9553](https://github.com/hashicorp/consul/issues/9553)]
* ui: Show a message to explain that health checks may be out of date if the serf health check is in a critical state [[GH-10194](https://github.com/hashicorp/consul/issues/10194)]
* ui: Updating the wording for the banner and the popover for a service with an upstream that is not explicitly defined. [[GH-10133](https://github.com/hashicorp/consul/issues/10133)]
* ui: Use older (~2016) native ES6 features to reduce transpilation and UI JS payload [[GH-9729](https://github.com/hashicorp/consul/issues/9729)]
* ui: add permanently visible indicator when ACLs are disabled [[GH-9864](https://github.com/hashicorp/consul/issues/9864)]
* ui: improve accessibility of modal dialogs [[GH-9819](https://github.com/hashicorp/consul/issues/9819)]
* ui: restrict the viewing/editing of certain UI elements based on the users ACL token [[GH-9687](https://github.com/hashicorp/consul/issues/9687)]
* ui: updates the ui with the new consul brand assets [[GH-10081](https://github.com/hashicorp/consul/issues/10081)]
* xds: add support for envoy 1.17.0 [[GH-9658](https://github.com/hashicorp/consul/issues/9658)]
* xds: default to speaking xDS v3, but allow for v2 to be spoken upon request [[GH-9658](https://github.com/hashicorp/consul/issues/9658)]
* xds: ensure that all envoyproxy/go-control-plane protobuf symbols are linked into the final binary [[GH-10131](https://github.com/hashicorp/consul/issues/10131)]
* xds: remove deprecated usages of xDS and drop support for envoy 1.13.x [[GH-9602](https://github.com/hashicorp/consul/issues/9602)]

BUG FIXES:

* checks: add TLSServerName field to allow setting the TLS server name for HTTPS health checks. [[GH-9475](https://github.com/hashicorp/consul/issues/9475)]
* config: Fixed a bug where `rpc_max_conns_per_client` could not be changed by reloading the
config. [[GH-8696](https://github.com/hashicorp/consul/issues/8696)]
* connect: Fix bug that prevented transparent proxies from working when mesh config restricted routing to catalog destinations. [[GH-10365](https://github.com/hashicorp/consul/issues/10365)]
* memberlist: fixes a couple bugs which allowed malformed input to cause a crash in a Consul
client or server. [[GH-10161](https://github.com/hashicorp/consul/issues/10161)]
* monitor: fix monitor to produce json format logs when requested [[GH-10358](https://github.com/hashicorp/consul/issues/10358)]
* proxycfg: Ensure that endpoints for explicit upstreams in other datacenters are watched in transparent mode. [[GH-10391](https://github.com/hashicorp/consul/issues/10391)]
* proxycfg: avoid panic when transparent proxy upstream is added and then removed. [[GH-10423](https://github.com/hashicorp/consul/issues/10423)]
* streaming: fixes a bug that would cause context cancellation errors when a cache entry expired while requests were active. [[GH-10112](https://github.com/hashicorp/consul/issues/10112)]
* streaming: lookup in health properly handle case-sensitivity and perform filtering based on tags and node-meta [[GH-9703](https://github.com/hashicorp/consul/issues/9703)]
* telemetry: fixes a bug with Prometheus metrics where Gauges and Summaries were incorrectly
being expired. [[GH-10161](https://github.com/hashicorp/consul/issues/10161)]
* ui: Adding conditional to prevent Service Mesh from breaking when there are no Upstreams [[GH-10122](https://github.com/hashicorp/consul/issues/10122)]
* ui: Fix text searching through upstream instances. [[GH-10151](https://github.com/hashicorp/consul/issues/10151)]
* ui: Update conditional for topology empty state [[GH-10124](https://github.com/hashicorp/consul/issues/10124)]
* ui: ensure proxy instance API requests perform blocking queries correctly [[GH-10039](https://github.com/hashicorp/consul/issues/10039)]
* xds: (beta-only) ensure that dependent xDS resources are reconfigured during primary type warming [[GH-10381](https://github.com/hashicorp/consul/issues/10381)]

NOTES:

* legal: **(Enterprise only)** Enterprise binary downloads will now include a copy of the EULA and Terms of Evaluation in the zip archive

## 1.9.11 (November 11, 2021)

SECURITY:

* agent: Use SHA256 instead of MD5 to generate persistence file names. [[GH-11491](https://github.com/hashicorp/consul/issues/11491)]
* namespaces: **(Enterprise only)** Creating or editing namespaces that include default ACL policies or ACL roles now requires `acl:write` permission in the default namespace.  This change fixes CVE-2021-41805.

IMPROVEMENTS:

* ci: Artifact builds will now only run on merges to the release branches or to `main` [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* ci: The Linux packages are now available for all supported Linux architectures including arm, arm64, 386, and amd64 [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* ci: The Linux packaging service configs and pre/post install scripts are now available under  [.release/linux] [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* telemetry: Add new metrics for the count of connect service instances and configuration entries. [[GH-11222](https://github.com/hashicorp/consul/issues/11222)]

BUG FIXES:

* acl: fixes the fallback behaviour of down_policy with setting extend-cache/async-cache when the token is not cached. [[GH-11136](https://github.com/hashicorp/consul/issues/11136)]
* rpc: only attempt to authorize the DNSName in the client cert when verify_incoming_rpc=true [[GH-11255](https://github.com/hashicorp/consul/issues/11255)]
* server: **(Enterprise only)** Ensure that servers leave network segments when leaving other gossip pools
* ui: Fixed styling of Role remove dialog on the Token edit page [[GH-11298](https://github.com/hashicorp/consul/issues/11298)]
* xds: fixes a bug where replacing a mesh gateway node used for WAN federation (with another that has a different IP) could leave gateways in the other DC unable to re-establish the connection [[GH-11522](https://github.com/hashicorp/consul/issues/11522)]

## 1.9.10 (September 27, 2021)

FEATURES:

* sso/oidc: **(Enterprise only)** Add support for providing acr_values in OIDC auth flow [[GH-11026](https://github.com/hashicorp/consul/issues/11026)]

IMPROVEMENTS:

* audit-logging: **(Enterprise Only)** Audit logs will now include select HTTP headers in each logs payload. Those headers are: `Forwarded`, `Via`, `X-Forwarded-For`, `X-Forwarded-Host` and `X-Forwarded-Proto`. [[GH-11107](https://github.com/hashicorp/consul/issues/11107)]
* connect: update supported envoy versions to 1.16.5 [[GH-10961](https://github.com/hashicorp/consul/issues/10961)]
* telemetry: Add new metrics for the count of KV entries in the Consul store. [[GH-11090](https://github.com/hashicorp/consul/issues/11090)]

BUG FIXES:

* tls: consider presented intermediates during server connection tls handshake. [[GH-10964](https://github.com/hashicorp/consul/issues/10964)]
* ui: **(Enterprise Only)** Fix saving intentions with namespaced source/destination [[GH-11095](https://github.com/hashicorp/consul/issues/11095)]

## 1.9.9 (August 27, 2021)

KNOWN ISSUES:

* tls: The fix for [CVE-2021-37219](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-37219) introduced an issue that could prevent TLS certificate validation when intermediate CA certificates used to sign server certificates are transmitted in the TLS session but are not present in all Consul server's configured CA certificates. This has the effect of preventing Raft RPCs between the affected servers. As a work around until the next patch releases, ensure that all intermediate CA certificates are present in all Consul server configurations prior to using certificates that they have signed.

SECURITY:

* rpc: authorize raft requests [CVE-2021-37219](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-37219) [[GH-10932](https://github.com/hashicorp/consul/issues/10932)]

IMPROVEMENTS:

* areas: **(Enterprise only)** Add 15s timeout to opening streams over pooled connections.
* areas: **(Enterprise only)** Apply backpressure to area gossip packet ingestion when more than 512 packets are waiting to be ingested.
* areas: **(Enterprise only)** Make implementation of WriteToAddress non-blocking to avoid slowing down memberlist's packetListen routine.
* deps: update to gogo/protobuf v1.3.2 [[GH-10813](https://github.com/hashicorp/consul/issues/10813)]

BUG FIXES:

* acl: fixes a bug that prevented the default user token from being used to authorize service registration for connect proxies. [[GH-10824](https://github.com/hashicorp/consul/issues/10824)]
* ca: fixed a bug when ca provider fail and provider state is stuck in `INITIALIZING` state. [[GH-10630](https://github.com/hashicorp/consul/issues/10630)]
* ca: report an error when setting the ca config fail because of an index check. [[GH-10657](https://github.com/hashicorp/consul/issues/10657)]
* cli: Ensure the metrics endpoint is accessible when Envoy is configured to use
a non-default admin bind address. [[GH-10757](https://github.com/hashicorp/consul/issues/10757)]
* cli: Fix a bug which prevented initializing a watch when using a namespaced
token. [[GH-10795](https://github.com/hashicorp/consul/issues/10795)]
* connect: proxy upstreams inherit namespace from service if none are defined. [[GH-10688](https://github.com/hashicorp/consul/issues/10688)]
* dns: fixes a bug with edns truncation where the response could exceed the size limit in some cases. [[GH-10009](https://github.com/hashicorp/consul/issues/10009)]
* txn: fixes Txn.Apply to properly authorize service registrations. [[GH-10798](https://github.com/hashicorp/consul/issues/10798)]
* ui: Fix dropdown option duplication in the new intentions form [[GH-10706](https://github.com/hashicorp/consul/issues/10706)]
* ui: Hide all metrics for ingress gateway services [[GH-10858](https://github.com/hashicorp/consul/issues/10858)]
* ui: Properly encode non-URL safe characters in OIDC responses [[GH-10901](https://github.com/hashicorp/consul/issues/10901)]
* ui: fixes a bug with some service failovers not showing the routing tab visualization [[GH-10913](https://github.com/hashicorp/consul/issues/10913)]

## 1.9.8 (July 15, 2021)

SECURITY:

* xds: ensure envoy verifies the subject alternative name for upstreams [CVE-2021-32574](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-32574) [[GH-10621](https://github.com/hashicorp/consul/issues/10621)]
* xds: ensure single L7 deny intention with default deny policy does not result in allow action [CVE-2021-36213](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-36213) [[GH-10619](https://github.com/hashicorp/consul/issues/10619)]

BUG FIXES:

* ca: Fixed a bug that returned a malformed certificate chain when the certificate did not having a trailing newline. [[GH-10411](https://github.com/hashicorp/consul/issues/10411)]
* ui: Send service name down to Stats to properly call endpoint for Upstreams and Downstreams metrics [[GH-10535](https://github.com/hashicorp/consul/issues/10535)]

## 1.9.7 (June 21, 2021)

IMPROVEMENTS:

* debug: capture a single stream of logs, and single pprof profile and trace for the whole duration [[GH-10279](https://github.com/hashicorp/consul/issues/10279)]
* licensing: **(Enterprise Only)** In order to have forward compatibility with Consul Enterprise v1.10, the ability to parse licenses from the configuration or environment has been added. This can be specified with the `license_path` configuration, the `CONSUL_LICENSE` environment variable or the `CONSUL_LICENSE_PATH` environment variable. On server agents this configuration will be ignored. Client agents and the snapshot agent will use the configured license instead of automatically retrieving one. [[GH-10441](https://github.com/hashicorp/consul/issues/10441)]
* monitoring: optimize the monitoring endpoint to avoid losing logs when under high load. [[GH-10368](https://github.com/hashicorp/consul/issues/10368)]

BUG FIXES:

* license: **(Enterprise only)** Fixed an issue that would cause client agents on versions before 1.10 to not be able to retrieve the license from a 1.10+ server. [[GH-10432](https://github.com/hashicorp/consul/issues/10432)]
* monitor: fix monitor to produce json format logs when requested [[GH-10358](https://github.com/hashicorp/consul/issues/10358)]

## 1.9.6 (June 04, 2021)

IMPROVEMENTS:

* acl: Give more descriptive error if auth method not found. [[GH-10163](https://github.com/hashicorp/consul/issues/10163)]
* areas: **(Enterprise only)** Use server agent's gossip_wan config when setting memberlist configuration for network areas. Previously they used memberlists WAN defaults.
* cli: added a `-force-without-cross-signing` flag to the `ca set-config` command.
connect/ca: The ForceWithoutCrossSigning field will now work as expected for CA providers that support cross signing. [[GH-9672](https://github.com/hashicorp/consul/issues/9672)]
* connect: update supported envoy versions to 1.16.3, 1.15.4, 1.14.7, 1.13.7 [[GH-10105](https://github.com/hashicorp/consul/issues/10105)]
* connect: update supported envoy versions to 1.16.4, 1.15.5, 1.14.6, and 1.13.7 [[GH-10232](https://github.com/hashicorp/consul/issues/10232)]
* telemetry: Add new metrics for status of secondary datacenter replication. [[GH-10073](https://github.com/hashicorp/consul/issues/10073)]
* telemetry: The usage data in the `metrics` API now includes cluster member counts, reporting clients on a per segment basis. [[GH-10340](https://github.com/hashicorp/consul/issues/10340)]
* ui: Added CRD popover 'informed action' for intentions managed by CRDs [[GH-10100](https://github.com/hashicorp/consul/issues/10100)]
* ui: Added humanized formatting to lock session durations [[GH-10062](https://github.com/hashicorp/consul/issues/10062)]
* ui: Only show a partial list of intention permissions, with the option to show all [[GH-10174](https://github.com/hashicorp/consul/issues/10174)]
* ui: updates the ui with the new consul brand assets [[GH-10090](https://github.com/hashicorp/consul/issues/10090)]

BUG FIXES:

* agent: ensure we hash the non-deprecated upstream fields on ServiceConfigRequest [[GH-10240](https://github.com/hashicorp/consul/issues/10240)]
* agent: fix logging output by removing leading whitespace from every log line [[GH-10338](https://github.com/hashicorp/consul/issues/10338)]
* api: include the default value of raft settings in the output of /v1/agent/self [[GH-8812](https://github.com/hashicorp/consul/issues/8812)]
* areas: **(Enterprise only)** Revert to the 10s dial timeout used before connection pooling was introduced in 1.7.3.
* areas: **(Enterprise only)** Selectively merge gossip_wan config for network areas to avoid attempting to enable gossip encryption where it was not intended or necessary.
* autopilot: **(Enterprise only)** Fixed an issue where autopilot could cause a new leader to demote the wrong voter when redundancy zones are in use and the previous leader failed. [[GH-10306](https://github.com/hashicorp/consul/issues/10306)]
* cli: removes the need to set debug_enabled=true to collect debug data from the CLI. Now
the CLI behaves the same way as the API and accepts either an ACL token with operator:read, or
debug_enabled=true. [[GH-10273](https://github.com/hashicorp/consul/issues/10273)]
* cli: snapshot inspect command would panic on invalid input. [[GH-10091](https://github.com/hashicorp/consul/issues/10091)]
* envoy: fixes a bug where a large envoy config could cause the `consul connect envoy` command to deadlock when attempting to start envoy. [[GH-10324](https://github.com/hashicorp/consul/issues/10324)]
* http: fix a bug that caused the `X-Consul-Effective-Consistency` header to be missing on
request for service health [[GH-10189](https://github.com/hashicorp/consul/issues/10189)]
* local: agents will no longer persist the default user token along with a service or check. [[GH-10188](https://github.com/hashicorp/consul/issues/10188)]
* namespaces: **(Enterprise only)** fixes a problem where the logs would contain many warnings about namespaces not being licensed.
* server: ensure that central service config flattening properly resets the state each time [[GH-10239](https://github.com/hashicorp/consul/issues/10239)]
* ui: Add conditionals to lock sessions tab [[GH-10121](https://github.com/hashicorp/consul/issues/10121)]
* ui: De-duplicate tags in rendered tag listings [[GH-10186](https://github.com/hashicorp/consul/issues/10186)]
* ui: Don't render a DOM element for empty namespace descriptions [[GH-10157](https://github.com/hashicorp/consul/issues/10157)]
* ui: Reflect the change of Session API response shape for Checks in post 1.7 Consul [[GH-10225](https://github.com/hashicorp/consul/issues/10225)]
* ui: Removes the extra rendering of namespace in service upstream list [[GH-10152](https://github.com/hashicorp/consul/issues/10152)]

## 1.9.5 (April 15, 2021)

SECURITY:

* Add content-type headers to raw KV responses to prevent XSS attacks [CVE-2020-25864](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-25864) [[GH-10023](https://github.com/hashicorp/consul/issues/10023)]
* audit-logging: Parse endpoint URL to prevent requests from bypassing the audit log [CVE-2021-28156](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-28156)

IMPROVEMENTS:

* api: `AutopilotServerHelath` now handles the 429 status code returned by the v1/operator/autopilot/health endpoint and still returned the parsed reply which will indicate server healthiness [[GH-8599](https://github.com/hashicorp/consul/issues/8599)]
* client: when a client agent is attempting to dereigster a service, anddoes not have access to the ACL token used to register a service, attempt to use the agent token instead of the default user token. If no agent token is set, fall back to the default user token. [[GH-9683](https://github.com/hashicorp/consul/issues/9683)]
* connect: Automatically rewrite the Host header for Terminating Gateway HTTP services [[GH-9042](https://github.com/hashicorp/consul/issues/9042)]
* ui: support stricter content security policies [[GH-9847](https://github.com/hashicorp/consul/issues/9847)]

BUG FIXES:

* api: ensure v1/health/ingress/:service endpoint works properly when streaming is enabled [[GH-9967](https://github.com/hashicorp/consul/issues/9967)]
* areas: Fixes a bug which would prevent newer servers in a network areas from connecting to servers running a version of Consul prior to 1.7.3.
* audit-logging: (Enterprise only) Fixed an issue that resulted in usage of the agent master token or managed service provider tokens from being resolved properly. [[GH-10013](https://github.com/hashicorp/consul/issues/10013)]
* cache: fix a bug in the client agent cache where streaming could potentially leak resources. [[GH-9978](https://github.com/hashicorp/consul/pull/9978)]. [[GH-9978](https://github.com/hashicorp/consul/issues/9978)]
* cache: fix a bug in the client agent cache where streaming would disconnect every
20 minutes and cause delivery delays. [[GH-9979](https://github.com/hashicorp/consul/pull/9979)]. [[GH-9979](https://github.com/hashicorp/consul/issues/9979)]
* command: when generating envoy bootstrap configs to stdout do not mix informational logs into the json [[GH-9980](https://github.com/hashicorp/consul/issues/9980)]
* config: correct config key from `advertise_addr_ipv6` to `advertise_addr_wan_ipv6` [[GH-9851](https://github.com/hashicorp/consul/issues/9851)]
* http: fix a bug in Consul Enterprise that would cause the UI to believe namespaces were supported, resulting in warning logs and incorrect UI behaviour. [[GH-9923](https://github.com/hashicorp/consul/issues/9923)]
* snapshot: fixes a bug that would cause snapshots to be missing all but the first ACL Auth Method. [[GH-10025](https://github.com/hashicorp/consul/issues/10025)]
* ui: Fix intention form cancel button [[GH-9901](https://github.com/hashicorp/consul/issues/9901)]

## 1.9.4 (March 04, 2021)

IMPROVEMENTS:

* connect: if the token given to the vault provider returns no data avoid a panic [[GH-9806](https://github.com/hashicorp/consul/issues/9806)]
* connect: update supported envoy point releases to 1.16.2, 1.15.3, 1.14.6, 1.13.7 [[GH-9737](https://github.com/hashicorp/consul/issues/9737)]
* xds: only try to create an ipv6 expose checks listener if ipv6 is supported by the kernel [[GH-9765](https://github.com/hashicorp/consul/issues/9765)]

BUG FIXES:

* api: Remove trailing periods from the gateway internal HTTP API endpoint [[GH-9752](https://github.com/hashicorp/consul/issues/9752)]
* cache: Prevent spamming the logs for days when a cached request encounters an "ACL not found" error. [[GH-9738](https://github.com/hashicorp/consul/issues/9738)]
* connect: connect CA Roots in the primary datacenter should use a SigningKeyID derived from their local intermediate [[GH-9428](https://github.com/hashicorp/consul/issues/9428)]
* proxycfg: avoid potential deadlock in delivering proxy snapshot to watchers. [[GH-9689](https://github.com/hashicorp/consul/issues/9689)]
* replication: Correctly log all replication warnings that should not be suppressed [[GH-9320](https://github.com/hashicorp/consul/issues/9320)]
* streaming: fixes a bug caused by caching an incorrect snapshot, that would cause clients
to error until the cache expired. [[GH-9772](https://github.com/hashicorp/consul/issues/9772)]
* ui: Exclude proxies when showing the total number of instances on a node. [[GH-9749](https://github.com/hashicorp/consul/issues/9749)]
* ui: Fixed a bug in older browsers relating to String.replaceAll and fieldset w/flexbox usage [[GH-9715](https://github.com/hashicorp/consul/issues/9715)]
* xds: deduplicate mesh gateway listeners by address in a stable way to prevent some LDS churn [[GH-9650](https://github.com/hashicorp/consul/issues/9650)]
* xds: prevent LDS flaps in mesh gateways due to unstable datacenter lists; also prevent some flaps in terminating gateways as well [[GH-9651](https://github.com/hashicorp/consul/issues/9651)]

## 1.9.3 (February 01, 2021)

FEATURES:

* ui: Add additional search/filter status pills for viewing and removing current
filters in listing views [[GH-9442](https://github.com/hashicorp/consul/issues/9442)]

IMPROVEMENTS:

* cli: Add new `-cluster-id` and `common-name` to `consul tls ca create` to support creating a CA for Consul Connect. [[GH-9585](https://github.com/hashicorp/consul/issues/9585)]
* license: **(Enterprise only)** Temporary client license duration was increased from 30m to 6h.
* server: **(Enterprise Only)** Validate source namespaces in service-intentions config entries. [[GH-9527](https://github.com/hashicorp/consul/issues/9527)]
* server: use the presence of stored federation state data as a sign that we already activated the federation state feature flag [[GH-9519](https://github.com/hashicorp/consul/issues/9519)]

BUG FIXES:

* autopilot: Fixed a bug that would cause snapshot restoration to stop autopilot on the leader. [[GH-9626](https://github.com/hashicorp/consul/issues/9626)]
* server: When wan federating via mesh gateways after initial federation default to using the local mesh gateways unless the heuristic indicates a bypass is required. [[GH-9528](https://github.com/hashicorp/consul/issues/9528)]
* server: When wan federating via mesh gateways only do heuristic primary DC bypass on the leader. [[GH-9366](https://github.com/hashicorp/consul/issues/9366)]
* ui: Fixed a bug that would cause missing or duplicate service instance healthcheck listings. [[GH-9660](https://github.com/hashicorp/consul/issues/9660)]

## 1.9.2 (January 20, 2021)

FEATURES:

* agent: add config flag `MaxHeaderBytes` to control the maximum size of the http header per client request. [[GH-9067](https://github.com/hashicorp/consul/issues/9067)]
* cli: The `consul intention` command now has a new `list` subcommand to allow the listing of configured intentions. It also supports the `-namespace=` option. [[GH-9468](https://github.com/hashicorp/consul/issues/9468)]

IMPROVEMENTS:

* server: deletions of intentions by name using the intention API is now idempotent [[GH-9278](https://github.com/hashicorp/consul/issues/9278)]
* streaming: display a warning on agent(s) when incompatible streaming parameters are used [[GH-9530](https://github.com/hashicorp/consul/issues/9530)]
* ui: Various accessibility scan test improvements [[GH-9485](https://github.com/hashicorp/consul/issues/9485)]

DEPRECATIONS:

* api: the `tag`, `node-meta`, and `passing` query parameters for various health and catalog
endpoints are now deprecated. The `filter` query parameter should be used as a replacement
for all of the deprecated fields. The deprecated query parameters will be removed in a future
version of Consul. [[GH-9262](https://github.com/hashicorp/consul/issues/9262)]

BUG FIXES:

* client: Help added in Prometheus in relases 1.9.0 does not generate warnings anymore in logs [[GH-9510](https://github.com/hashicorp/consul/issues/9510)]
* client: properly set GRPC over RPC magic numbers when encryption was not set or partially set in the cluster with streaming enabled [[GH-9512](https://github.com/hashicorp/consul/issues/9512)]
* connect: Fixed a bug in the AWS PCA Connect CA provider that could cause the intermediate PKI path to be deleted after reconfiguring the CA [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* connect: Fixed a bug in the Vault Connect CA provider that could cause the intermediate PKI path to be deleted after reconfiguring the CA [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* connect: Fixed an issue that would prevent updating the Connect CA configuration if the CA provider didn't complete initialization previously. [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* leader: Fixed a bug that could cause Connect CA initialization failures from allowing leader establishment to complete resulting in potentially infinite leader elections. [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* rpc: Prevent misleading RPC error claiming the lack of a leader when Raft is ok but there are issues with client agents gossiping with the leader. [[GH-9487](https://github.com/hashicorp/consul/issues/9487)]
* server: Fixes a server panic introduced in 1.9.0 where Connect service mesh is
being used. Node de-registration could panic if it hosted services with
multiple upstreams. [[GH-9589](https://github.com/hashicorp/consul/issues/9589)]
* state: fix computation of usage metrics to account for various places that can modify multiple services in a single transaction. [[GH-9440](https://github.com/hashicorp/consul/issues/9440)]
* ui: Display LockDelay in nanoseconds as a temporary fix to the formatting [[GH-9594](https://github.com/hashicorp/consul/issues/9594)]
* ui: Fix an issue where registering an ingress-gateway with no central config
would result in a JS error due to the API reponse returning `null` [[GH-9593](https://github.com/hashicorp/consul/issues/9593)]
* ui: Fixes an issue where clicking backwards and forwards between a service instance can result in a 404 error [[GH-9524](https://github.com/hashicorp/consul/issues/9524)]
* ui: Fixes an issue where intention description or metadata could be overwritten if saved from the topology view. [[GH-9513](https://github.com/hashicorp/consul/issues/9513)]
* ui: Fixes an issue with setting -ui-content-path flag/config [[GH-9569](https://github.com/hashicorp/consul/issues/9569)]
* ui: ensure namespace is used for node API requests [[GH-9410](https://github.com/hashicorp/consul/issues/9410)]
* ui: request intention listing with ns=* parameter to retrieve all intentions
across namespaces [[GH-9432](https://github.com/hashicorp/consul/issues/9432)]

## 1.9.1 (December 11, 2020)

FEATURES:

* ui: add copyable IDs to the Role and Policy views [[GH-9296](https://github.com/hashicorp/consul/issues/9296)]

IMPROVEMENTS:

* cli: **(Enterprise only)** A new `-read-replica` flag can now be used to enable running a server as a read only replica. Previously this was enabled with the now deprecated `-non-voting-server` flag. [[GH-9191](https://github.com/hashicorp/consul/issues/9191)]
* config: **(Enterprise only)** A new `read_replica` configuration setting can now be used to enable running a server as a read only replica. Previously this was enabled with the now deprecated `non_voting_server` setting. [[GH-9191](https://github.com/hashicorp/consul/issues/9191)]

DEPRECATIONS:

* cli: **(Enterprise only)** The `-non-voting-server` flag is deprecated in favor of the new `-read-replica` flag. The `-non-voting-server` flag is still present along side the new flag but it will be removed in a future release. [[GH-9191](https://github.com/hashicorp/consul/issues/9191)]
* config: **(Enterprise only)** The `non_voting_server` configuration setting is deprecated in favor of the new `read_replica` setting. The `non_voting_server` configuration setting is still present but will be removed in a future release. [[GH-9191](https://github.com/hashicorp/consul/issues/9191)]
* gossip: **(Enterprise only)** Read replicas now advertise themselves by setting the `read_replica` tag. The old `nonvoter` tag is still present but is deprecated and will be removed in a future release. [[GH-9191](https://github.com/hashicorp/consul/issues/9191)]
* server: **(Enterprise only)** Addition of the `nonvoter` tag to the service registration made for read replicas is deprecated in favor of the new tag name of `read_replica`. Both are present in the registration but the `nonvoter` tag will be completely removed in a future release. [[GH-9191](https://github.com/hashicorp/consul/issues/9191)]

BUG FIXES:

* agent: prevent duplicate services and check registrations from being synced to servers. [[GH-9284](https://github.com/hashicorp/consul/issues/9284)]
* connect: fixes a case when updating the CA config in a secondary datacenter to correctly trigger the creation of a new intermediate certificate [[GH-9009](https://github.com/hashicorp/consul/issues/9009)]
* connect: only unset the active root in a secondary datacenter when a new one is replacing it [[GH-9318](https://github.com/hashicorp/consul/issues/9318)]
* namespaces: **(Enterprise only)** Prevent stalling of replication in secondary datacenters due to conflicts between the namespace replicator and other replicators. [[GH-9271](https://github.com/hashicorp/consul/issues/9271)]
* streaming: ensure the order of results provided by /health/service/:serviceName is consistent with and without streaming enabled [[GH-9247](https://github.com/hashicorp/consul/issues/9247)]

## 1.9.0 (November 24, 2020)

BREAKING CHANGES:

* agent: The `enable_central_service_config` option now defaults to true. [[GH-8746](https://github.com/hashicorp/consul/issues/8746)]
* connect: Switch the default gateway port from 443 to 8443 to avoid assumption of Envoy running as root. [[GH-9113](https://github.com/hashicorp/consul/issues/9113)]
* connect: Update Envoy metrics names and labels for proxy listeners so that attributes like datacenter and namespace can be extracted. [[GH-9207](https://github.com/hashicorp/consul/issues/9207)]
* connect: intention destinations can no longer be reassigned [[GH-8834](https://github.com/hashicorp/consul/issues/8834)]
* raft: Raft protocol v2 is no longer supported. If currently using protocol v2 then an intermediate upgrade to a version supporting both v2 and v3 protocols will be necessary (1.0.0 - 1.8.x). Note that the Raft protocol configured with the `raft_protocol` setting and the Consul RPC protocol configured with the `protocol` setting and output by the `consul version` command are distinct and supported Consul RPC protocol versions are not altered. [[GH-9103](https://github.com/hashicorp/consul/issues/9103)]
* sentinel: **(Consul Enterprise only)** update to v0.16.0, which replaces `whitelist` and `blacklist` with `allowlist` and `denylist`
* server: **(Enterprise only)** Pre-existing intentions defined with
non-existent destination namespaces were non-functional and are erased during
the upgrade process. This should not matter as these intentions had nothing to
enforce. [[GH-9186](https://github.com/hashicorp/consul/issues/9186)]
* server: **(OSS only)** Pre-existing intentions defined with either a source or
destination namespace value that is not "default" are rewritten or deleted
during the upgrade process. Wildcards first attempt to downgrade to "default"
unless an intention already exists, otherwise these non-functional intentions
are deleted. [[GH-9186](https://github.com/hashicorp/consul/issues/9186)]
* xds: Drop support for Envoy versions 1.12.0, 1.12.1, 1.12.2, and 1.13.0, due to a lack of support for url_path in RBAC. [[GH-8839](https://github.com/hashicorp/consul/issues/8839)]

SECURITY:

* Fix Consul Enterprise Namespace Config Entry Replication DoS. Previously an operator with service:write ACL permissions in a Consul Enterprise cluster could write a malicious config entry that caused infinite raft writes due to issues with the namespace replication logic. [[CVE-2020-25201](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-25201)] [[GH-9024](https://github.com/hashicorp/consul/issues/9024)]
* Increase the permissions to read from the `/connect/ca/configuration` endpoint to `operator:write`. Previously Connect CA configuration, including the private key, set via this endpoint could be read back by an operator with `operator:read` privileges. [CVE-2020-28053](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-28053) [[GH-9240](https://github.com/hashicorp/consul/issues/9240)]

FEATURES:

* agent: Add a new RPC endpoint for streaming cluster state change events to clients.
* agent: Allow client agents to be configured with an advertised reconnect timeout to control how long until the nodes are reaped by others in the cluster. [[GH-8781](https://github.com/hashicorp/consul/issues/8781)]
* agent: moved ui config options to a new `ui_config` stanza in agent configuration and added new options to display service metrics in the UI. [[GH-8694](https://github.com/hashicorp/consul/issues/8694)]
* agent: return the default ACL policy to callers as a header [[GH-9101](https://github.com/hashicorp/consul/issues/9101)]
* autopilot: A new `/v1/operator/autopilot/state` HTTP API was created to give greater visibility into what autopilot is doing and how it has classified all the servers it is tracking. [[GH-9103](https://github.com/hashicorp/consul/issues/9103)]
* autopilot: Added a new `consul operator autopilot state` command to retrieve and view the Autopilot state from consul. [[GH-9142](https://github.com/hashicorp/consul/issues/9142)]
* cli: update `snapshot inspect` command to provide more detailed snapshot data [[GH-8787](https://github.com/hashicorp/consul/issues/8787)]
* connect: support defining intentions using layer 7 criteria [[GH-8839](https://github.com/hashicorp/consul/issues/8839)]
* telemetry: add initialization and definition for non-expiring key metrics in Prometheus [[GH-9088](https://github.com/hashicorp/consul/issues/9088)]
* telemetry: track node and service counts and emit them as metrics [[GH-8603](https://github.com/hashicorp/consul/issues/8603)]
* ui: If Prometheus is being used for monitoring the sidecars, the topology view can be configured to display overview metrics for the services. [[GH-8858](https://github.com/hashicorp/consul/issues/8858)]
* ui: Services using Connect with Envoy sidecars have a topology tab in the UI showing their upstream and downstream services. [[GH-8788](https://github.com/hashicorp/consul/issues/8788)]
* xds: use envoy's rbac filter to handle intentions entirely within envoy [[GH-8569](https://github.com/hashicorp/consul/issues/8569)]

IMPROVEMENTS:

* agent: Return HTTP 429 when connections per clients limit (`limits.http_max_conns_per_client`) has been reached. [[GH-8221](https://github.com/hashicorp/consul/issues/8221)]
* agent: add path_allowlist config option to restrict metrics proxy queries [[GH-9059](https://github.com/hashicorp/consul/issues/9059)]
* agent: allow the /v1/connect/intentions/match endpoint to use the agent cache [[GH-8875](https://github.com/hashicorp/consul/issues/8875)]
* agent: protect the metrics proxy behind ACLs [[GH-9099](https://github.com/hashicorp/consul/issues/9099)]
* api: The `v1/connect/ca/roots` endpoint now accepts a `pem=true` query parameter and will return a PEM encoded certificate chain of all the certificates that would normally be in the JSON version of the response. [[GH-8774](https://github.com/hashicorp/consul/issues/8774)]
* api: support GetMeta() and GetNamespace() on all config entry kinds [[GH-8764](https://github.com/hashicorp/consul/issues/8764)]
* autopilot: **(Enterprise Only)** Autopilot now supports using both Redundancy Zones and Automated Upgrades together. [[GH-9103](https://github.com/hashicorp/consul/issues/9103)]
* checks: add health status to the failure message when gRPC healthchecks fail. [[GH-8726](https://github.com/hashicorp/consul/issues/8726)]
* chore: Update to Go 1.15 with mitigation for [golang/go#42138](https://github.com/golang/go/issues/42138) [[GH-9036](https://github.com/hashicorp/consul/issues/9036)]
* command: remove conditional envoy bootstrap generation for versions <=1.10.0 since those are not supported [[GH-8855](https://github.com/hashicorp/consul/issues/8855)]
* connect: The Vault provider will now automatically renew the lease of the token used, if supported. [[GH-8560](https://github.com/hashicorp/consul/issues/8560)]
* connect: add support for specifying load balancing policy in service-resolver [[GH-8585](https://github.com/hashicorp/consul/issues/8585)]
* connect: intentions are now managed as a new config entry kind "service-intentions" [[GH-8834](https://github.com/hashicorp/consul/issues/8834)]
* raft: Update raft to v1.2.0 to prevent non-voters from becoming eligible for leader elections and adding peer id as metric label to reduce cardinality in metric names [[GH-8822](https://github.com/hashicorp/consul/issues/8822)]
* server: **(Consul Enterprise only)** ensure that we also shutdown network segment serf instances on server shutdown [[GH-8786](https://github.com/hashicorp/consul/issues/8786)]
* server: break up Intention.Apply monolithic method [[GH-9007](https://github.com/hashicorp/consul/issues/9007)]
* server: create new memdb table for storing system metadata [[GH-8703](https://github.com/hashicorp/consul/issues/8703)]
* server: make sure that the various replication loggers use consistent logging [[GH-8745](https://github.com/hashicorp/consul/issues/8745)]
* server: remove config entry CAS in legacy intention API bridge code [[GH-9151](https://github.com/hashicorp/consul/issues/9151)]
* snapshot agent: Deregister critical snapshotting TTL check if leadership is transferred.
* telemetry: All metrics should be present and available to prometheus scrapers when Consul starts. If any non-deprecated metrics are missing please submit an issue with its name. [[GH-9198](https://github.com/hashicorp/consul/issues/9198)]
* telemetry: add config flag `telemetry { disable_compat_1.9 = (true|false) }` to disable deprecated metrics in 1.9 [[GH-8877](https://github.com/hashicorp/consul/issues/8877)]
* telemetry: add counter `consul.api.http` with labels for each HTTP path and method. This is intended to replace `consul.http...` [[GH-8877](https://github.com/hashicorp/consul/issues/8877)]
* ui: Add the Upstreams and Exposed Paths tabs for services in mesh [[GH-9141](https://github.com/hashicorp/consul/issues/9141)]
* ui: Moves the Proxy health checks to be displayed with the Service health check under the Health Checks tab [[GH-9141](https://github.com/hashicorp/consul/issues/9141)]
* ui: Upstream and downstream services in the topology tab will show a visual indication if a deny intention or intention with L7 policies is configured. [[GH-8846](https://github.com/hashicorp/consul/issues/8846)]
* ui: add dashboard_url_template config option for external dashboard links [[GH-9002](https://github.com/hashicorp/consul/issues/9002)]

DEPRECATIONS:

* Go 1.15 has dropped support for 32-bit binaries for Darwin, so darwin_386 builds will not be available for any 1.9.x+ releases. [[GH-9036](https://github.com/hashicorp/consul/issues/9036)]
* agent: `ui`, `ui_dir` and `ui_content_path` are now deprecated for use in agent configuration files. Use `ui_config.{enabled, dir, content_path}` instead. The command arguments `-ui`, `-ui-dir`, and `-ui-content-path` remain supported. [[GH-8694](https://github.com/hashicorp/consul/issues/8694)]
* telemetry: The measurements in all of the `consul.http...` prefixed metrics have been migrated to `consul.api.http`. `consul.http...` prefixed metrics will be removed in a future version of Consul. [[GH-8877](https://github.com/hashicorp/consul/issues/8877)]
* telemetry: the disable_compat_1.9 config will cover more metrics deprecations in future 1.9 point releases. These metrics will be emitted twice for backwards compatibility - if the flag is true, only the new metric name will be written. [[GH-9181](https://github.com/hashicorp/consul/issues/9181)]

BUG FIXES:

* agent: make the json/hcl decoding of ConnectProxyConfig fully work with CamelCase and snake_case [[GH-8741](https://github.com/hashicorp/consul/issues/8741)]
* agent: when enable_central_service_config is enabled ensure agent reload doesn't revert check state to critical [[GH-8747](https://github.com/hashicorp/consul/issues/8747)]
* api: Fixed a bug where the Check.GRPCUseTLS field could not be set using snake case. [[GH-8771](https://github.com/hashicorp/consul/issues/8771)]
* api: Fixed a bug where additional headers configured with `http_config.response_headers` would not be served on index and error pages [[GH-8694](https://github.com/hashicorp/consul/pull/8694/files#diff-160c9abf1b1868a8505065ab02d736fd2dc522a7a555d57383e8428883dc7755R545-R548)]
* autopilot: **(Enterprise Only)** Previously servers in other zones would not be promoted when all servers in a second zone had failed. Now the actual behavior matches the docs and autopilot will promote a healthy non-voter from any zone to replace failure of an entire zone. [[GH-9103](https://github.com/hashicorp/consul/issues/9103)]
* autopilot: Prevent panic when requesting the autopilot health immediately after a leader is elected. [[GH-9204](https://github.com/hashicorp/consul/issues/9204)]
* command: when generating envoy bootstrap configs use the datacenter returned from the agent services endpoint [[GH-9229](https://github.com/hashicorp/consul/issues/9229)]
* connect: Fixed an issue where the Vault intermediate was not renewed in the primary datacenter. [[GH-8784](https://github.com/hashicorp/consul/issues/8784)]
* connect: fix Vault provider not respecting IntermediateCertTTL [[GH-8646](https://github.com/hashicorp/consul/issues/8646)]
* connect: fix connect sidecars registered via the API not being automatically deregistered with their parent service after an agent restart by persisting the LocallyRegisteredAsSidecar property. [[GH-8924](https://github.com/hashicorp/consul/issues/8924)]
* connect: use stronger validation that ingress gateways have compatible protocols defined for their upstreams [[GH-8470](https://github.com/hashicorp/consul/issues/8470)]
* license: (Enterprise only) Fixed an issue where the UI would see Namespaces and SSO as licensed when they were not.
* license: **(Enterprise only)** Fixed an issue where warnings about Namespaces being unlicensed would be emitted erroneously.
* namespace: **(Enterprise Only)** Fixed a bug that could case snapshot restoration to fail when it contained a namespace marked for deletion while still containing other resources in that namespace. [[GH-9156](https://github.com/hashicorp/consul/issues/9156)]
* namespace: **(Enterprise Only)** Fixed an issue where namespaced services and checks were not being deleted when the containing namespace was deleted.
* raft: (Enterprise only) properly update consul server meta non_voter for non-voting Enterprise Consul servers [[GH-8731](https://github.com/hashicorp/consul/issues/8731)]
* server: skip deleted and deleting namespaces when migrating intentions to config entries [[GH-9186](https://github.com/hashicorp/consul/issues/9186)]
* telemetry: fixed a bug that caused logs to be flooded with `[WARN] agent.router: Non-server in server-only area` [[GH-8685](https://github.com/hashicorp/consul/issues/8685)]
* ui: show correct datacenter for gateways [[GH-8704](https://github.com/hashicorp/consul/issues/8704)]

## 1.8.17 (November 11, 2021)

SECURITY:

* namespaces: **(Enterprise only)** Creating or editing namespaces that include default ACL policies or ACL roles now requires `acl:write` permission in the default namespace.  This change fixes CVE-2021-41805.

IMPROVEMENTS:

* ci: Artifact builds will now only run on merges to the release branches or to `main` [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* ci: The Linux packages are now available for all supported Linux architectures including arm, arm64, 386, and amd64 [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]
* ci: The Linux packaging service configs and pre/post install scripts are now available under  [.release/linux] [[GH-11417](https://github.com/hashicorp/consul/issues/11417)]

BUG FIXES:

* acl: fixes the fallback behaviour of down_policy with setting extend-cache/async-cache when the token is not cached. [[GH-11136](https://github.com/hashicorp/consul/issues/11136)]
* raft: Consul leaders will attempt to transfer leadership to another server as part of gracefully leaving the cluster. [[GH-11242](https://github.com/hashicorp/consul/issues/11242)]
* rpc: only attempt to authorize the DNSName in the client cert when verify_incoming_rpc=true [[GH-11255](https://github.com/hashicorp/consul/issues/11255)]
* server: **(Enterprise only)** Ensure that servers leave network segments when leaving other gossip pools
* ui: Fixed styling of Role delete confirmation button with the Token edit page [[GH-11297](https://github.com/hashicorp/consul/issues/11297)]
* xds: fixes a bug where replacing a mesh gateway node used for WAN federation (with another that has a different IP) could leave gateways in the other DC unable to re-establish the connection [[GH-11522](https://github.com/hashicorp/consul/issues/11522)]

## 1.8.16 (September 27, 2021)

FEATURES:

* sso/oidc: **(Enterprise only)** Add support for providing acr_values in OIDC auth flow [[GH-11026](https://github.com/hashicorp/consul/issues/11026)]

IMPROVEMENTS:

* audit-logging: **(Enterprise Only)** Audit logs will now include select HTTP headers in each logs payload. Those headers are: `Forwarded`, `Via`, `X-Forwarded-For`, `X-Forwarded-Host` and `X-Forwarded-Proto`. [[GH-11107](https://github.com/hashicorp/consul/issues/11107)]

BUG FIXES:

* tls: consider presented intermediates during server connection tls handshake. [[GH-10964](https://github.com/hashicorp/consul/issues/10964)]
* ui: **(Enterprise Only)** Fixes a visual issue where namespaces would "double up" in the Source/Destination select menu when creating/editing intentions [[GH-11102](https://github.com/hashicorp/consul/issues/11102)]

## 1.8.15 (August 27, 2021)

KNOWN ISSUES:

* tls: The fix for [CVE-2021-37219](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-37219) introduced an issue that could prevent TLS certificate validation when intermediate CA certificates used to sign server certificates are transmitted in the TLS session but are not present in all Consul server's configured CA certificates. This has the effect of preventing Raft RPCs between the affected servers. As a work around until the next patch releases, ensure that all intermediate CA certificates are present in all Consul server configurations prior to using certificates that they have signed.

SECURITY:

* rpc: authorize raft requests [CVE-2021-37219](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-37219) [[GH-10933](https://github.com/hashicorp/consul/issues/10933)]

IMPROVEMENTS:

* areas: **(Enterprise only)** Add 15s timeout to opening streams over pooled connections.
* areas: **(Enterprise only)** Apply backpressure to area gossip packet ingestion when more than 512 packets are waiting to be ingested.
* areas: **(Enterprise only)** Make implementation of WriteToAddress non-blocking to avoid slowing down memberlist's packetListen routine.
* deps: update to gogo/protobuf v1.3.2 [[GH-10813](https://github.com/hashicorp/consul/issues/10813)]

BUG FIXES:

* acl: fixes a bug that prevented the default user token from being used to authorize service registration for connect proxies. [[GH-10824](https://github.com/hashicorp/consul/issues/10824)]
* ca: fixed a bug when ca provider fail and provider state is stuck in `INITIALIZING` state. [[GH-10630](https://github.com/hashicorp/consul/issues/10630)]
* ca: report an error when setting the ca config fail because of an index check. [[GH-10657](https://github.com/hashicorp/consul/issues/10657)]
* cli: Ensure the metrics endpoint is accessible when Envoy is configured to use
a non-default admin bind address. [[GH-10757](https://github.com/hashicorp/consul/issues/10757)]
* cli: Fix a bug which prevented initializing a watch when using a namespaced
token. [[GH-10795](https://github.com/hashicorp/consul/issues/10795)]
* connect: proxy upstreams inherit namespace from service if none are defined. [[GH-10688](https://github.com/hashicorp/consul/issues/10688)]
* dns: fixes a bug with edns truncation where the response could exceed the size limit in some cases. [[GH-10009](https://github.com/hashicorp/consul/issues/10009)]
* txn: fixes Txn.Apply to properly authorize service registrations. [[GH-10798](https://github.com/hashicorp/consul/issues/10798)]

## 1.8.14 (July 15, 2021)

SECURITY:

* xds: ensure envoy verifies the subject alternative name for upstreams [CVE-2021-32574](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-32574) [[GH-10621](https://github.com/hashicorp/consul/issues/10621)]

BUG FIXES:

* ca: Fixed a bug that returned a malformed certificate chain when the certificate did not having a trailing newline. [[GH-10411](https://github.com/hashicorp/consul/issues/10411)]

## 1.8.13 (June 21, 2021)

IMPROVEMENTS:

* debug: capture a single stream of logs, and single pprof profile and trace for the whole duration [[GH-10279](https://github.com/hashicorp/consul/issues/10279)]
* licensing: **(Enterprise Only)** In order to have forward compatibility with Consul Enterprise v1.10, the ability to parse licenses from the configuration or environment has been added. This can be specified with the `license_path` configuration, the `CONSUL_LICENSE` environment variable or the `CONSUL_LICENSE_PATH` environment variable. On server agents this configuration will be ignored. Client agents and the snapshot agent will use the configured license instead of automatically retrieving one. [[GH-10442](https://github.com/hashicorp/consul/issues/10442)]
* monitoring: optimize the monitoring endpoint to avoid losing logs when under high load. [[GH-10368](https://github.com/hashicorp/consul/issues/10368)]

BUG FIXES:

* license: **(Enterprise only)** Fixed an issue that would cause client agents on versions before 1.10 to not be able to retrieve the license from a 1.10+ server. [[GH-10432](https://github.com/hashicorp/consul/issues/10432)]
* monitor: fix monitor to produce json format logs when requested [[GH-10358](https://github.com/hashicorp/consul/issues/10358)]

## 1.8.12 (June 04, 2021)

BUG FIXES:

* agent: fix logging output by removing leading whitespace from every log line [[GH-10338](https://github.com/hashicorp/consul/issues/10338)]
* cli: removes the need to set debug_enabled=true to collect debug data from the CLI. Now
the CLI behaves the same way as the API and accepts either an ACL token with operator:read, or
debug_enabled=true. [[GH-10273](https://github.com/hashicorp/consul/issues/10273)]
* envoy: fixes a bug where a large envoy config could cause the `consul connect envoy` command to deadlock when attempting to start envoy. [[GH-10324](https://github.com/hashicorp/consul/issues/10324)]
* namespaces: **(Enterprise only)** fixes a problem where the logs would contain many warnings about namespaces not being licensed.

## 1.8.11 (June 03, 2021)

IMPROVEMENTS:

* areas: **(Enterprise only)** Use server agent's gossip_wan config when setting memberlist configuration for network areas. Previously they used memberlists WAN defaults.
* cli: added a `-force-without-cross-signing` flag to the `ca set-config` command.
connect/ca: The ForceWithoutCrossSigning field will now work as expected for CA providers that support cross signing. [[GH-9672](https://github.com/hashicorp/consul/issues/9672)]
* connect: update supported envoy versions to 1.14.7, 1.13.7, 1.12.7, 1.11.2 [[GH-10106](https://github.com/hashicorp/consul/issues/10106)]
* telemetry: Add new metrics for status of secondary datacenter replication. [[GH-10073](https://github.com/hashicorp/consul/issues/10073)]

BUG FIXES:

* agent: ensure we hash the non-deprecated upstream fields on ServiceConfigRequest [[GH-10240](https://github.com/hashicorp/consul/issues/10240)]
* api: include the default value of raft settings in the output of /v1/agent/self [[GH-8812](https://github.com/hashicorp/consul/issues/8812)]
* areas: **(Enterprise only)** Revert to the 10s dial timeout used before connection pooling was introduced in 1.7.3.
* areas: **(Enterprise only)** Selectively merge gossip_wan config for network areas to avoid attempting to enable gossip encryption where it was not intended or necessary.
* local: agents will no longer persist the default user token along with a service or check. [[GH-10188](https://github.com/hashicorp/consul/issues/10188)]
* server: ensure that central service config flattening properly resets the state each time [[GH-10239](https://github.com/hashicorp/consul/issues/10239)]

## 1.8.10 (April 15, 2021)

SECURITY:

* Add content-type headers to raw KV responses to prevent XSS attacks [CVE-2020-25864](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-25864) [[GH-10023](https://github.com/hashicorp/consul/issues/10023)]
* audit-logging: Parse endpoint URL to prevent requests from bypassing the audit log [CVE-2021-28156](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-28156)

BUG FIXES:

* areas: Fixes a bug which would prevent newer servers in a network areas from connecting to servers running a version of Consul prior to 1.7.3.
* audit-logging: (Enterprise only) Fixed an issue that resulted in usage of the agent master token or managed service provider tokens from being resolved properly. [[GH-10013](https://github.com/hashicorp/consul/issues/10013)]
* command: when generating envoy bootstrap configs to stdout do not mix informational logs into the json [[GH-9980](https://github.com/hashicorp/consul/issues/9980)]
* config: correct config key from `advertise_addr_ipv6` to `advertise_addr_wan_ipv6` [[GH-9851](https://github.com/hashicorp/consul/issues/9851)]
* snapshot: fixes a bug that would cause snapshots to be missing all but the first ACL Auth Method. [[GH-10025](https://github.com/hashicorp/consul/issues/10025)]

## 1.8.9 (March 04, 2021)

IMPROVEMENTS:

* cli: Add new `-cluster-id` and `common-name` to `consul tls ca create` to support creating a CA for Consul Connect. [[GH-9585](https://github.com/hashicorp/consul/issues/9585)]
* connect: if the token given to the vault provider returns no data avoid a panic [[GH-9806](https://github.com/hashicorp/consul/issues/9806)]
* connect: update supported envoy point releases to 1.14.6, 1.13.7, 1.12.7, 1.11.2 [[GH-9739](https://github.com/hashicorp/consul/issues/9739)]
* license: **(Enterprise only)** Temporary client license duration was increased from 30m to 6h.
* server: use the presense of stored federation state data as a sign that we already activated the federation state feature flag [[GH-9519](https://github.com/hashicorp/consul/issues/9519)]
* xds: only try to create an ipv6 expose checks listener if ipv6 is supported by the kernel [[GH-9765](https://github.com/hashicorp/consul/issues/9765)]

BUG FIXES:

* api: Remove trailing periods from the gateway internal HTTP API endpoint [[GH-9752](https://github.com/hashicorp/consul/issues/9752)]
* cache: Prevent spamming the logs for days when a cached request encounters an "ACL not found" error. [[GH-9738](https://github.com/hashicorp/consul/issues/9738)]
* connect: connect CA Roots in the primary datacenter should use a SigningKeyID derived from their local intermediate [[GH-9428](https://github.com/hashicorp/consul/issues/9428)]
* proxycfg: avoid potential deadlock in delivering proxy snapshot to watchers. [[GH-9689](https://github.com/hashicorp/consul/issues/9689)]
* server: When wan federating via mesh gateways after initial federation default to using the local mesh gateways unless the heuristic indicates a bypass is required. [[GH-9528](https://github.com/hashicorp/consul/issues/9528)]
* server: When wan federating via mesh gateways only do heuristic primary DC bypass on the leader. [[GH-9366](https://github.com/hashicorp/consul/issues/9366)]
* xds: deduplicate mesh gateway listeners by address in a stable way to prevent some LDS churn [[GH-9650](https://github.com/hashicorp/consul/issues/9650)]
* xds: prevent LDS flaps in mesh gateways due to unstable datacenter lists; also prevent some flaps in terminating gateways as well [[GH-9651](https://github.com/hashicorp/consul/issues/9651)]
* 
## 1.8.8 (January 22, 2021)

BUG FIXES:

* connect: Fixed a bug in the AWS PCA Connect CA provider that could cause the intermediate PKI path to be deleted after reconfiguring the CA [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* connect: Fixed a bug in the Vault Connect CA provider that could cause the intermediate PKI path to be deleted after reconfiguring the CA [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* connect: Fixed an issue that would prevent updating the Connect CA configuration if the CA provider didn't complete initialization previously. [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* leader: Fixed a bug that could cause Connect CA initialization failures from allowing leader establishment to complete resulting in potentially infinite leader elections. [[GH-9498](https://github.com/hashicorp/consul/issues/9498)]
* rpc: Prevent misleading RPC error claiming the lack of a leader when Raft is ok but there are issues with client agents gossiping with the leader. [[GH-9487](https://github.com/hashicorp/consul/issues/9487)]
* ui: ensure namespace is used for node API requests [[GH-9488](https://github.com/hashicorp/consul/issues/9488)]

## 1.8.7 (December 10, 2020)

BUG FIXES:

* acl: global tokens created by auth methods now correctly replicate to secondary datacenters [[GH-9351](https://github.com/hashicorp/consul/issues/9351)]
* connect: fixes a case when updating the CA config in a secondary datacenter to correctly trigger the creation of a new intermediate certificate [[GH-9009](https://github.com/hashicorp/consul/issues/9009)]
* connect: only unset the active root in a secondary datacenter when a new one is replacing it [[GH-9318](https://github.com/hashicorp/consul/issues/9318)]
* license: (Enterprise only) Fixed an issue where the UI would see Namespaces and SSO as licensed when they were not.
* license: **(Enterprise only)** Fixed an issue where warnings about Namespaces being unlicensed would be emitted erroneously.
* namespace: **(Enterprise Only)** Fixed a bug that could case snapshot restoration to fail when it contained a namespace marked for deletion while still containing other resources in that namespace. [[GH-9156](https://github.com/hashicorp/consul/issues/9156)]
* namespace: **(Enterprise Only)** Fixed an issue where namespaced services and checks were not being deleted when the containing namespace was deleted.
* namespaces: **(Enterprise only)** Prevent stalling of replication in secondary datacenters due to conflicts between the namespace replicator and other replicators. [[GH-9271](https://github.com/hashicorp/consul/issues/9271)]

## 1.8.6 (November 19, 2020)

SECURITY:

* Increase the permissions to read from the `/connect/ca/configuration` endpoint to `operator:write`. Previously Connect CA configuration, including the private key, set via this endpoint could be read back by an operator with `operator:read` privileges. [CVE-2020-28053](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-28053) [[GH-9240](https://github.com/hashicorp/consul/issues/9240)]

## 1.8.5 (October 23, 2020)

SECURITY:

* Fix Consul Enterprise Namespace Config Entry Replication DoS. Previously an operator with service:write ACL permissions in a Consul Enterprise cluster could write a malicious config entry that caused infinite raft writes due to issues with the namespace replication logic. [[CVE-2020-25201](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-25201)] [[GH-9024](https://github.com/hashicorp/consul/issues/9024)]

IMPROVEMENTS:

* api: The `v1/connect/ca/roots` endpoint now accepts a `pem=true` query parameter and will return a PEM encoded certificate chain of all the certificates that would normally be in the JSON version of the response. [[GH-8774](https://github.com/hashicorp/consul/issues/8774)]
* connect: The Vault provider will now automatically renew the lease of the token used, if supported. [[GH-8560](https://github.com/hashicorp/consul/issues/8560)]
* connect: update supported envoy releases to 1.14.5, 1.13.6, 1.12.7, 1.11.2 for 1.8.x [[GH-8999](https://github.com/hashicorp/consul/issues/8999)]

BUG FIXES:

* agent: when enable_central_service_config is enabled ensure agent reload doesn't revert check state to critical [[GH-8747](https://github.com/hashicorp/consul/issues/8747)]
* connect: Fixed an issue where the Vault intermediate was not renewed in the primary datacenter. [[GH-8784](https://github.com/hashicorp/consul/issues/8784)]
* connect: fix Vault provider not respecting IntermediateCertTTL [[GH-8646](https://github.com/hashicorp/consul/issues/8646)]
* connect: fix connect sidecars registered via the API not being automatically deregistered with their parent service after an agent restart by persisting the LocallyRegisteredAsSidecar property. [[GH-8924](https://github.com/hashicorp/consul/issues/8924)]
* fixed a bug that caused logs to be flooded with `[WARN] agent.router: Non-server in server-only area` [[GH-8685](https://github.com/hashicorp/consul/issues/8685)]
* ui: show correct datacenter for gateways [[GH-8704](https://github.com/hashicorp/consul/issues/8704)]

## 1.8.4 (September 11, 2020)

FEATURES:

* agent: expose the list of supported envoy versions on /v1/agent/self [[GH-8545](https://github.com/hashicorp/consul/issues/8545)]
* cache: Config parameters for cache throttling are now reloaded automatically on agent reload. Restarting the agent is not needed anymore. [[GH-8552](https://github.com/hashicorp/consul/issues/8552)]
* connect: all config entries pick up a meta field [[GH-8596](https://github.com/hashicorp/consul/issues/8596)]

IMPROVEMENTS:

* api: Added `ACLMode` method to the `AgentMember` type to determine what ACL mode the agent is operating in. [[GH-8575](https://github.com/hashicorp/consul/issues/8575)]
* api: Added `IsConsulServer` method to the `AgentMember` type to easily determine whether the agent is a server. [[GH-8575](https://github.com/hashicorp/consul/issues/8575)]
* api: Added constants for common tag keys and values in the `Tags` field of the `AgentMember` struct. [[GH-8575](https://github.com/hashicorp/consul/issues/8575)]
* api: Allow for the client to use TLS over a Unix domain socket. [[GH-8602](https://github.com/hashicorp/consul/issues/8602)]
* api: `GET v1/operator/keyring` also lists primary keys. [[GH-8522](https://github.com/hashicorp/consul/issues/8522)]
* connect: Add support for http2 and grpc to ingress gateways [[GH-8458](https://github.com/hashicorp/consul/issues/8458)]
* serf: update to `v0.9.4` which supports primary keys in the ListKeys operation. [[GH-8522](https://github.com/hashicorp/consul/issues/8522)]

BUGFIXES:

* connect: use stronger validation that ingress gateways have compatible protocols defined for their upstreams [[GH-8494](https://github.com/hashicorp/consul/issues/8494)]
* agent: ensure that we normalize bootstrapped config entries [[GH-8547](https://github.com/hashicorp/consul/issues/8547)]
* api: Fixed a panic caused by an api request with Connect=null [[GH-8537](https://github.com/hashicorp/consul/issues/8537)]
* connect: `connect envoy` command now respects the `-ca-path` flag [[GH-8606](https://github.com/hashicorp/consul/issues/8606)]
* connect: fix bug in preventing some namespaced config entry modifications [[GH-8601](https://github.com/hashicorp/consul/issues/8601)]
* connect: fix renewing secondary intermediate certificates [[GH-8588](https://github.com/hashicorp/consul/issues/8588)]
* ui: fixed a bug related to in-folder KV creation [[GH-8613](https://github.com/hashicorp/consul/issues/8613)]

## 1.8.3 (August 12, 2020)

BUGFIXES:

* catalog: fixed a bug where nodes, services, and checks would not be restored with the correct Create/ModifyIndex when restoring from a snapshot [[GH-8485](https://github.com/hashicorp/consul/pull/8474)]
* vendor: update github.com/armon/go-metrics to v0.3.4 to mitigate a potential panic when emitting Prometheus metrics at an interval longer than the metric expiry time [[GH-8478](https://github.com/hashicorp/consul/pull/8478)]
* connect: **(Consul Enterprise only)** Fixed a regression that prevented mesh gateways from routing to services in their local datacenter that reside outside of the default namespace.

## 1.8.2 (August 07, 2020)

* auto_config: Fixed an issue where auto-config could be enabled in secondary DCs without enabling token replication when ACLs were enabled. [[GH-8451](https://github.com/hashicorp/consul/pull/8451)]
* xds: revert setting set_node_on_first_message_only to true when generating envoy bootstrap config [[GH-8440](https://github.com/hashicorp/consul/issues/8440)]

## 1.8.1 (July 30, 2020)

FEATURES:

* acl: Added ACL Node Identities for easier creation of Consul Agent tokens. [[GH-7970](https://github.com/hashicorp/consul/pull/7970)]
* agent: Added Consul client agent automatic configuration utilizing JWTs for authorizing the request to generate ACL tokens, TLS certificates and retrieval of the gossip encryption key. [[GH-8003](https://github.com/hashicorp/consul/pull/8003)], [[GH-8035](https://github.com/hashicorp/consul/pull/8035)], [[GH-8086](https://github.com/hashicorp/consul/pull/8086)], [[GH-8148](https://github.com/hashicorp/consul/pull/8148)], [[GH-8157](https://github.com/hashicorp/consul/pull/8157)], [[GH-8159](https://github.com/hashicorp/consul/pull/8159)], [[GH-8193](https://github.com/hashicorp/consul/pull/8193)], [[GH-8253](https://github.com/hashicorp/consul/pull/8253)], [[GH-8301](https://github.com/hashicorp/consul/pull/8301)], [[GH-8360](https://github.com/hashicorp/consul/pull/8360)], [[GH-8362](https://github.com/hashicorp/consul/pull/8362)], [[GH-8363](https://github.com/hashicorp/consul/pull/8363)], [[GH-8364](https://github.com/hashicorp/consul/pull/8364)], [[GH-8409](https://github.com/hashicorp/consul/pull/8409)]

IMPROVEMENTS:

* acl: allow auth methods created in the primary datacenter to optionally create global tokens [[GH-7899](https://github.com/hashicorp/consul/issues/7899)]
* agent: Allow to restrict servers that can join a given Serf Consul cluster. [[GH-7628](https://github.com/hashicorp/consul/issues/7628)]
* agent: new configuration options allow ratelimiting of the agent-cache: `cache.entry_fetch_rate` and `cache.entry_fetch_max_burst`. [[GH-8226](https://github.com/hashicorp/consul/pull/8226)]
* api: Added methods to allow passing query options to leader and peers endpoints to mirror HTTP API [[GH-8395](https://github.com/hashicorp/consul/pull/8395)]
* auto_config: when configuring auto_config, connect is turned on automatically [[GH-8433](https://github.com/hashicorp/consul/pull/8433)]
* connect: various changes to make namespaces for intentions work more like for other subsystems [[GH-8194](https://github.com/hashicorp/consul/issues/8194)]
* connect: Append port number to expected ingress hosts [[GH-8190](https://github.com/hashicorp/consul/pull/8190)]
* connect: add support for envoy 1.15.0 and drop support for 1.11.x [[GH-8424](https://github.com/hashicorp/consul/issues/8424)]
* connect: support Envoy v1.14.4, v1.13.4, v1.12.6 [[GH-8216](https://github.com/hashicorp/consul/issues/8216)]
* dns: Improve RCODE of response when query targets a non-existent datacenter. [[GH-8102](https://github.com/hashicorp/consul/issues/8102)],[[GH-8218](https://github.com/hashicorp/consul/issues/8218)]
* version: The `version` CLI subcommand was altered to always show the git revision the binary was built from on the second line of output. Additionally the command gained a `-format` flag with the option now of outputting the version information in JSON form. **NOTE** This change has the potential to break any parsing done by users of the `version` commands output. In many cases nothing will need to be done but it is possible depending on how the output is parsed. [[GH-8268](https://github.com/hashicorp/consul/pull/8268)]

BUGFIXES:

* agent: Fixed a bug where Consul could crash when `verify_outgoing` was set to true but no client certificate was used. [[GH-8211](https://github.com/hashicorp/consul/pull/8211)]
* agent: Fixed an issue with lock contention during RPCs when under load while using the Prometheus metrics sink. [[GH-8372](https://github.com/hashicorp/consul/pull/8372)]
* auto_encrypt: Fixed an issue where auto encrypt certificate signing wasn't using the connect signing rate limiter. [[GH-8211](https://github.com/hashicorp/consul/pull/8211)]
* auto_encrypt: Fixed several issues around retrieving the first TLS certificate where it would have the wrong CN and SANs. This was being masked by a second bug (also fixed) causing that certificate to immediately be discarded with a second certificate request being made afterwards. [[GH-8211](https://github.com/hashicorp/consul/pull/8211)]
* auto_encrypt: Fixed an issue that caused auto encrypt certificates to not be updated properly if the agents token was changed and the old token was deleted. [[GH-8311](https://github.com/hashicorp/consul/pull/8311)]
* autopilot: **(Consul Enterprise only)** Fixed an issue where using autopilot with redundancy zones wouldn't demote extra voters in a zone to match the "one voter per zone" desired state when rebalancing.
* connect: fix crash that would result if a mesh or terminating gateway's upstream has a hostname as an address and no healthy service instances available. [[GH-8158](https://github.com/hashicorp/consul/issues/8158)]
* connect: Fixed issue where specifying a prometheus bind address would cause ingress gateways to fail to start up [[GH-8371]](https://github.com/hashicorp/consul/pull/8371)
* gossip: Avoid issue where two unique leave events for the same node could lead to infinite rebroadcast storms [[GH-8343](https://github.com/hashicorp/consul/issues/8343)]
* router: Mark its own cluster as healthy when rebalancing. [[GH-8406](https://github.com/hashicorp/consul/pull/8406)]
* snapshot: **(Consul Enterprise only)** Fixed a regression when using Azure blob storage.
* xds: version sniff envoy and switch regular expressions from 'regex' to 'safe_regex' on newer envoy versions [[GH-8222](https://github.com/hashicorp/consul/issues/8222)]

## 1.8.0 (June 18, 2020)

BREAKING CHANGES:
* acl: Remove deprecated `acl_enforce_version_8` option [[GH-7991](https://github.com/hashicorp/consul/issues/7991)]

FEATURES:

* **Terminating Gateway**: Envoy can now be run as a gateway to enable services in a Consul service mesh to connect to external services through their local proxy. Terminating gateways unlock several of the benefits of a service mesh in the cases where a sidecar proxy cannot be deployed alongside services such as legacy applications or managed cloud databases.
* **Ingress Gateway**: Envoy can now be run as a gateway to ingress traffic into the Consul service mesh, enabling a more incremental transition for applications.
* **WAN Federation over Mesh Gateways**: Allows Consul datacenters to federate by forwarding WAN gossip and RPC traffic through Mesh Gateways rather than requiring the servers to be exposed to the WAN directly.
* **JSON Web Token (JWT) Auth Method**: Allows exchanging a signed JWT from a trusted external identity provider for a Consul ACL token.
* **Single Sign-On (SSO) [Enterprise]**: Lets an operator configure Consul to use an external OpenID Connect (OIDC) provider to automatically handle the lifecycle of creating, distributing and managing ACL tokens for performing CLI operations or accessing the UI.
* **Audit Logging [Enterprise]**: Adds instrumentation to record a trail of events (both attempted and authorized) by users of Consul’s HTTP API for purposes of regulatory compliance.

* acl: add DisplayName field to auth methods [[GH-7769](https://github.com/hashicorp/consul/issues/7769)]
* acl: add MaxTokenTTL field to auth methods [[GH-7779](https://github.com/hashicorp/consul/issues/7779)]
* agent/xds: add support for configuring passive health checks [[GH-7713](https://github.com/hashicorp/consul/pull/7713)]
* cli: Add -config flag to "acl authmethod update/create" [[GH-7776](https://github.com/hashicorp/consul/pull/7776)]
* serf: allow to restrict servers that can join a given Serf Consul cluster. [[GH-7628](https://github.com/hashicorp/consul/pull/7628)]
* ui: Help menu to provide further documentation/learn links [[GH-7310](https://github.com/hashicorp/consul/pull/7310)]
* ui: **(Consul Enterprise only)** SSO support [[GH-7742](https://github.com/hashicorp/consul/pull/7742)] [[GH-7771](https://github.com/hashicorp/consul/pull/7771)] [[GH-7790](https://github.com/hashicorp/consul/pull/7790)]
* ui: Support for termininating and ingress gateways [[GH-7858](https://github.com/hashicorp/consul/pull/7858)] [[GH-7865](https://github.com/hashicorp/consul/pull/7865)]

IMPROVEMENTS:

* acl: change authmethod.Validator to take a logger [[GH-7758](https://github.com/hashicorp/consul/issues/7758)]
* agent: show warning when enable_script_checks is enabled without safety net [[GH-7437](https://github.com/hashicorp/consul/pull/7437)]
* api: Added filtering support to the v1/connect/intentions endpoint. [[GH-7478](https://github.com/hashicorp/consul/issues/7478)]
* auto_encrypt: add validations for auto_encrypt.{tls,allow_tls} [[GH-7704](https://github.com/hashicorp/consul/pull/7704)]
* build: switched to compile with Go 1.14.1 [[GH-7481](https://github.com/hashicorp/consul/pull/7481)]
* config: validate system limits against limits.http_max_conns_per_client [[GH-7434](https://github.com/hashicorp/consul/pull/7434)]
* connect: support envoy 1.12.3, 1.13.1, and 1.14.1. Envoy 1.10 is no longer officially supported. [[GH-7380](https://github.com/hashicorp/consul/pull/7380)],[[GH-7624](https://github.com/hashicorp/consul/pull/7624)]
* connect: add DNSSAN and IPSAN to cache key for ConnectCALeafRequest [[GH-7597](https://github.com/hashicorp/consul/pull/7597)]
* connect: Added a new expose CLI command for ingress gateways [[GH-8099](https://github.com/hashicorp/consul/issues/8099)]
* license: **(Consul Enterprise only)** Update licensing to align with the current modules licensing structure.
* logging: catch problems with the log destination earlier by creating the file immediately [[GH-7469](https://github.com/hashicorp/consul/pull/7469)]
* proxycfg: support path exposed with non-HTTP2 protocol [[GH-7510](https://github.com/hashicorp/consul/pull/7510)]
* tls: remove old ciphers [[GH-7282](https://github.com/hashicorp/consul/pull/7282)]
* ui: Show the last 8 characters of AccessorIDs in listing views [[GH-7327](https://github.com/hashicorp/consul/pull/7327)]
* ui: Make all tabs within the UI linkable/bookmarkable and include in history [[GH-7592](https://github.com/hashicorp/consul/pull/7592)]
* ui: Redesign of all service pages [[GH-7605](https://github.com/hashicorp/consul/pull/7605)] [[GH-7632](https://github.com/hashicorp/consul/pull/7632)] [[GH-7655](https://github.com/hashicorp/consul/pull/7655)] [[GH-7683](https://github.com/hashicorp/consul/pull/7683)]
* ui: Show intentions per individual service [[GH-7615](https://github.com/hashicorp/consul/pull/7615)]
* ui: Improved login/logout flow [[GH-7790](https://github.com/hashicorp/consul/pull/7790)]
* ui: Revert search to search as you type, add sort control for the service listing page [[GH-7489](https://github.com/hashicorp/consul/pull/7489)]
* ui: Omit proxy services from the service listing view and mark services as being proxied [[GH-7820](https://github.com/hashicorp/consul/pull/7820)]
* ui: Display proxies in a proxy info tab with the service instance detail page [[GH-7745](https://github.com/hashicorp/consul/pull/7745)]
* ui: Add live updates/blocking queries to gateway listings [[GH-7967](https://github.com/hashicorp/consul/pull/7967)]
* ui: Improved 'empty states'  [[GH-7940](https://github.com/hashicorp/consul/pull/7940)]
* ui: Add ability to sort services based on health  [[GH-7989](https://github.com/hashicorp/consul/pull/7989)]
* ui: Add explanatory tooltip panels for gateway services [[GH-8048]](https://github.com/hashicorp/consul/pull/8048)
* ui: Reduce discovery-chain log errors [[GH-8065]](https://github.com/hashicorp/consul/pull/8065)

BUGFIXES:

* agent: **(Consul Enterprise only)** Fixed several bugs related to Network Area and Network Segment compatibility with other features caused by incorrectly doing version or serf tag checking. [[GH-7491](https://github.com/hashicorp/consul/pull/7491)]
* agent: rewrite checks with proxy address, not local service address [[GH-7518](https://github.com/hashicorp/consul/pull/7518)]
* agent: Preserve ModifyIndex for unchanged entry in KV transaciton [[GH-7832](https://github.com/hashicorp/consul/pull/7832)]
* agent: use default resolver scheme for gRPC dialing [[GH-7617](https://github.com/hashicorp/consul/pull/7617)]
* cache: Fix go routine leak in the agent cache which could cause increasing memory usage. [[GH-8092](https://github.com/hashicorp/consul/pull/8092)]
* cli: enable TLS when `CONSUL_HTTP_ADDR` has an `https` scheme [[GH-7608](https://github.com/hashicorp/consul/pull/7608)]
* connect: Internal refactoring to allow Connect proxy config to contain lists of structured configuration [[GH-7963](https://github.com/hashicorp/consul/issues/7963)][[GH-7964](https://github.com/hashicorp/consul/issues/7964)]
* license: **(Consul Enterprise only)** Fixed a bug that would cause a license reset request to only be applied on the leader server.
* sdk: Fix race condition in freeport [[GH-7567](https://github.com/hashicorp/consul/issues/7567)]
* server: strip local ACL tokens from RPCs during forwarding if crossing datacenters [[GH-7419](https://github.com/hashicorp/consul/issues/7419)]
* ui: Quote service names when filtering intentions to prevent 500 errors when accessing a service [[GH-7896](https://github.com/hashicorp/consul/issues/7896)] [[GH-7888](https://github.com/hashicorp/consul/pull/7888)]
* ui: Miscellaneous amends for Safari and Firefox [[GH-7904](https://github.com/hashicorp/consul/issues/7904)] [[GH-7907](https://github.com/hashicorp/consul/pull/7907)]
* ui: Ensure a value is always passed to CONSUL_SSO_ENABLED [[GH-7913](https://github.com/hashicorp/consul/pull/7913)]

## 1.7.14 (April 15, 2021)

SECURITY:

* Add content-type headers to raw KV responses to prevent XSS attacks [CVE-2020-25864](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-25864) [[GH-10023](https://github.com/hashicorp/consul/issues/10023)]

BUG FIXES:

* command: when generating envoy bootstrap configs to stdout do not mix informational logs into the json [[GH-9980](https://github.com/hashicorp/consul/issues/9980)]
* config: correct config key from `advertise_addr_ipv6` to `advertise_addr_wan_ipv6` [[GH-9851](https://github.com/hashicorp/consul/issues/9851)]
* snapshot: fixes a bug that would cause snapshots to be missing all but the first ACL Auth Method. [[GH-10025](https://github.com/hashicorp/consul/issues/10025)]

## 1.7.13 (March 04, 2021)

IMPROVEMENTS:

* connect: update supported envoy point releases to 1.13.7, 1.12.7, 1.11.2, 1.10.0 [[GH-9740](https://github.com/hashicorp/consul/issues/9740)]
* license: **(Enterprise only)** Temporary client license duration was increased from 30m to 6h.
* xds: only try to create an ipv6 expose checks listener if ipv6 is supported by the kernel [[GH-9765](https://github.com/hashicorp/consul/issues/9765)]

BUG FIXES:

* cache: Prevent spamming the logs for days when a cached request encounters an "ACL not found" error. [[GH-9738](https://github.com/hashicorp/consul/issues/9738)]
* connect: connect CA Roots in the primary datacenter should use a SigningKeyID derived from their local intermediate [[GH-9428](https://github.com/hashicorp/consul/issues/9428)]
* xds: deduplicate mesh gateway listeners by address in a stable way to prevent some LDS churn [[GH-9650](https://github.com/hashicorp/consul/issues/9650)]
* xds: prevent LDS flaps in mesh gateways due to unstable datacenter lists; also prevent some flaps in terminating gateways as well [[GH-9651](https://github.com/hashicorp/consul/issues/9651)]

## 1.7.12 (January 22, 2021)

BUG FIXES:

* rpc: Prevent misleading RPC error claiming the lack of a leader when Raft is ok but there are issues with client agents gossiping with the leader. [[GH-9487](https://github.com/hashicorp/consul/issues/9487)]
* ui: ensure namespace is used for node API requests [[GH-9488](https://github.com/hashicorp/consul/issues/9488)]

## 1.7.11 (December 10, 2020)

BUG FIXES:

* license: **(Enterprise only)** Fixed an issue where warnings about Namespaces being unlicensed would be emitted erroneously.
* namespace: **(Enterprise Only)** Fixed a bug that could case snapshot restoration to fail when it contained a namespace marked for deletion while still containing other resources in that namespace. [[GH-9156](https://github.com/hashicorp/consul/issues/9156)]
* namespace: **(Enterprise Only)** Fixed an issue where namespaced services and checks were not being deleted when the containing namespace was deleted.
* namespaces: **(Enterprise only)** Prevent stalling of replication in secondary datacenters due to conflicts between the namespace replicator and other replicators. [[GH-9271](https://github.com/hashicorp/consul/issues/9271)]

## 1.7.10 (November 19, 2020)

SECURITY:

* Increase the permissions to read from the `/connect/ca/configuration` endpoint to `operator:write`. Previously Connect CA configuration, including the private key, set via this endpoint could be read back by an operator with `operator:read` privileges. [CVE-2020-28053](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-28053) [[GH-9240](https://github.com/hashicorp/consul/issues/9240)]

## 1.7.9 (October 26, 2020)

SECURITY:

* Fix Consul Enterprise Namespace Config Entry Replication DoS. Previously an operator with service:write ACL permissions in a Consul Enterprise cluster could write a malicious config entry that caused infinite raft writes due to issues with the namespace replication logic. [CVE-2020-25201] (https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-25201) [[GH-9024](https://github.com/hashicorp/consul/issues/9024)]

IMPROVEMENTS:

* connect: update supported envoy releases to 1.13.6, 1.12.7, 1.11.2, 1.10.0 for 1.7.x [[GH-9000](https://github.com/hashicorp/consul/issues/9000)]

BUG FIXES:

* agent: when enable_central_service_config is enabled ensure agent reload doesn't revert check state to critical [[GH-8747](https://github.com/hashicorp/consul/issues/8747)]

## 1.7.8 (September 11, 2020)

FEATURES:

* agent: expose the list of supported envoy versions on /v1/agent/self [[GH-8545](https://github.com/hashicorp/consul/issues/8545)]

BUG FIXES:

* connect: fix bug in preventing some namespaced config entry modifications [[GH-8601](https://github.com/hashicorp/consul/issues/8601)]
* api: fixed a panic caused by an api request with Connect=null [[GH-8537](https://github.com/hashicorp/consul/pull/8537)]

## 1.7.7 (August 12, 2020)

BUGFIXES:

* catalog: fixed a bug where nodes, services, and checks would not be restored with the correct Create/ModifyIndex when restoring from a snapshot [[GH-8486](https://github.com/hashicorp/consul/pull/8486)]
* vendor: update github.com/armon/go-metrics to v0.3.4 to mitigate a potential panic when emitting Prometheus metrics at an interval longer than the metric expiry time [[GH-8478](https://github.com/hashicorp/consul/pull/8478)]

## 1.7.6 (August 07, 2020)

BUG FIXES:

* xds: revert setting set_node_on_first_message_only to true when generating envoy bootstrap config [[GH-8441](https://github.com/hashicorp/consul/issues/8441)]
* telemetry: fix for a lock contention issue with the go-metrics Prometheus sink that could cause performance issues [[GH-8372](https://github.com/hashicorp/consul/pull/8372)]
* telemetry: fix for a lock contention issue with the go-metrics Dogstatsd sink that could cause performance issues [[GH-8372](https://github.com/hashicorp/consul/pull/8372)]

## 1.7.5 (July 30, 2020)

BUG FIXES:

* agent: Fixed an issue with lock contention during RPCs when under load while using the Prometheus metrics sink. [[GH-8372](https://github.com/hashicorp/consul/pull/8372)]
* gossip: Avoid issue where two unique leave events for the same node could lead to infinite rebroadcast storms [[GH-8353](https://github.com/hashicorp/consul/issues/8353)]
* snapshot: **(Consul Enterprise only)** Fixed a regression when using Azure blob storage.
* Return a service splitter's weight or a zero [[GH-8355](https://github.com/hashicorp/consul/issues/8355)]

## 1.7.4 (June 10, 2020)

SECURITY:

* Adding an option `http_config.use_cache` to disable agent caching for http endpoints, because Consul’s DNS and HTTP API expose a caching feature susceptible to DoS. [CVE-2020-13250](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-13250) [[GH-8023]](https://github.com/hashicorp/consul/pull/8023)
* Propagate and enforce changes to legacy ACL tokens rules in secondary data centers. [CVE-2020-12797](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-12797) [[GH-8047]](https://github.com/hashicorp/consul/pull/8047)
* Only resolve local acl token in the datacenter it belongs to. [CVE-2020-13170](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-13170) [[GH-8068]](https://github.com/hashicorp/consul/pull/8068)
* Requiring service:write permissions, a service-router entry without a destination no longer crashes Consul servers. [CVE-2020-12758](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-12758) [[GH-7783]](https://github.com/hashicorp/consul/pull/7783)

BUG FIXES:

* acl: Fixed an issue where legacy management tokens could not be used in secondary datacenters. [[GH-7908](https://github.com/hashicorp/consul/pull/7908)]
* agent: Fixed a race condition that could cause an agent to crash when first starting. [[GH-7955](https://github.com/hashicorp/consul/issues/7955)]
* connect: ensure proxy-defaults protocol is used for upstreams [[GH-7938](https://github.com/hashicorp/consul/issues/7938)]
* connect: setup intermediate_pki_path on secondary when using vault [[GH-8001]](https://github.com/hashicorp/consul/pull/8001)

## 1.7.3 (May 05, 2020)

IMPROVEMENTS:

* acl: **(Consul Enterprise only)** - Disable the ACL.Bootstrap RPC endpoints when managed service provider tokens are in use. [[GH-7614](https://github.com/hashicorp/consul/pull/7614)]
* acl: **(Consul Enterprise only)** - Consul agents will now use the first managed service provider token for the agents token when any are present.
* acl: Added a v1/acl/policy/name/:name HTTP endpoint to read a policy by name. [[GH-6615](https://github.com/hashicorp/consul/pull/6615)]
* acl: Added JSON format output to all of the ACL CLI commands. [[GH-7141](https://github.com/hashicorp/consul/issues/7141)]
* agent/xds: Update mesh gateway to use the service resolver connect timeout when configured [[GH-6370](https://github.com/hashicorp/consul/issues/6370)]
* cli: Log "newer version available" message at `INFO` level [[GH-7457](https://github.com/hashicorp/consul/issues/7457)]
* config: Consul Enterprise specific configuration are now parseable in OSS but will emit warnings about them not being used. [[GH-7714](https://github.com/hashicorp/consul/pull/7714)
* network areas: **(Consul Enterprise only)** - Network areas are using memberlist with TCP and for every message a new connection was established. Now the connections multiplexed with yamux, which means that way fewer connections are created.
* network segments: **(Consul Enterprise only)** - The segment configuration is no longer stored in serf node tags. There is now an RPC endpoint for the same information, which means that the number of network segment is no longer limited by node meta tag size.
* snapshot agent: **(Consul Enterprise only)** - Azure has different environments, of which it was only possible to use the public one so far. A new flag was added so that every other environment can be used as well, like Azure China.


BUGFIXES:

* agent: don't let left nodes hold onto their node-id [[GH-7775](https://github.com/hashicorp/consul/pull/7775)]
* agent: **(Consul Enterprise only)** Fixed several bugs related to Network Area ann Network Segment compatibility with other features caused by incorrectly doing version or serf tag checking. [[GH-7491](https://github.com/hashicorp/consul/pull/7491)]
* cli: ensure that 'snapshot save' is fsync safe and also only writes to the requested file on success [[GH-7698](https://github.com/hashicorp/consul/issues/7698)]
* cli: fix usage of gzip.Reader to better detect corrupt snapshots during save/restore [[GH-7697](https://github.com/hashicorp/consul/issues/7697)]
* connect: Fix panic when validating a service-router config entry with no destination [[GH-7783](https://github.com/hashicorp/consul/pull/7783)]
* namespace: **(Consul Enterprise only)** Fixed several bugs where results from multiple namespaces would be returned when only a single namespace was being queried when the token making the request had permissions to see all of them.
* snapshot agent **(Consul Enterprise only)**: Ensure snapshots persisted with the local backend are fsync safe and also only writes to the requested file on success.
* snapshot agent **(Consul Enterprise only)**: Verify integrity of snapshots locally before storing with the configured backend.
* ui: Ensure blocking queries are used in the service instance page instead of polling [[GH-7543](https://github.com/hashicorp/consul/pull/7543)]
* ui: Fix a refreshing/rescrolling issue for the healthcheck listings [[GH-7550](https://github.com/hashicorp/consul/pull/7550)] [[GH-7365](https://github.com/hashicorp/consul/issues/7365)]
* ui: Fix token duplication action bug [[GH-7552](https://github.com/hashicorp/consul/pull/7552)]
* ui: Lazily detect HTTP protocol along with a fallback for non-detection [[GH-7644](https://github.com/hashicorp/consul/pull/7644)] [[GH-7643](https://github.com/hashicorp/consul/issues/7643)]
* ui: Ensure KV names using 'special' terms within the default namespace are editable when the URL doesn't include the default namespace  [[GH-7734](https://github.com/hashicorp/consul/pull/7734)]
* xds: Fix flapping of mesh gateway connect-service watches [[GH-7575](https://github.com/hashicorp/consul/pull/7575)]

## 1.7.2 (March 16, 2020)

IMPROVEMENTS:

* agent: add option to configure max request length for `/v1/txn` endpoint [[GH-7388](https://github.com/hashicorp/consul/pull/7388)]
* build: bump the expected go language version of the main module to 1.13 [[GH-7429](https://github.com/hashicorp/consul/issues/7429)]
* agent: add http_config.response header to the UI headers [[GH-7369](https://github.com/hashicorp/consul/pull/7369)]
* agent: Added documentation and error messages related to `kv_max_value_size` option [[GH-7405]](https://github.com/hashicorp/consul/pull/7405)]
* agent: Take Prometheus MIME-type header into account [[GH-7371]](https://github.com/hashicorp/consul/pull/7371)]

BUGFIXES:

* acl: Updated token resolution so managed service provider token applies to all endpoints. [[GH-7431](https://github.com/hashicorp/consul/pull/7431)]
* agent: Fixed error output when agent crashes early [[GH-7411](https://github.com/hashicorp/consul/pull/7411)]
* agent: Handle bars in node names when displaying lists in CLI like `consul members` [[GH-6652]](https://github.com/hashicorp/consul/pull/6652)]
* agent: Avoid discarding health check status on `consul reload` [[GH-7345]](https://github.com/hashicorp/consul/pull/7345)]
* network areas: **(Consul Enterprise only)** - Fixed compatibility issues with network areas and v1.4.0+ ACLs as well as network areas and namespaces. The issue was that secondary datacenters connected to the primary via a network area were not properly detecting that the primary DC supported those other features.
* sessions: Fixed backwards incompatibility with 1.6.x and earlier [[GH-7395](https://github.com/hashicorp/consul/issues/7395)][[GH-7399](https://github.com/hashicorp/consul/pull/7399)]
* sessions: Fixed backwards incompatibility with 1.6.x and earlier [[GH-7395](https://github.com/hashicorp/consul/issues/7395)][[GH-7398](https://github.com/hashicorp/consul/pull/7398)]
* ui: Fixed a DOM refreshing bug on the node detail page which forced an scroll reset [[GH-7365](https://github.com/hashicorp/consul/issues/7365)][[GH-7377](https://github.com/hashicorp/consul/pull/7377)]
* ui: Fix blocking query requests for the coordinates API requests [[GH-7378](https://github.com/hashicorp/consul/pull/7378)]
* ui: Enable recovery from an unreachable datacenter [[GH-7404](https://github.com/hashicorp/consul/pull/7404)]

## 1.7.1 (February 20, 2020)

IMPROVEMENTS:

* agent: sensible keyring error [[GH-7272](https://github.com/hashicorp/consul/pull/7272)]
* agent: add server `raft.{last,applied}_index` gauges [[GH-6694](https://github.com/hashicorp/consul/pull/6694)]
* build: Switched to compile with Go 1.13.7 [[GH-7262](https://github.com/hashicorp/consul/issues/7262)]
* config: increase `http_max_conns_per_client` default to 200 [[GH-7289](https://github.com/hashicorp/consul/pull/7289)]
* tls: support TLS 1.3 [[GH-7325](https://github.com/hashicorp/consul/pull/7325)]

BUGFIXES:

* acl: **(Consul Enterprise only)** Fixed an issue that prevented remote policy and role resolution from working when namespace policy or role defaults were configured.
* dns: Fixed an issue that could cause the DNS server to consume excessive CPU resources when trying to parse IPv6 recursor addresses: [[GH-6120](https://github.com/hashicorp/consul/issues/6120)]
* dns: Fixed an issue that caused Consul to setup a root zone handler when no `alt_domain` was configured. [[GH-7323](https://github.com/hashicorp/consul/pull/7323)]
* sessions: Fixed an issue that was causing deletions of a non-existent session to return a 500 when ACLs were enabled. [[GH-6840](https://github.com/hashicorp/consul/issues/6840)]
* xds: Fix envoy retryOn behavior when multiple behaviors are configured [[GH-7280](https://github.com/hashicorp/consul/pull/7280)]
* xds: Mesh Gateway fixes to prevent configuring extra clusters and for properly handling a service-resolvers default subset. [[GH-7294](https://github.com/hashicorp/consul/pull/7294)]
* ui: Gracefully cope with errors in discovery-chain when connect is disabled [[GH-7291](https://github.com/hashicorp/consul/pull/7291)]

## 1.7.0 (February 11, 2020)

NOTES:

* cli: Our Windows 32-bit and 64-bit executables for this version and up will be signed with a HashiCorp certificate. Windows users will no longer see a warning about an "unknown publisher" when running our software.

* cli: Our darwin releases for this version and up will be signed and notarized according to Apple's requirements.

Prior to this release, MacOS 10.15+ users attempting to run our software may see the error: "'consul' cannot be opened because the developer cannot be verified." This error affected all MacOS 10.15+ users who downloaded our software directly via web browsers, and was caused by changes to [Apple's third-party software requirements](https://developer.apple.com/news/?id=09032019a).

MacOS 10.15+ users should plan to upgrade to 1.7.0+.

SECURITY:

* dns: Updated miekg/dns dependency to fix a memory leak and CVE-2019-19794. [[GH-6984](https://github.com/hashicorp/consul/issues/6984)], [[GH-7252](https://github.com/hashicorp/consul/issues/7252)]
* updated to compile with [[Go 1.12.16](https://groups.google.com/forum/m/#!topic/golang-announce/Hsw4mHYc470)] which includes a fix for CVE-2020-0601 on windows [[GH-7153](https://github.com/hashicorp/consul/pull/7153)]

BREAKING CHANGES:

* http: The HTTP API no longer accepts JSON fields that are unknown to it. Instead errors will be returned with 400 status codes [[GH-6874](https://github.com/hashicorp/consul/pull/6874)]
* dns: PTR record queries now return answers that contain the Consul datacenter as a label between `service` and the domain. [[GH-6909](https://github.com/hashicorp/consul/pull/6909)]
* agent: The ACL requirement for the [agent/force-leave endpoint](https://www.consul.io/api/agent.html#force-leave-and-shutdown) is now `operator:write` rather than `agent:write`. [[GH-7033](https://github.com/hashicorp/consul/pull/7033)]
* logging: Switch over to using go-hclog and allow emitting either structured or unstructured logs. This changes the log format quite a bit and could break any log parsing users may have in place. [[GH-1249](https://github.com/hashicorp/consul/issues/1249)][[GH-7130](https://github.com/hashicorp/consul/pull/7130)]
* intentions: Change the ACL requirement and enforcement for wildcard rules. Previously this would look for an ACL rule that would grant access to the service/intention `*`. Now, in order to write a wildcard intention requires write access to all intentions and reading a wildcard intention requires read access to any intention that would match. Additionally intention listing and reading allow access if the requester can read either side of the intention whereas before it only allowed it for permissions on the destination side. [[GH-7028](https://github.com/hashicorp/consul/pull/7028)]
* telemetry: `consul.rpc.query` has changed to only measure the _start_ of `srv.blockingQuery()` calls. In certain rare cases where there are lots of idempotent updates this will cause the metric to report lower than before. The counter should now provides more meaningful behavior that maps to the rate of client-initiated requests. [[GH-7224](https://github.com/hashicorp/consul/pull/7224)]

FEATURES:

* **Namespaces (Consul Enterprise only)** This version adds namespacing to Consul. Namespaces help reduce operational challenges by removing restrictions around uniqueness of resource names across distinct teams, and enable operators to provide self-service through delegation of administrative privileges. Namespace support was added to:
  * ACLs
  * Key/Value Store
  * Sessions
  * Catalog
  * Connect
  * UI [[GH6639](https://github.com/hashicorp/consul/pull/6639)]
* agent: Add Cloud Auto-join support for Tencent Cloud [[GH-6818](https://github.com/hashicorp/consul/pull/6818)]
* connect: Added a new CA provider allowing Connect certificates to be managed by AWS [ACM Private CA](https://www.consul.io/docs/connect/ca/aws.html).
* connect: Allow configuration of upstream connection limits in Envoy [[GH-6829](https://github.com/hashicorp/consul/pull/6829)]
* ui: Adds UI support for [Exposed Checks](https://github.com/hashicorp/consul/pull/6446) [[GH6575]](https://github.com/hashicorp/consul/pull/6575)
* ui: Visualisation of the Discovery Chain  [[GH6746]](https://github.com/hashicorp/consul/pull/6746)

IMPROVEMENTS:

* acl: Use constant time comparison when checking for the ACL agent master token. [[GH-6943](https://github.com/hashicorp/consul/pull/6943)]
* acl: Add accessorID of token when ops are denied by ACL system [[GH-7117](https://github.com/hashicorp/consul/pull/7117)]
* agent: default the primary_datacenter to the datacenter if not configured [[GH-7111](https://github.com/hashicorp/consul/issues/7111)]
* agent: configurable `MaxQueryTime` and `DefaultQueryTime` [[GH-3777](https://github.com/hashicorp/consul/pull/3777)]
* agent: do not deregister service checks twice [[GH-6168](https://github.com/hashicorp/consul/pull/6168)]
* agent: remove service sidecars in `cleanupRegistration` [[GH-7022](https://github.com/hashicorp/consul/pull/7022)]
* agent: setup grpc server with auto_encrypt certs and add `-https-port` [[GH-7086](https://github.com/hashicorp/consul/pull/7086)
* agent: some check types now support configuring a number of consecutive failure and success before the check status is updated in the catalog. [[GH-5739](https://github.com/hashicorp/consul/pull/5739)]
* agent: clients should only attempt to remove pruned nodes once per call [[GH-6591](https://github.com/hashicorp/consul/pull/6591)]
* agent: Consul HTTP checks can now send a configurable `body` in the request. [[GH-6602](https://github.com/hashicorp/consul/pull/6602)]
* agent: increase watchLimit to 8192. [[GH-7200](https://github.com/hashicorp/consul/pull/7200)]
* api: A new `/v1/catalog/node-services/:node` endpoint was added that mirrors the existing `/v1/catalog/node/:node` endpoint but has a response structure that contains a slice of services instead of a map of service ids to services. This new endpoint allows retrieving all services in all namespaces for a node. [[GH-7115](https://github.com/hashicorp/consul/pull/7115)]
* api: add option to set TLS options in-memory for API client [[GH-7093](https://github.com/hashicorp/consul/pull/7093)]
* api: add replace-existing-checks param to the api package [[GH-7136](https://github.com/hashicorp/consul/pull/7136)]
* auto_encrypt: set dns and ip san for k8s and provide configuration [[GH-6944](https://github.com/hashicorp/consul/pull/6944)]
* cli: improve the file safety of 'consul tls' subcommands [[GH-7186](https://github.com/hashicorp/consul/issues/7186)]
* cli: give feedback to CLI user on forceleave command if node does not exist [[GH-6841](https://github.com/hashicorp/consul/pull/6841)]
* connect: Envoy's whole stats endpoint can now be exposed to allow integrations like DataDog agent [[GH-7070](https://github.com/hashicorp/consul/pull/7070)]
* connect: check if intermediate cert needs to be renewed. [[GH-6835](https://github.com/hashicorp/consul/pull/6835)]
* connect: Allow inlining of the TLS certificate in the Envoy configuration. [[GH-6360](https://github.com/hashicorp/consul/issues/6360)]
* dns: Improvement to enable dual stack IPv4/IPv6 addressing of services and lookup via DNS [[GH-6531](https://github.com/hashicorp/consul/issues/6531)]
* lock: `consul lock` will now receive shutdown signals during the lock-acquisition process. [[GH-5909](https://github.com/hashicorp/consul/pull/5909)]
* raft: increase raft notify buffer [[GH-6863](https://github.com/hashicorp/consul/pull/6863)]
* raft: update raft to v1.1.2 [[GH-7079](https://github.com/hashicorp/consul/pull/7079)]
* router: do not surface left servers [[GH-6420](https://github.com/hashicorp/consul/pull/6420)]
* rpc: log method when a server/server RPC call fails [[GH-4548](https://github.com/hashicorp/consul/pull/4548)]
* sentinel: **(Consul Enterprise only)** The Sentinel framework was upgraded to v0.13.0. See the [Sentinel Release Notes](https://docs.hashicorp.com/sentinel/changelog/) for more information.
* telemetry: Added `consul.rpc.queries_blocking` gauge to measure the current number of in-flight blocking queries. [[GH-7224](https://github.com/hashicorp/consul/pull/7224)]
* ui: Discovery chain improvements for clarifying the default router [[GH-7222](https://github.com/hashicorp/consul/pull/7222)]
* ui: Added unique browser titles to each page [[GH-7118](https://github.com/hashicorp/consul/pull/7118)]
* ui: Add live updates/blocking queries to the Intention listing page [[GH-7161](https://github.com/hashicorp/consul/pull/7161)]
* ui: Use more consistent icons with other HashiCorp products in the UI [[GH-6851]](https://github.com/hashicorp/consul/pull/6851)
* ui: Improvements to the Discovery Chain visualisation in respect to redirects [[GH-7036]](https://github.com/hashicorp/consul/pull/7036)
* ui: Improvement keyboard navigation of the main menu [[GH-7090]](https://github.com/hashicorp/consul/pull/7090)
* ui: New row confirmation dialogs [[GH-7007]](https://github.com/hashicorp/consul/pull/7007)
* ui: Various visual CSS amends and alterations [[GH6495]](https://github.com/hashicorp/consul/pull/6495) [[GH6881]](https://github.com/hashicorp/consul/
* ui: Hides the Routing tab for a service proxy [[GH-7195](https://github.com/hashicorp/consul/pull/7195)]
* ui: Add ability to search nodes listing page with IP Address [[GH-7204](https://github.com/hashicorp/consul/pull/7204)]
* xds: mesh gateway CDS requests are now allowed to receive an empty CDS reply [[GH-6787](https://github.com/hashicorp/consul/issues/6787)]
* xds: Verified integration test suite with Envoy 1.12.2 & 1.13.0 [[GH-6947](https://github.com/hashicorp/consul/pull/7240)]
* agent: Added ACL token for Consul managed service providers [[GH-7218](https://github.com/hashicorp/consul/pull/7218)]

BUGFIXES:

* agent: fix watch event behavior [[GH-5265](https://github.com/hashicorp/consul/pull/5265)]
* agent: ensure node info sync and full sync [[GH-7189](https://github.com/hashicorp/consul/pull/7189)]
* autopilot: Fixed dead server removal condition to use correct failure tolerance. [[GH-4017](https://github.com/hashicorp/consul/pull/4017)]
* cli: services register command now correctly registers an unamed healthcheck [[GH-6800](https://github.com/hashicorp/consul/pull/6800)]
* cli: remove `-dev` from `consul version` in ARM builds in the 1.6.2 release [[GH-6875](https://github.com/hashicorp/consul/issues/6875)]
* cli: ui_content_path config option fix [[GH-6601](https://github.com/hashicorp/consul/pull/6601)]
* config: Fixed a bug that caused some config parsing to be case-sensitive: [[GH-7191](https://github.com/hashicorp/consul/pull/7191)]
* connect: CAs can now use RSA keys correctly to sign EC leafs [[GH-6638](https://github.com/hashicorp/consul/pull/6638)]
* connect: derive connect certificate serial numbers from a memdb index instead of the provider table max index [[GH-7011](https://github.com/hashicorp/consul/pull/7011)]
* connect: ensure that updates to the secondary root CA configuration use the correct signing key ID values for comparison [[GH-7012](https://github.com/hashicorp/consul/pull/7012)]
* connect: use correct subject key id for leaf certificates. [[GH-7091](https://github.com/hashicorp/consul/pull/7091)]
* log: handle discard all logfiles properly [[GH-6945](https://github.com/hashicorp/consul/pull/6945)]
* state: restore a few more service-kind index updates so blocking in ServiceDump works in more cases [[GH-6948](https://github.com/hashicorp/consul/issues/6948)]
* tls: fix behavior related to auto_encrypt and verify_incoming (#6899) [[GH-6811](https://github.com/hashicorp/consul/pull/6811)]
* ui: Ensure the main navigation menu is closed on click [[GH-7164](https://github.com/hashicorp/consul/pull/7164)]
* ui: Ensure KV flags are passed through to Consul on update [[GH-7216](https://github.com/hashicorp/consul/pull/7216)]
* ui: Fix positioning of active icon in main navigation menu [[GH-7233](https://github.com/hashicorp/consul/pull/7233)]
* ui: Ensure the Namespace property is sent to Consul in OSS [[GH-7238](https://github.com/hashicorp/consul/pull/7238)]
* ui: Remove the Policy/Service Identity selector from namespace policy form [[GH-7124](https://github.com/hashicorp/consul/pull/7124)]
* ui: Fix positioning of active icon in the selected menu item [[GH-7148](https://github.com/hashicorp/consul/pull/7148)]
* ui: Discovery-Chain: Improve parsing of redirects [[GH-7174](https://github.com/hashicorp/consul/pull/7174)]
* ui: Fix styling of ‘duplicate intention’ error message [[GH6936]](https://github.com/hashicorp/consul/pull/6936)

## 1.6.10 (November 19, 2020)

SECURITY:

* Increase the permissions to read from the `/connect/ca/configuration` endpoint to `operator:write`. Previously Connect CA configuration, including the private key, set via this endpoint could be read back by an operator with `operator:read` privileges. [CVE-2020-28053](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-28053) [[GH-9240](https://github.com/hashicorp/consul/issues/9240)]

## 1.6.9 (September 11, 2020)

BUG FIXES:

* api: fixed a panic caused by an api request with Connect=null [[GH-8537](https://github.com/hashicorp/consul/pull/8537)]

## 1.6.8 (August 12, 2020)

BUG FIXES:

* vendor: update github.com/armon/go-metrics to v0.3.4 to mitigate a potential panic when emitting Prometheus metrics at an interval longer than the metric expiry time [[GH-8478](https://github.com/hashicorp/consul/pull/8478)]

## 1.6.7 (July 30, 2020)

BUG FIXES:

* agent: Fixed an issue with lock contention during RPCs when under load while using the Prometheus metrics sink. [[GH-8372](https://github.com/hashicorp/consul/pull/8372)]
* gossip: Avoid issue where two unique leave events for the same node could lead to infinite rebroadcast storms [[GH-8345](https://github.com/hashicorp/consul/issues/8345)]

## 1.6.6 (June 10, 2020)

SECURITY:

* Adding an option `http_config.use_cache` to disable agent caching for http endpoints, because Consul’s DNS and HTTP API expose a caching feature susceptible to DoS. [CVE-2020-13250](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-13250) [[GH-8023]](https://github.com/hashicorp/consul/pull/8023)
* Propagate and enforce changes to legacy ACL tokens rules in secondary data centers. [CVE-2020-12797](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-12797) [[GH-8047]](https://github.com/hashicorp/consul/pull/8047)
* Only resolve local acl token in the datacenter it belongs to. [CVE-2020-13170](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-13170) [[GH-8068]](https://github.com/hashicorp/consul/pull/8068)

BUG FIXES:

* acl: Fixed an issue where legacy management tokens could not be used in secondary datacenters. [[GH-7908](https://github.com/hashicorp/consul/pull/7908)]
* agent: Fixed a race condition that could cause an agent to crash when first starting. [[GH-7955](https://github.com/hashicorp/consul/issues/7955)]

## 1.6.5 (April 14, 2020)

BUG FIXES:

* agent: **(Consul Enterprise only)** Fixed several bugs related to Network Area and Network Segment compatibility with other features caused by incorrectly doing version or serf tag checking. [[GH-7551](https://github.com/hashicorp/consul/pull/7551)]

## 1.6.4 (February 20, 2020)

SECURITY:

* dns: Updated miekg/dns dependency to fix a memory leak and CVE-2019-19794. [[GH-6984](https://github.com/hashicorp/consul/issues/6984)], [[GH-7261](https://github.com/hashicorp/consul/pull/7261)]

## 1.6.3 (January 30, 2020)

SECURITY

* agent: mitigate potential DoS vector allowing unbounded server resource usage from unauthenticated connections [[GH-7159](https://github.com/hashicorp/consul/issues/7159)]
* acl: add ACL enforcement to the `v1/agent/health/service/*` endpoints [[GH-7160](https://github.com/hashicorp/consul/issues/7160)]

IMPROVEMENTS

* tls: `auto_encrypt` and `verify_incoming` [[GH-6811](https://github.com/hashicorp/consul/pull/6811)]

BUG FIXES

* agent: output proper HTTP status codes for Txn requests that are too large [[GH-7158](https://github.com/hashicorp/consul/pull/7158)]
* connect: derive connect certificate serial numbers from a memdb index instead of the provider table max index [[GH-7011](https://github.com/hashicorp/consul/pull/7011)]
* connect: ensure that updates to the secondary root CA configuration use the correct signing key ID values for comparison [[GH-7012](https://github.com/hashicorp/consul/pull/7012)]

## 1.6.2 (November 13, 2019)

SECURITY

* Updated to compile with Go 1.12.13 which includes a fix for CVE-2019-17596 in [[Go 1.12.11](https://groups.google.com/forum/#!msg/golang-announce/lVEm7llp0w0/VbafyRkgCgAJ)] [[GH-6319](https://github.com/hashicorp/consul/pull/6759)]

FEATURES

* agent: store check type in catalog [[GH-6561](https://github.com/hashicorp/consul/pull/6561)]
* agent: update force-leave to allow for complete removal of members [[GH-6571](https://github.com/hashicorp/consul/issues/6571)]
* agent: updates to the agent token trigger anti-entropy full syncs [[GH-6577](https://github.com/hashicorp/consul/issues/6577)]
* snapshot agent (Consul Enterprise): Added support for saving snapshots to Google Cloud Storage.
* connect: Added proxy config stanza to allow exposing HTTP paths through Envoy for non-Connect-enabled services [[GH-5396](https://github.com/hashicorp/consul/issues/5396)]

IMPROVEMENTS

* licensing (Consul Enterprise): Increase initial server temporary license duration to 6 hours to allow for longer upgrades/migrations.
* server: ensure the primary datacenter and ACL datacenter match [[GH-6634](https://github.com/hashicorp/consul/issues/6634)]
* sdk: ignore panics due to stray goroutines logging after a test completes [[GH-6632](https://github.com/hashicorp/consul/issues/6632)]
* agent: allow mesh gateways to initialize even if there are no connect services registered yet [[GH-6576](https://github.com/hashicorp/consul/issues/6576)]
* agent: endpoint performance improvements, Txn endpoint in particular. [[GH-6680](https://github.com/hashicorp/consul/pull/6680)]
* sdk: add NewTestServerT, deprecate NewTestServer in testutil to prevent nil point dereference [[GH-6761](https://github.com/hashicorp/consul/pull/6761)]
* agent: auto_encrypt provided TLS certificates can now be used to enable HTTPS on clients [[GH-6489](https://github.com/hashicorp/consul/pull/6489)]
* sentinel (Consul Enterprise): update to v0.13.0, see [Sentinel changelog](https://docs.hashicorp.com/sentinel/changelog/) for more details

BUG FIXES

* ARM release binaries: Starting with v1.6.2, Consul will ship three separate versions of ARM builds. The previous ARM binaries of Consul could potentially crash due to the way the Go runtime manages internal pointers to its Go routine management constructs and how it keeps track of them especially during signal handling. From v1.6.2 forward, it is recommended to use:
  * consul_{version}_linux_armelv5.zip for all 32-bit armel systems
  * consul_{version}_linux_armhfv6.zip for all armhf systems with v6+ architecture
  * consul_{version}_linux_arm64.zip for all v8 64-bit architectures
* agent: Parse the HTTP Authorization header as case-insensitive. [[GH-6568](https://github.com/hashicorp/consul/issues/6568)]
* agent: minimum quorum check added to Autopilot with minQuorum option [[GH-6654](https://github.com/hashicorp/consul/issues/6654)]
* agent: cache notifications work after error if the underlying RPC returns index=1 [[GH-6547](https://github.com/hashicorp/consul/issues/6547)]
* agent: tolerate more failure scenarios during service registration with central config enabled [[GH-6472](https://github.com/hashicorp/consul/issues/6472)]
* cache: remove data race in agent cache [[GH-6470](https://github.com/hashicorp/consul/issues/6470)]
* connect: connect CA Roots in secondary datacenters should use a SigningKeyID derived from their local intermediate [[GH-6513](https://github.com/hashicorp/consul/issues/6513)]
* connect: don't colon-hex-encode the AuthorityKeyId and SubjectKeyId fields in connect certs [[GH-6492](https://github.com/hashicorp/consul/issues/6492)]
* connect: intermediate CA certs generated with the vault provider lack URI SANs [[GH-6491](https://github.com/hashicorp/consul/issues/6491)]
* debug: Fix a bug in sync.WaitGroup usage. [[GH-6649](https://github.com/hashicorp/consul/pull/6649)]
* xds: tcp services using the discovery chain should not assume RDS during LDS [[GH-6623](https://github.com/hashicorp/consul/issues/6623)]
* ui: Fix a bug where switching datacenters using the datacenter menu would lead to an empty service listing [[GH-6555](https://github.com/hashicorp/consul/pull/6555)]

## 1.6.1 (September 12, 2019)

IMPROVEMENTS

* agent: Distinguish between DC not existing and not being available [[GH-6399](https://github.com/hashicorp/consul/pull/6399)]
* agent: Added `replace-existing-checks` param to service registration endpoint to replace existing checks when re-registering a service. [[GH-4905](https://github.com/hashicorp/consul/pull/4905)]
* auto_encrypt: verify_incoming_rpc is good enough for auto_encrypt.allow_tls [[GH-6376](https://github.com/hashicorp/consul/pull/6376)]
* connect: Ensure that a secondary CA's intermediate certificate will show in the various API endpoints CA Roots output [[GH-6333](https://github.com/hashicorp/consul/pull/6333)]
* ui: Reconcile ember-data store [[GH-5745](https://github.com/hashicorp/consul/pull/5745)]
* ui: Allow text selection of clickable elements and their contents without then jumping the user to the linked page [[GH-5770](https://github.com/hashicorp/consul/pull/5770)]
* ui: Adds the ability to frontend search instances by address (ip:port) [[GH-5993](https://github.com/hashicorp/consul/pull/5993)]
* ui: Add CheckID to the output panels of healthchecks  [[GH-6195](https://github.com/hashicorp/consul/pull/6195)]
* ui: Enable blocking queries by default [[GH-6194](https://github.com/hashicorp/consul/pull/6194)]
* txn: don't try to decode request bodies > raft.SuggestedMaxDataSize [[GH-6422](https://github.com/hashicorp/consul/pull/6422)]

BUG FIXES

* network areas (Consul Enterprise): Ensure that TCP based transport for network area memberlist propgates failed nodes properly [[GH-6479](https://github.com/hashicorp/consul/pull/6479)]
* network areas (Consul Enterprise): make sure network areas are left as well when consul is leaving [[GH-6453](https://github.com/hashicorp/consul/pull/6453)]
* ui: Show the correct message when a session has been removed from a KV [[GH-6167](https://github.com/hashicorp/consul/pull/6167)]
* ui: Ensure KV sessions visually aren't shared between multiple KV's [[GH-6166](https://github.com/hashicorp/consul/pull/6166)]
* tls: make sure auto_encrypt has private key type and bits [[GH-6392](https://github.com/hashicorp/consul/pull/6392)]


FEATURES

* ui: Add leader icon for node listing view to call out which node is the current leader [[GH-6265](https://github.com/hashicorp/consul/pull/6265)]

## 1.6.0 (August 23, 2019)

SECURITY:

* Updated to compile with Go 1.12.8 which mitigates CVE-2019-9512 and CVE-2019-9514 for the builtin HTTP server [[GH-6319](https://github.com/hashicorp/consul/pull/6319)]
* Updated the google.golang.org/grpc dependency to v1.23.0 to mitigate CVE-2019-9512, CVE-2019-9514, and CVE-2019-9515 for the gRPC server. [[GH-6320](https://github.com/hashicorp/consul/pull/6320)]

BREAKING CHANGES:

* connect: remove deprecated managed proxies and ProxyDestination config [[GH-6220](https://github.com/hashicorp/consul/pull/6220)]

FEATURES:

* **Connect Envoy Supports L7 Routing:** Additional configuration entry types `service-router`, `service-resolver`, and `service-splitter`, allow for configuring Envoy sidecars to enable reliability and deployment patterns at L7 such as HTTP path-based routing, traffic shifting, and advanced failover capabilities. For more information see the [L7 traffic management](https://www.consul.io/docs/connect/l7-traffic-management.html) docs.
* **Mesh Gateways:** Envoy can now be run as a gateway to route Connect traffic across datacenters using SNI headers, allowing connectivty across platforms and clouds and other complex network topologies. Read more in the [mesh gateway docs](https://www.consul.io/docs/connect/mesh_gateway.html).
* **Intention & CA Replication:** In order to enable connecitivty for services across datacenters, Connect intentions are now replicated and the Connect CA cross-signs from the [primary_datacenter](/docs/agent/options.html#primary_datacenter). This feature was previously part of Consul Enterprise.
* agent: add `local-only` parameter to operator/keyring list requests to force queries to only hit local servers. [[GH-6279](https://github.com/hashicorp/consul/pull/6279)]
* connect: expose an API endpoint to compile the discovery chain [[GH-6248](https://github.com/hashicorp/consul/issues/6248)]
* connect: generate the full SNI names for discovery targets in the compiler rather than in the xds package [[GH-6340](https://github.com/hashicorp/consul/issues/6340)]
* connect: introduce ExternalSNI field on service-defaults [[GH-6324](https://github.com/hashicorp/consul/issues/6324)]
* xds: allow http match criteria to be applied to routes on services using grpc protocols [[GH-6149](https://github.com/hashicorp/consul/issues/6149)]

IMPROVEMENTS:

* agent: Added tagged addressing to services similar to the already present Node tagged addressing [[GH-5965](https://github.com/hashicorp/consul/pull/5965)]
* agent: health checks: change long timeout behavior to use to user-configured `timeout` value [[GH-6094](https://github.com/hashicorp/consul/pull/6094)]
* api: Display allowed HTTP CIDR information nicely [[GH-6029](https://github.com/hashicorp/consul/pull/6029)]
* api: Update filtering language to include substring and regular expression matching on string values [[GH-6190](https://github.com/hashicorp/consul/pull/6190)]
* connect: added a new `-bind-address` cli option for envoy to create a mapping of the desired bind addresses to use instead of the default rules or tagged addresses [[GH-6107](https://github.com/hashicorp/consul/pull/6107)]
* connect: allow L7 routers to match on http methods [[GH-6164](https://github.com/hashicorp/consul/issues/6164)]
* connect: change router syntax for matching query parameters to resemble the syntax for matching paths and headers for consistency. [[GH-6163](https://github.com/hashicorp/consul/issues/6163)]
* connect: detect and prevent circular discovery chain references [[GH-6246](https://github.com/hashicorp/consul/issues/6246)]
* connect: ensure time.Duration fields retain their human readable forms in the API [[GH-6348](https://github.com/hashicorp/consul/issues/6348)]
* connect: reconcile how upstream configuration works with discovery chains [[GH-6225](https://github.com/hashicorp/consul/issues/6225)]
* connect: rework how the service resolver subset OnlyPassing flag works [[GH-6173](https://github.com/hashicorp/consul/issues/6173)]
* connect: simplify the compiled discovery chain data structures [[GH-6242](https://github.com/hashicorp/consul/issues/6242)]
* connect: validate and test more of the L7 config entries [[GH-6156](https://github.com/hashicorp/consul/issues/6156)]
* gossip: increase size of gossip key generated by keygen to 32 bytes and document support for AES 256 [[GH-6244](https://github.com/hashicorp/consul/issues/6244)]
* license (enterprise): Added license endpoint support to the API client [[GH-6268](https://github.com/hashicorp/consul/pull/6268)]
* xds: improve how envoy metrics are emitted [[GH-6312](https://github.com/hashicorp/consul/issues/6312)]
* xds: Verified integration test suite with Envoy 1.11.1 [[GH-6347](https://github.com/hashicorp/consul/pull/6347)]

BUG FIXES:

* acl: Fixed a bug that could prevent transition from legacy ACL mode to new ACL mode [[GH-6332](https://github.com/hashicorp/consul/pull/6332)
* agent: blocking central config RPCs iterations should not interfere with each other [[GH-6316](https://github.com/hashicorp/consul/issues/6316)]
* agent: fix an issue that could cause a panic while transferring leadership due to replication [[GH-6104](https://github.com/hashicorp/consul/issues/6104)]
* api: Fix a bug where the service tagged addresses were not being returned through the `v1/agent/service/:service` api. [[GH-6299](https://github.com/hashicorp/consul/issues/6299)]
* api: un-deprecate api.DecodeConfigEntry [[GH-6278](https://github.com/hashicorp/consul/issues/6278)]
* auto_encrypt: use server-port [[GH-6287](https://github.com/hashicorp/consul/pull/6287)]
* autopilot: update to also remove failed nodes from WAN gossip pool [[GH-6028](https://github.com/hashicorp/consul/pull/6028)]
* cli: ensure that the json form of config entries can be submitted with 'consul config write' [[GH-6290](https://github.com/hashicorp/consul/issues/6290)]
* cli: Fixed bindable IP detection with the `connect envoy` command. [[GH-6238](https://github.com/hashicorp/consul/pull/6238)]
* config: Ensure that all config entry writes are transparently forwarded to the primary datacneter. [[GH-6327](https://github.com/hashicorp/consul/issues/6327)]
* connect: allow 'envoy_cluster_json' escape hatch to continue to function [[GH-6378](https://github.com/hashicorp/consul/issues/6378)]
* connect: allow mesh gateways to use central config [[GH-6302](https://github.com/hashicorp/consul/issues/6302)]
* connect: ensure intention replication continues to work when the replication ACL token changes [[GH-6288](https://github.com/hashicorp/consul/issues/6288)]
* connect: ensure local dc connections do not use the gateway [[GH-6085](https://github.com/hashicorp/consul/issues/6085)]
* connect: fix bug in service-resolver redirects if the destination uses a default resolver [[GH-6122](https://github.com/hashicorp/consul/pull/6122)]
* connect: Fixed a bug that would prevent CA replication/initializing in a secondary DC from working when ACLs were enabled. [[GH-6192](https://github.com/hashicorp/consul/issues/6192)]
* connect : Fixed a regression that broken xds endpoint generation for prepared query upstreams. [[GH-6236](https://github.com/hashicorp/consul/pull/6236)]
* connect: fix failover through a mesh gateway to a remote datacenter [[GH-6259](https://github.com/hashicorp/consul/issues/6259)]
* connect: resolve issue where `MeshGatewayConfig` could be returned empty [[GH-6093](https://github.com/hashicorp/consul/pull/6093)]
* connect: updating a service-defaults config entry should leave an unset protocol alone [[GH-6342](https://github.com/hashicorp/consul/issues/6342)]
* connect: validate upstreams and prevent duplicates [[GH-6224](https://github.com/hashicorp/consul/issues/6224)]
* server: if inserting bootstrap config entries fails don't silence the errors [[GH-6256](https://github.com/hashicorp/consul/issues/6256)]
* snapshot: fix TCP half-close implementation for TLS connections [[GH-6216](https://github.com/hashicorp/consul/pull/6216)]

KNOWN ISSUES

* auto_encrypt: clients with auto_encrypt enabled won't be able to start because of [[GH-6391](https://github.com/hashicorp/consul/issues/6391)]. There is a fix, but it came too late and we couldn't include it in the release. It will be part of 1.6.1 and we recommend that if you are using auto_encrypt you postpone the update.

## 1.5.3 (July 25, 2019)

IMPROVEMENTS:
* raft: allow trailing logs to be configured as an escape hatch for extreme load that prevents followers catching up with leader [[GH-6186](https://github.com/hashicorp/consul/pull/6186)]
* raft: added raft log chunking capabilities to allow for storing larger KV entries [[GH-6172](https://github.com/hashicorp/consul/pull/6172)]
* agent: added configurable limit for log files to be rotated [[GH-5831](https://github.com/hashicorp/consul/pull/5831)]
* api: The v1/status endpoints can now be forwarded to remote datacenters [[GH-6198](https://github.com/hashicorp/consul/pull/6198)]

BUG FIXES:

* autopilot: update to also remove failed nodes from WAN gossip pool [[GH-6028](https://github.com/hashicorp/consul/pull/6028)]
* agent: avoid reverting any check updates that occur while a service is being added or the config is reloaded [[GH-6144](https://github.com/hashicorp/consul/issues/6144)]
* auto-encrypt: fix an issue that could cause cloud retry-join to fail when utilized with auto-encrypt by falling back to a default port [[GH-6205]](https://github.com/hashicorp/consul/pull/6205)

## 1.5.2 (June 27, 2019)

FEATURE

* tls: auto_encrypt enables automatic RPC cert provisioning for consul clients [[GH-5597](https://github.com/hashicorp/consul/pull/5597)]

IMPROVEMENTS

* ui: allow for customization of consul UI path [[GH-5950](https://github.com/hashicorp/consul/pull/5950)]
* acl: allow service deregistration with node write permission [[GH-5217](https://github.com/hashicorp/consul/pull/5217)]
* agent: support for maximum size for Output of checks [[GH-5233](https://github.com/hashicorp/consul/pull/5233)]
* agent: improve startup message when no error occurs [[GH-5896](https://github.com/hashicorp/consul/issues/5896)]
* agent: make sure client agent rate limits apply when hitting the client interface on a server directly [[GH-5927](https://github.com/hashicorp/consul/pull/5927)]
* agent: use stale requests when performing full sync [[GH-5873](https://github.com/hashicorp/consul/pull/5873)]
* agent: transfer leadership when establishLeadership fails [[GH-5247](https://github.com/hashicorp/consul/pull/5247)]
* agent: added metadata information about servers into consul service description [[GH-5455](https://github.com/hashicorp/consul/pull/5455)]
* connect: provide -admin-access-log-path for envoy [[GH-5858](https://github.com/hashicorp/consul/pull/5858)]
* connect: upgrade Envoy xDS protocol to support Envoy 1.10 [[GH-5872](https://github.com/hashicorp/consul/pull/5872)]
* dns: support alt domains for dns resolution [[GH-5940](https://github.com/hashicorp/consul/pull/5940)]
* license (enterprise): add command to reset license to builtin one
* ui: Improve linking between sidecars and proxies and their services/service instances [[GH-5944](https://github.com/hashicorp/consul/pull/5944)]
* ui: Add ability to search for tokens by policy, role or service identity name [[GH-5811](https://github.com/hashicorp/consul/pull/5811)]

BUG FIXES:

* agent: fix several data races and bugs related to node-local alias checks [[GH-5876](https://github.com/hashicorp/consul/issues/5876)]
* api: update link to agent caching in comments [[GH-5935](https://github.com/hashicorp/consul/pull/5935)]
* connect: fix proxy address formatting for IPv6 addresses [[GH-5460](https://github.com/hashicorp/consul/issues/5460)]
* connect: store signingKeyId instead of authorityKeyId [[GH-6005](https://github.com/hashicorp/consul/pull/6005)]
* ui: fix service instance linking when multiple non-unique service id's exist on multiple nodes [[GH-5933](https://github.com/hashicorp/consul/pull/5933)]
* ui: Improve error messaging for ACL policies [[GH-5836](https://github.com/hashicorp/consul/pull/5836)]
* txn: Fixed an issue that would allow a CAS operation on a service to work when it shouldn't have. [[GH-5971](https://github.com/hashicorp/consul/pull/5971)]

## 1.5.1 (May 22, 2019)

SECURITY:

* acl: fixed an issue that if an ACL rule is used for prefix matching in a policy, keys not matching that specific prefix can be deleted by a token using that policy even with default_deny settings configured [[GH-5888](https://github.com/hashicorp/consul/issues/5888)]

BUG FIXES:

* agent: Fixed an issue where recreating a node using a different ID would prevent the new node from correctly joining. [[GH-5485](https://github.com/hashicorp/consul/pull/5485)]

## 1.5.0 (May 08, 2019)

SECURITY:
* connect: Envoy versions lower than 1.9.1 are vulnerable to
 [CVE-2019-9900](https://github.com/envoyproxy/envoy/issues/6434) and
 [CVE-2019-9901](https://github.com/envoyproxy/envoy/issues/6435). Both are
 related to HTTP request parsing and so only affect Consul Connect users if they
 have configured HTTP routing rules via the ["escape
 hatch"](#custom-configuration). We recommend Envoy 1.9.1 be used.
 Note that while we officially deprecate support for older version of Envoy in 1.5.0,
 we recommend using Envoy 1.9.1 with all previous versions of Consul Connect too
 (back to 1.3.0 where Envoy support was introduced).

BREAKING CHANGES:

* /watch: (note this only affects downstream programs importing `/watch` package as a library not the `watch` feature in Consul) The watch package was moved from github.com/hashicorp/consul/watch to github.com/hashicorp/consul/api/watch to live in the API module. This was necessary after updating the repo to use Go modules or else various other bugs cropped up. The watch package API has not changed so projects depending on it should need to only update the import statement to get their code functioning again. [[GH-5664](https://github.com/hashicorp/consul/pull/5664)]
* ui: Legacy UI has been removed. Setting the CONSUL_UI_LEGACY environment variable to 1 or true will no longer revert to serving the old UI. [[GH-5643](https://github.com/hashicorp/consul/pull/5643)]

FEATURES:
* **Connect Envoy Supports L7 Observability:** We introduce features that allow configuring Envoy sidecars to emit metrics and tracing at L7 (http, http2, grpc supported). For more information see the [Envoy Integration](https://consul.io/docs/connect/proxies/envoy.html) docs.
* **Centralized Configuration:** Enables central configuration of some service and proxy defaults. For more information see the [Configuration Entries](https://consul.io/docs/agent/config_entries.html) docs
* api: Implement data filtering for some endpoints using a new filtering language. [[GH-5579](https://github.com/hashicorp/consul/pull/5579)]
* snapshot agent (Consul Enterprise): Added support for saving snapshots to Azure Blob Storage.
* acl: tokens can be created with an optional expiration time [[GH-5353](https://github.com/hashicorp/consul/issues/5353)]
* acl: tokens can now be assigned an optional set of service identities [[GH-5390](https://github.com/hashicorp/consul/issues/5390)]
* acl: tokens can now be assigned to roles [[GH-5514](https://github.com/hashicorp/consul/issues/5514)]
* acl: adding support for kubernetes auth provider login [[GH-5600](https://github.com/hashicorp/consul/issues/5600)]
* ui: Template-able Dashboard links for Service detail pages [[GH-5704](https://github.com/hashicorp/consul/pull/5704)] [[GH-5777](https://github.com/hashicorp/consul/pull/5777)]
* ui: support for ACL Roles [[GH-5635](https://github.com/hashicorp/consul/pull/5635)]


IMPROVEMENTS:
* cli: allow to add ip addresses as Subject Alternative Names when creating certificates with `consul tls cert create` [[GH-5602](https://github.com/hashicorp/consul/pull/5602)]
* dns: Allow for hot-reload of many DNS configurations. [[GH-4875](https://github.com/hashicorp/consul/pull/4875)]
* agent: config is now read if json or hcl is set as the config-format or the extension is either json or hcl [[GH-5723](https://github.com/hashicorp/consul/issues/5723)]
* acl: Allow setting token accessor ids and secret ids during token creation. [[GH-4977](https://github.com/hashicorp/consul/issues/4977)]
* ui: Service Instances page redesign and further visibility of Connect Proxies [[GH-5326]](https://github.com/hashicorp/consul/pull/5326)
* ui: Blocking Query support / live updates for Services and Nodes, requires enabling per user via the UI Settings area [[GH-5070]](https://github.com/hashicorp/consul/pull/5070) [[GH-5267]](https://github.com/hashicorp/consul/pull/5267)
* ui: Finer grained searching for the Service listing page [[GH-5507]](https://github.com/hashicorp/consul/pull/5507)
* ui: Add proxy icons to proxy services and instances where appropriate [[GH-5463](https://github.com/hashicorp/consul/pull/5463)]

BUG FIXES:

* api: fix panic in 'consul acl set-agent-token' [[GH-5533](https://github.com/hashicorp/consul/issues/5533)]
* api: fix issue in the transaction API where the health check definition struct wasn't being deserialized properly [[GH-5553](https://github.com/hashicorp/consul/issues/5553)]
* acl: memdb filter of tokens-by-policy was inverted [[GH-5575](https://github.com/hashicorp/consul/issues/5575)]
* acl: Fix legacy rules translation for JSON based rules. [[GH-5493](https://github.com/hashicorp/consul/issues/5493)]
* agent: Fixed a bug causing RPC errors when the `discovery_max_stale` time was exceeded. [[GH-4673](https://github.com/hashicorp/consul/issues/4673)]
* agent: Fix an issue with registering health checks for an agent service where the service name would be missing. [[GH-5705](https://github.com/hashicorp/consul/issues/5705)]
* connect: fix an issue where Envoy would fail to bootstrap if some upstreams were unavailable [[GH-5499](https://github.com/hashicorp/consul/pull/5499)]
* connect: fix an issue where health checks on proxies might be missed by watchers of `/health/service/:service` API [[GH-5506](https://github.com/hashicorp/consul/issues/5506)]
* connect: fix a race condition that could leave proxies with no configuration for long periods on startup [[GH-5793](https://github.com/hashicorp/consul/issues/5793)]
* logger: fix an issue where the `log-file` option was not respecting the `log-level` [[GH-4778](https://github.com/hashicorp/consul/issues/4778)]
* catalog: fix an issue where renaming nodes could cause registration instability [[GH-5518](https://github.com/hashicorp/consul/issues/5518)]
* network areas (Consul Enterprise): Fixed an issue that could cause a lock to be held unnecessarily causing other operations to hang.

## 1.4.5 (May 22, 2019)

SECURITY:

* acl: fixed an issue that if an ACL rule is used for prefix matching in a policy, keys not matching that specific prefix can be deleted by a token using that policy even with default_deny settings configured [[GH-5888](https://github.com/hashicorp/consul/issues/5888)]

## 1.4.4 (March 21, 2019)

SECURITY:

* Fixed a problem where `verify_server_hostname` was not being respected and the default `false` was being used. This problem exists only in Consul 1.4.3. (CVE-2019-9764) [[GH-5519](https://github.com/hashicorp/consul/issues/5519)]

FEATURES:
* agent: enable reloading of agent-to-agent TLS configuration [[GH-5419](https://github.com/hashicorp/consul/pull/5419)]

IMPROVEMENTS:
* api: `/health/service/:service` blocking queries now only need a single goroutine regardless of number of instances in the service and watch channel which can massively reduce the number of goroutines on busy servers. [[GH-5449](https://github.com/hashicorp/consul/pull/5449)]

BUG FIXES:

* api: Fixed a bug where updating node information wasn't reflected in health result index. [[GH-5450](https://github.com/hashicorp/consul/issues/5450)]
* agent: Fixed a bug that would cause removal of all of an agents health checks when only one service was removed. [[GH-5456](https://github.com/hashicorp/consul/issues/5456)]
* connect: Fixed a bug where `sidecar_service` registered proxies might not be removed correctly due to ACLs for the service being removed first dissallowing the agent permission to delete the proxy. [[GH-5482](https://github.com/hashicorp/consul/pull/5482)]
* tlsutil: don't use `server_name` config for RPC connections. [[GH-5394](https://github.com/hashicorp/consul/pull/5394)]

## 1.4.3 (March 5, 2019)

SECURITY:

* Fixed a potential privilege escalation issue with the Consul 1.4.X ACL system when ACL token replication was enabled. (CVE-2019-8336) [[GH-5423](https://github.com/hashicorp/consul/issues/5423)]

BUG FIXES:

* agent: Fixed a bug that could cause invalid memberlist protocol versions to propagate throughout the cluster. [[GH-3217](https://github.com/hashicorp/consul/issues/3217)]
* server: Fixed a race condition during server initialization and leadership monitoring. [[GH-5322](https://github.com/hashicorp/consul/pull/5322)]
* agent: only enable TLS on gRPC if the HTTPS API port is enabled [[GH-5287](https://github.com/hashicorp/consul/issues/5287)]
* agent: Fixed default log file permissions. [[GH-5346](https://github.com/hashicorp/consul/issues/5346)]
* api: Fixed bug where `/connect/intentions` endpoint didn't return `X-Consul-Index` [[GH-5355](https://github.com/hashicorp/consul/pull/5355)]
* agent: Ensure that reaped servers are removed from RPC routing. [[GH-5317](https://github.com/hashicorp/consul/pull/5317)]
* acl: Fix potential race condition when listing or retrieving ACL tokens. [[GH-5412](https://github.com/hashicorp/consul/pull/5412)]
* agent: Fixed race condition that could turn up while registering services on the local agent. [[GH-4998](https://github.com/hashicorp/consul/issues/4998)]

FEATURES:
* prepared queries: Enable ServiceMeta filtering for prepared queries. [[GH-5291](https://github.com/hashicorp/consul/pull/5291)]
* dns: Enabled caching of RPC responses within the DNS server. [[GH-5300](https://github.com/hashicorp/consul/pull/5300)]

IMPROVEMENTS:

* agent: Check ACLs more often for xDS stream endpoints. [[GH-5237](https://github.com/hashicorp/consul/issues/5237)]
* connect: Sidecar services now inherit tags and service metadata of the parent service by default. [[GH-5291](https://github.com/hashicorp/consul/pull/5291)]
* connect: Envoy proxies can now have cluster-specific config overrides via new "escape hatches": [[GH-5308](https://github.com/hashicorp/consul/pull/5308)]
* agent: Added opt-in ACL token persistence for tokens set with the agent/token/* endpoints: [[GH-5328](https://github.com/hashicorp/consul/pull/5328)]
* agent: Default to requiring protocol version 1.2 for TLS connections. The docs previously said this was going to be the default in 0.8+ but it had been left at 1.0 until now. [[GH-5340](https://github.com/hashicorp/consul/pull/5340)]

## 1.4.2 (January 28, 2019)

BUG FIXES:

* api: Fixed backwards compatibility in the Consul Go API client. [[GH-5270](https://github.com/hashicorp/consul/issues/5270)]
* dns: Fixed a bug that would cause node meta TXT records to always be generated even if they were not used in the responses. [[GH-5271](https://github.com/hashicorp/consul/issues/5271)]

## 1.4.1 (January 23, 2019)

**Note:** Consul 1.4.1 can break compatibility with older versions of the Consul Go API client. At this time, we recommend that you not upgrade to 1.4.1 if you use the Go API client or other applications that utilize it such as Nomad. Read more: [[GH-5270](https://github.com/hashicorp/consul/issues/5270)]

FEATURES:

* api: The transaction API now supports catalog operations for interacting with nodes, services and checks. See the [transacton API page](https://www.consul.io/api/txn.html#tables-of-operations) for more information. [[GH-4869](https://github.com/hashicorp/consul/pull/4869)]

SECURITY:

* Fixed an issue that caused `verify_server_hostname` to not implicitly configure `verify_outgoing` to true. The documentation stated this was implicit. The previous implementation had a bug that resulted in this being partially incorrect and resulted in plaintext communication in agent-to-agent RPC when `verify_outgoing` was not explicitly set. (CVE-2018-19653) [[GH-5069](https://github.com/hashicorp/consul/issues/5069)]


IMPROVEMENTS:

* agent: Improve blocking queries for services that do not exist. [[GH-4810](https://github.com/hashicorp/consul/pull/4810)]
* api: Added new `/v1/agent/health/service/name/<service name>` and `/v1/agent/health/service/id/<service id>` endpoints  to allow querying a services status from the agent itself and avoid querying a Consul server. [[GH-2488](https://github.com/hashicorp/consul/issues/2488)]
* api: Added a new `allow_write_http_from` configuration to set which CIDR network ranges can send non GET/HEAD/OPTIONS HTTP requests. Requests originating from other addresses will be denied. [[GH-4712](https://github.com/hashicorp/consul/issues/4712)]
* cli: Added a new cli command: `consul tls` with subcommands `ca create` and `cert create` to help bootstrapping a secure agent TLS setup. This includes a new guide for creating certificates.
* connect: clients are smarter about when they regenerate leaf certificates to improve performance and reliability [[GH-5091](https://github.com/hashicorp/consul/pull/5091)]
* gossip: CPU performance improvements to memberlist gossip on very large clusters [[GH-5189](https://github.com/hashicorp/consul/pull/5189)]
* connect: Added support for prepared query upstream proxy destination type watching. [[GH-4969](https://github.com/hashicorp/consul/issues/4969)
* connect: (Consul Enterprise) Now forwards any intention API calls from secondary datacenters to the primary instead of erroring when intention replication is enabled.
* connect: Now controls rate of Certificate Signing Requests during a CA rotation so the servers aren't overwhelmed. [[GH-5228](https://github.com/hashicorp/consul/pull/5228)]

BUG FIXES:

* acl: Fixed a concurrent policy resolution issue that would fail to resolve policies for a token [[GH-5219](https://github.com/hashicorp/consul/issues/5219)]
* acl: Fixed a few racey edge cases regarding policy resolution where the RPC request could error out due to the token used for the request being deleted or modified after the token was read but before policy resolution. [[GH-5246](https://github.com/hashicorp/consul/pull/5246)]
* acl: Fixed a bug that would cause legacy ACL tokens of type management to not get full privileges when they also had rules set on them. [[GH-5261](https://github.com/hashicorp/consul/pull/5261)]
* agent: Prevent health check status flapping during check re-registration. [[GH-4904](https://github.com/hashicorp/consul/pull/4904)]
* agent: Consul 1.2.3 added DNS weights but this caused an issue with agent Anti-Entropy that didn't set the same default and so performed a re-sync every 2 minutes despite no changes. [[GH-5096](https://github.com/hashicorp/consul/pull/5096)]
* agent: Fix an anti-entropy state syncing issue where an invalid token being used for registration of 1 service could cause a failure to register a different service with a valid token. [[GH-3676](https://github.com/hashicorp/consul/issues/3676)]
* agent: (Consul Enterprise) Snapshot agent now uses S3 API for unversioned objects to workaround an issue when a bucket has versioning enabled.
* agent: Fixed a bug where agent cache could return an error older than the last non-error value stored. This mostly affected Connect bootstrapping in integration environments but lead to some very hard to track down "impossible" issues [[GH-4480](https://github.com/hashicorp/consul/issues/4480)]
* agent: snapshot verification now works regardless of spacing in `meta.json` [[GH-5193](https://github.com/hashicorp/consul/issues/5193)]
* agent: Fixed a bug where `disable_host_node_id = false` was not working properly [[GH-4914](https://github.com/hashicorp/consul/issues/4914)]
* agent: Fixed issue where DNS weights added in 1.2.3 caused unnecessary Anti-Entropy syncs due to implicit vs explicit default weights being considered "different". [[GH-5126](https://github.com/hashicorp/consul/pull/5126)]
* api: Fixed an issue where service discovery requests that use both `?cached` and multiple repeated tag filters might incorrectly see the cached result for a different query [[GH-4987](https://github.com/hashicorp/consul/pull/4987)]
* api: Fixed an issue causing blocking query wait times to not be used when retrieving leaf certificates. [[GH-4462](https://github.com/hashicorp/consul/issues/4462)]
* cli: display messages from serf in cli [[GH-5236](https://github.com/hashicorp/consul/pull/5236)]
* connect: Fixed an issue where a blank CA config could be written to a snapshot when Connect was disabled. [[GH-4954](https://github.com/hashicorp/consul/pull/4954)]
* connect: Fixed a bug with the create and modify indices of leaf certificates not being incremented properly. [[GH-4463](https://github.com/hashicorp/consul/issues/4463)]
* connect: Fixed an issue where certificates could leak and remain in client memory forever [[GH-5091](https://github.com/hashicorp/consul/pull/5091)]
* connect: (Consul Enterprise) When requesting to sign intermediates the primary dc is now used
* connect: added tls config for vault connect ca provider [[GH-5125](https://github.com/hashicorp/consul/issues/5125)]
* connect: Fix a panic on 32 bit systems for unaligned 64 bit atomic operations. [[GH-5128](https://github.com/hashicorp/consul/issues/5128)]
* debug: Fixed an issue causing the debug archive to not be gzipped. [[GH-5141](https://github.com/hashicorp/consul/issues/5141)]
* dns: Fix an issue causing infinite recursion for some DNS queries when a nodes address had bee misconfigured [[GH-4907](https://github.com/hashicorp/consul/issues/4907)]
* watch: Fix a data race during setting up a watch plan. [[GH-4357](https://github.com/hashicorp/consul/issues/4357)]
* ui: Correctly encode/decode URLs within the KV areas. Also encode/decode slashes in URLS related to service names [[GH5206](https://github.com/hashicorp/consul/pull/5206)]

## 1.4.0 (November 14, 2018)

FEATURES:

* **New ACL System:** The ACL system has been redesigned while allowing for
  in-place upgrades that will automatically migrate to the new system while
  retaining compatibility for existing ACL tokens for clusters where ACLs are
  enabled. This new system introduces a number of improvements to tokens
  including accessor IDs and a new policy model. It also includes a new CLI for
  ACL interactions and a completely redesigned UI experience to manage ACLs and
  policies. WAN federated clusters will need to add the additional replication
  token configuration in order to ensure WAN ACL replication in the new system.
  [[GH-4791](https://github.com/hashicorp/consul/pull/4791)]
    * ACL CLI.
    * New ACL HTTP APIs.
    * Splitting ACL Tokens into Tokens and Policies with rules being defined on policies and tokens being linked to policies.
    * ACL Tokens have a public accessor ID now in addition to the secret ID that they used to have.
    * Setting a replication token is now required but it only needs "read" permissions on ACLs.
    * Update to the rules language to allow for exact-matching rules in addition to prefix matching rules
    * Added DC local tokens.
    * Auto-Transitioning from legacy mode to normal mode as the cluster's servers get upgraded.
    * ACL UI updates to support new functionality.

* **Multi-datacenter Connect:** (Consul Enterprise) Consul Connect now supports multi-datacenter connections and
replicates intentions. This allows WAN federated DCs to provide connections
from source and destination proxies in any DC.

* New command `consul debug` which gathers information about the cluster to help
  resolve incidents and debug issues faster. [[GH-4754](https://github.com/hashicorp/consul/issues/4754)]

IMPROVEMENTS:

* dns: Implement prefix lookups for DNS TTL. [[GH-4605](https://github.com/hashicorp/consul/issues/4605)]
* ui: Add JSON and YAML linting to the KV code editor. [[GH-4814](https://github.com/hashicorp/consul/pull/4814)]
* connect: Fix comment DYNAMIC_DNS to LOGICAL_DNS. [[GH-4799](https://github.com/hashicorp/consul/pull/4799)]
* terraform: fix formatting of consul.tf. [[GH-4580](https://github.com/hashicorp/consul/pull/4580)]

BUG FIXES:

* snapshot: Fixed a bug where node ID and datacenter weren't being included in or restored from the snapshots. [[GH-4872](https://github.com/hashicorp/consul/issues/4872)]
* api: Fixed migration issue where changes to allow multiple tags in 1.3.0 would cause broken results during a migration from earlier versions [[GH-4944](https://github.com/hashicorp/consul/pull/4944)]

## 1.3.1 (November 13, 2018)

BUG FIXES:
 * api: Fix issue introduced in 1.3.0 where catalog queries with tag filters
   change behaviour during upgrades from 1.2.x or earlier. (Back-ported from
   1.4.0 release candidate.) [[GH-4944](https://github.com/hashicorp/consul/issues/4944)].

## 1.3.0 (October 11, 2018)

FEATURES:

* **Connect Envoy Support**: This release includes support for using Envoy as a
  Proxy with Consul Connect (Beta). Read the [announcement blog
  post](https://www.hashicorp.com/blog/consul-1-3-envoy) or [reference
  documentation](https://www.consul.io/docs/connect/proxies/envoy.html)
  for more detail.
* **Sidecar Service Registration**: As part of the ongoing Connect Beta we add a
  new, more convenient way to [register sidecar
  proxies](https://www.consul.io/docs/connect/proxies/sidecar-service.html)
  from within a regular service definition.
* **Deprecating Managed Proxies**: The Connect Beta launched with a feature
  named "managed proxies". These will no longer be supported in favour of the
  simpler sidecar service registration. Existing functionality will not be
  removed until a later major release but will not be supported with fixes. See
  the [deprecation
  notice](https://www.consul.io/docs/connect/proxies/managed-deprecated.html)
  for full details.
* New command `consul services register` and `consul services deregister` for
  registering and deregistering services from the command line. [[GH-4732](https://github.com/hashicorp/consul/issues/4732)]
* api: Service discovery endpoints now support [caching results in the local agent](https://www.consul.io/api/index.html#agent-caching). [[GH-4541](https://github.com/hashicorp/consul/pull/4541)]
* dns: Added SOA configuration for DNS settings. [[GH-4713](https://github.com/hashicorp/consul/issues/4713)]

IMPROVEMENTS:

* ui: Improve layout of node 'cards' by restricting the grid layout to a maximum of 4 columns [[GH-4761]](https://github.com/hashicorp/consul/pull/4761)
* ui: Load the TextEncoder/Decoder polyfill dynamically so it's not downloaded to browsers with native support [[GH-4767](https://github.com/hashicorp/consul/pull/4767)]
* cli: `consul connect proxy` now supports a [`--sidecar-for`
  option](https://www.consul.io/docs/commands/connect/proxy.html#sidecar-for) to
  allow simple integration with new sidecar service registrations.
* api: /health and /catalog endpoints now support filtering by multiple tags [[GH-1781](https://github.com/hashicorp/consul/issues/1781)]
* agent: Only update service `ModifyIndex` when it's state actually changes. This makes service watches much more efficient on large clusters. [[GH-4720](https://github.com/hashicorp/consul/pull/4720)]
* config: Operators can now enable script checks from local config files only. [[GH-4711](https://github.com/hashicorp/consul/issues/4711)]

BUG FIXES:

* agent: (Consul Enterprise) Fixed an issue where the `non_voting_server` setting could be ignored when bootstrapping the cluster. [[GH-4699](https://github.com/hashicorp/consul/pull/4699)]
* cli: forward SIGTERM to child process of 'lock' and 'watch' subcommands [[GH-4737](https://github.com/hashicorp/consul/pull/4737)]
* connect: Fix to ensure leaf certificates for a service are not shared between clients on the same agent using different ACL tokens [[GH-4736](https://github.com/hashicorp/consul/pull/4736)]
* ui: Ensure service names that contain slashes are displayable [[GH-4756]](https://github.com/hashicorp/consul/pull/4756)
* watch: Fix issue with HTTPs only agents not executing watches properly. [[GH-4727](https://github.com/hashicorp/consul/pull/4727)]

## 1.2.4 (November 27, 2018)

SECURITY:

 * agent: backported enable_local_script_checks feature from 1.3.0. [Announcement](https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations) [[GH-4711](https://github.com/hashicorp/consul/issues/4711)]

## 1.2.3 (September 13, 2018)

FEATURES:

* agent: New Cloud Auto-join provider: Kubernetes (K8S) [[GH-4635](https://github.com/hashicorp/consul/issues/4635)]
* http: Added support for "Authorization: Bearer" head in addition to the X-Consul-Token header. [[GH-4483](https://github.com/hashicorp/consul/issues/4483)]
* dns: Added a way to specify SRV weights for each service instance to allow weighted DNS load-balancing. [[GH-4198](https://github.com/hashicorp/consul/pull/4198)]
* dns: Include EDNS-ECS options in EDNS responses where appropriate: see [RFC 7871](https://tools.ietf.org/html/rfc7871#section-7.1.3) [[GH-4647](https://github.com/hashicorp/consul/pull/4647)]
* ui: Add markers/icons for external sources [[GH-4640]](https://github.com/hashicorp/consul/pull/4640)

IMPROVEMENTS:

* ui: Switch to fullscreen layout for lists and detail, left aligned forms [[GH-4435]](https://github.com/hashicorp/consul/pull/4435)
* connect: TLS certificate readiness now performs x509 certificate verification to determine whether the cert is usable. [[GH-4540](https://github.com/hashicorp/consul/pull/4540)]
* ui: The syntax highlighting/code editor is now on by default [[GH-4651]](https://github.com/hashicorp/consul/pull/4651)
* ui: Fallback to showing `Node.Address` if `Service.Address` is not set [[GH-4579]](https://github.com/hashicorp/consul/issues/4579)
* gossip: Improvements to Serf and memberlist improving gossip stability on very large clusters (over 35k tested) [[GH-4511](https://github.com/hashicorp/consul/pull/4511)]

BUG FIXES:
* agent: Avoid returning empty data on startup of a non-leader server [[GH-4554](https://github.com/hashicorp/consul/pull/4554)]
* agent: Fixed a panic when serf_wan port was -1 but a reconnect_timeout_wan value was set. [[GH-4515](https://github.com/hashicorp/consul/issues/4515)]
* agent: Fixed a problem where errors regarding DNS server creation where never shown. [[GH-4578](https://github.com/hashicorp/consul/issues/4578)]
* agent: Start with invalid http configuration again, even though the build-in proxy for connect won't start in that case. [[GH-4655](https://github.com/hashicorp/consul/pull/4655)]
* catalog: Allow renaming nodes with IDs. [[GH-3974](https://github.com/hashicorp/consul/issues/3974)],[[GH-4413](https://github.com/hashicorp/consul/issues/4413)],[[GH-4415](https://github.com/hashicorp/consul/pull/4415)]
* dns: Fixes a bug with the DNS recursor, where we would not move onto the next provided recursor if we encounter a **SERVFAIL** or **REFUSED** status. [[GH-4461](https://github.com/hashicorp/consul/pull/4461)]
* server: Fixed a memory leak in blocking queries against /event/list. [[GH-4482](https://github.com/hashicorp/consul/issues/4482)]
* server: Fixed an issue where autopilot health checking could mistakenly mark healthy servers as failed, causing a non-voting server to be promoted unnecessarily. [[GH-4528]](https://github.com/hashicorp/consul/pull/4528)
* snapshot: Fixed a bug where node metadata wasn't being included in or restored from the snapshots. [[GH-4524](https://github.com/hashicorp/consul/issues/4524)]
* connect: Fixed a bug where managed proxy instances registered for instances with different name and ID and with restrictive ACL would not be allowed. [[GH-4619](https://github.com/hashicorp/consul/issues/4619)]
* connect: Fixed a bug where built-in CA state was not correctly restored from a snapshot [[GH-4535](https://github.com/hashicorp/consul/pull/4535)]
* connect: Fixed a bug where Checks with `deregister_critical_service_after` would deregister the service but not remove the managed proxy [[GH-4649](github.com/hashicorp/consul/pull/4649)]
* connect: Fixed a bug that would output an error about pruning CAs every hour on the leader and might cause some CA configurations not to be pruned correctly [[GH-4669](https://github.com/hashicorp/consul/pull/4669)]
* raft: Update raft vendoring to pull in a fix for a potential memory leak. [[GH-4539](https://github.com/hashicorp/consul/pull/4539)]
* license: (Consul Enterprise) Fix an issue with the license not being reloaded from snapshots.
* license: (Consul Enterprise) Fix an issue with encoding/decoding of the license package type from the /v1/operator/license endpoint.
* cli: Correctly exit with error code 1 when failing to list DCs with the catalog command [[GH-4583]]
* ui: Improve layout on screens of a large portrait orientation [[GH-4564]](https://github.com/hashicorp/consul/pull/4564)
* ui: Various browser layout bugs for various vendors/setups [[GH-4608]](https://github.com/hashicorp/consul/pull/4608) [[GH-4613]](https://github.com/hashicorp/consul/pull/4613) [[GH-4615]](https://github.com/hashicorp/consul/pull/4615)

## 1.2.2 (July 30, 2018)

SECURITY:
* acl: Fixed an issue where writes operations on the Keyring and Operator were being allowed with a default allow policy even when explicitly denied in the policy. [[GH-4378](https://github.com/hashicorp/consul/issues/4378)]

FEATURES:

* **Alias Checks:** Alias checks allow a service or node to alias the health status of another service or node in the cluster. [[PR-4320](https://github.com/hashicorp/consul/pull/4320)]
* agent: New Cloud Auto-join providers: vSphere and Packet.net. [[GH-4412](https://github.com/hashicorp/consul/issues/4412)]
* cli: Added `-serf-wan-port`, `-serf-lan-port`, and `-server-port` flags to CLI for cases where these can't be specified in config files and `-hcl` is too cumbersome. [[GH-4353](https://github.com/hashicorp/consul/pull/4353#issuecomment-404408827)]
* connect: The TTL of leaf (service) certificates in Connect is now configurable. [[GH-4400](https://github.com/hashicorp/consul/pull/4400)]

IMPROVEMENTS:

* proxy: With `-register` flag, heartbeat failures will only log once service registration succeeds. [[GH-4314](https://github.com/hashicorp/consul/pull/4314)]
* http: 1.0.3 introduced rejection of non-printable chars in HTTP URLs due to a security vulnerability. Some users who had keys written with an older version which are now dissallowed were unable to delete them. A new config option [disable_http_unprintable_char_filter](https://www.consul.io/docs/agent/options.html#disable_http_unprintable_char_filter) is added to allow those users to remove the offending keys. Leaving this new option set long term is strongly discouraged as it bypasses filtering necessary to prevent some known vulnerabilities. [[GH-4442](https://github.com/hashicorp/consul/pull/4442)]
* agent: Allow for advanced configuration of some gossip related parameters. [[GH-4058](https://github.com/hashicorp/consul/issues/4058)]
* agent: Make some Gossip tuneables configurable via the config file [[GH-4444](https://github.com/hashicorp/consul/pull/4444)]
* ui: Included searching on `.Tags` when using the freetext search field. [[GH-4383](https://github.com/hashicorp/consul/pull/4383)]
* ui: Service.ID's are now shown in the Service detail page and (only if it is different from the service name) the Node Detail > [Services] tab. [[GH-4387](https://github.com/hashicorp/consul/pull/4387)]

BUG FIXES:

* acl/connect: Fix an issue that was causing managed proxies not to work when ACLs were enabled. [[GH-4441](https://github.com/hashicorp/consul/issues/4441)]
* connect: Fix issue with managed proxies and watches attempting to use a client addr that is 0.0.0.0 or :: [[GH-4403](https://github.com/hashicorp/consul/pull/4403)]
* connect: Allow Native and Unmanaged proxy configurations via config file [[GH-4443](https://github.com/hashicorp/consul/pull/4443)]
* connect: Fix bug causing 100% CPU on agent when Connect is disabled but a proxy is still running [[GH-4421](https://github.com/hashicorp/consul/issues/4421)]
* proxy: Don't restart proxies setup in a config file when Consul restarts [[GH-4407](https://github.com/hashicorp/consul/pull/4407)]
* ui: Display the Service.IP address instead of the Node.IP address in the Service detail view. [[GH-4410](https://github.com/hashicorp/consul/pull/4410)]
* ui: Watch for trailing slash stripping 301 redirects and forward the user to the correct location. [[GH-4373](https://github.com/hashicorp/consul/pull/4373)]
* connect: Fixed an issue in the connect native HTTP client where it failed to resolve service names. [[GH-4392](https://github.com/hashicorp/consul/pull/4392)]

## 1.2.1 (July 12, 2018)

IMPROVEMENTS:

* acl: Prevented multiple ACL token refresh operations from occurring simultaneously. [[GH-3524](https://github.com/hashicorp/consul/issues/3524)]
* acl: Add async-cache down policy mode to always do ACL token refreshes in the background to reduce latency. [[GH-3524](https://github.com/hashicorp/consul/issues/3524)]
* proxy: Pass through HTTP client env vars to managed proxies so that they can connect back to Consul over HTTPs when not serving HTTP. [[PR-4374](https://github.com/hashicorp/consul/pull/4374)]
* connect: Persist intermediate CAs on leader change. [[PR-4379](https://github.com/hashicorp/consul/pull/4379)]

BUG FIXES:

* api: Intention APIs parse error response body for error message. [[GH-4297](https://github.com/hashicorp/consul/issues/4297)]
* agent: Intention read endpoint returns a 400 on invalid UUID [[GH-4297](https://github.com/hashicorp/consul/issues/4297)]
* agent: Service registration with "services" does not error on Connect upstream configuration. [[GH-4308](https://github.com/hashicorp/consul/issues/4308)]
* dns: Ensure that TXT RRs dont get put in the Answer section for A/AAAA queries. [[GH-4354](https://github.com/hashicorp/consul/issues/4354)]
* dns: Ensure that only 1 CNAME is returned when querying for services that have non-IP service addresses. [[PR-4328](https://github.com/hashicorp/consul/pull/4328)]
* api: Fixed issue where `Lock` and `Semaphore` would return earlier than their requested timeout when unable to acquire the lock. [[GH-4003](https://github.com/hashicorp/consul/issues/4003)], [[GH-3262](https://github.com/hashicorp/consul/issues/3262)], [[GH-2399](https://github.com/hashicorp/consul/issues/2399)]
* watch: Fix issue with HTTPs only agents not executing watches properly [[GH-4358](https://github.com/hashicorp/consul/issues/4358)]
* agent: Managed proxies that bind to 0.0.0.0 now get a health check on a reasonable IP [[GH-4301](https://github.com/hashicorp/consul/issues/4301)]
* server: (Consul Enterprise) Fixed an issue causing Consul to panic when network areas were used
* license: (Consul Enterprise) Fixed an issue causing the snapshot agent to log erroneous licensing errors

## 1.2.0 (June 26, 2018)

FEATURES:

* **Connect Feature Beta**: This version includes a major new feature for Consul named Connect. Connect enables secure service-to-service communication with automatic TLS encryption and identity-based authorization. For more details and links to demos and getting started guides, see the [announcement blog post](https://www.hashicorp.com/blog/consul-1-2-service-mesh).
  * Connect must be enabled explicitly in configuration so upgrading a cluster will not affect any existing functionality until it's enabled.
  * This is a Beta feature, we don't recommend enabling this in production yet. Please see the documentation for more information.
* dns: Enable PTR record lookups for services with IPs that have no registered node [[PR-4083](https://github.com/hashicorp/consul/pull/4083)]
* ui: Default to serving the new UI. Setting the `CONSUL_UI_LEGACY` environment variable to `1` or `true` will revert to serving the old UI

IMPROVEMENTS:

* agent: A Consul user-agent string is now sent to providers when making retry-join requests [[GH-4013](https://github.com/hashicorp/consul/issues/4013)]
* client: Add metrics for failed RPCs [PR-4220](https://github.com/hashicorp/consul/pull/4220)
* agent: Add configuration entry to control including TXT records for node meta in DNS responses [PR-4215](https://github.com/hashicorp/consul/pull/4215)
* client: Make RPC rate limit configuration reloadable [[GH-4012](https://github.com/hashicorp/consul/issues/4012)]

BUG FIXES:

* agent: Fixed an issue where watches were being duplicated on reload. [[GH-4179](https://github.com/hashicorp/consul/issues/4179)]
* agent: Fixed an issue with Agent watches on a HTTPS only agent would fail to use TLS. [[GH-4076](https://github.com/hashicorp/consul/issues/4076)]
* agent: Fixed bug that would cause unnecessary and frequent logging yamux keepalives [[GH-3040](https://github.com/hashicorp/consul/issues/3040)]
* dns: Re-enable full DNS compression [[GH-4071](https://github.com/hashicorp/consul/issues/4071)]


## 1.1.1 (November 27, 2018)

SECURITY:

 * agent: backported enable_local_script_checks feature from 1.3.0. [Announcement](https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations) [[GH-4711](https://github.com/hashicorp/consul/issues/4711)]

## 1.1.0 (May 11, 2018)

FEATURES:

* UI: The web UI has been completely redesigned and rebuilt and is in an opt-in beta period.
Setting the `CONSUL_UI_BETA` environment variable to `1` or `true` will replace the existing UI
with the new one. The existing UI will be deprecated and removed in a future release. [[GH-4086](https://github.com/hashicorp/consul/pull/4086)]
* api: Added support for Prometheus client format in metrics endpoint with `?format=prometheus` (see [docs](https://www.consul.io/api/agent.html#view-metrics)) [[GH-4014](https://github.com/hashicorp/consul/issues/4014)]
* agent: New Cloud Auto-join provider: Joyent Triton. [[GH-4108](https://github.com/hashicorp/consul/pull/4108)]
* agent: (Consul Enterprise) Implemented license management with license propagation within a datacenter.

BREAKING CHANGES:

* agent: The following previously deprecated fields and config options have been removed [[GH-4097](https://github.com/hashicorp/consul/pull/4097)]:
  - `CheckID` has been removed from config file check definitions (use `id` instead).
  - `script` has been removed from config file check definitions (use `args` instead).
  - `enableTagOverride` is no longer valid in service definitions (use `enable_tag_override` instead).
  - The [deprecated set of metric names](https://consul.io/docs/upgrade-specific.html#metric-names-updated) (beginning with `consul.consul.`) has been removed along with the `enable_deprecated_names` option from the metrics configuration.

IMPROVEMENTS:

* agent: Improve DNS performance on large clusters [[GH-4036](https://github.com/hashicorp/consul/issues/4036)]
* agent: `start_join`, `start_join_wan`, `retry_join`, `retry_join_wan` config params now all support go-sockaddr templates [[GH-4102](https://github.com/hashicorp/consul/pull/4102)]
* server: Added new configuration options `raft_snapshot_interval` and `raft_snapshot_threshold` to allow operators to  configure how often servers take raft snapshots. The default values for these have been tuned for large and busy clusters with high write load. [[GH-4105](https://github.com/hashicorp/consul/pull/4105/)]

BUG FIXES:

* agent: Only call signal.Notify once during agent startup [[PR-4024](https://github.com/hashicorp/consul/pull/4024)]
* agent: Add support for the new Service Meta field in agent config [[GH-4045](https://github.com/hashicorp/consul/issues/4045)]
* api: Add support for the new Service Meta field in API client [[GH-4045](https://github.com/hashicorp/consul/issues/4045)]
* agent: Updated serf library for two bug fixes - allow enough time for leave intents to propagate [[GH-510](https://github.com/hashicorp/serf/pull/510)] and preventing a deadlock [[GH-507](https://github.com/hashicorp/serf/pull/510)]
* agent: When node-level checks (e.g. maintenance mode) were deleted, some watchers currently in between blocking calls may have missed the change in index. See [[GH-3970](https://github.com/hashicorp/consul/pull/3970)]

## 1.0.8 (November 27, 2018)

SECURITY:

 * agent: backported enable_local_script_checks feature from 1.3.0. [Announcement](https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations) [[GH-4711](https://github.com/hashicorp/consul/issues/4711)]

## 1.0.7 (April 13, 2018)

IMPROVEMENTS:

* build: Bumped Go version to 1.10 [[GH-3988](https://github.com/hashicorp/consul/pull/3988)]
* agent: Blocking queries on service-specific health and catalog endpoints now return a per-service `X-Consul-Index` improving watch performance on very busy clusters. [[GH-3890](https://github.com/hashicorp/consul/issues/3890)]. **Note this may break blocking clients that relied on undocumented implementation details** as noted in the [upgrade docs](https://github.com/hashicorp/consul/blob/main/website/source/docs/upgrading.html.md#upgrade-from-version-106-to-higher).
* agent: All endpoints now respond to OPTIONS requests. [[GH-3885](https://github.com/hashicorp/consul/issues/3885)]
* agent: List of supported TLS cipher suites updated to include newer options, [[GH-3962](https://github.com/hashicorp/consul/pull/3962)]
* agent: WAN federation can now be disabled by setting the serf WAN port to -1. [[GH-3984](https://github.com/hashicorp/consul/issues/3984)]
* agent: Added support for specifying metadata during service registration. [[GH-3881](https://github.com/hashicorp/consul/issues/3881)]
* agent: Added a new `discover-max-stale` config option to enable stale requests for service discovery endpoints. [[GH-4004](https://github.com/hashicorp/consul/issues/4004)]
* agent: (Consul Enterprise) Added a new option to the snapshot agent for configuring the S3 endpoint.
* dns: Introduced a new config param to limit the number of A/AAAA records returned. [[GH-3940](https://github.com/hashicorp/consul/issues/3940)]
* dns: Upgrade vendored DNS library to pick up bugfixes and improvements. [[GH-3978](https://github.com/hashicorp/consul/issues/3978)]
* server: Updated yamux library to pick up a performance improvement. [[GH-3982](https://github.com/hashicorp/consul/issues/3982)]
* server: Add near=\_ip support for prepared queries [[GH-3798](https://github.com/hashicorp/consul/issues/3798)]
* api: Add support for GZIP compression in HTTP responses. [[GH-3687](https://github.com/hashicorp/consul/issues/3687)]
* api: Add `IgnoreCheckIDs` to Prepared Query definition to allow temporarily bypassing faulty health checks [[GH-3727](https://github.com/hashicorp/consul/issues/3727)]

BUG FIXES:

* agent: Fixed an issue where the coordinate update endpoint was not correctly parsing the ACL token. [[GH-3892](https://github.com/hashicorp/consul/issues/3892)]
* agent: Fixed an issue where `consul monitor` couldn't be terminated until the first log line is delivered [[GH-3891](https://github.com/hashicorp/consul/issues/3891)]
* agent: Added warnings for when a node name isn't a valid DNS name and when the node name, a service name or service tags would exceed the allowed lengths for DNS names [[GH-3854](https://github.com/hashicorp/consul/issues/3854)]
* agent: Added truncation of TCP DNS responses to prevent errors for exceeding message size limits [[GH-3850](https://github.com/hashicorp/consul/issues/3850)]
* agent: Added -config-format flag to validate command to specify the syntax that should be used for parsing the config [[GH-3996](https://github.com/hashicorp/consul/issues/3996)]
* agent: HTTP Checks now report the HTTP method used instead of always reporting as a GET
* server: Fixed an issue where the leader could miss clean up after a leadership transition. [[GH-3909](https://github.com/hashicorp/consul/issues/3909)]

## 1.0.6 (February 9, 2018)

BUG FIXES:

* agent: Fixed a panic when using the Azure provider for retry-join. [[GH-3875](https://github.com/hashicorp/consul/issues/3875)]
* agent: Fixed a panic when querying Consul's DNS interface over TCP. [[GH-3877](https://github.com/hashicorp/consul/issues/3877)]

## 1.0.5 (February 7, 2018)

NOTE ON SKIPPED RELEASE 1.0.4:

We found [[GH-3867](https://github.com/hashicorp/consul/issues/3867)] after cutting the 1.0.4 release and pushing the 1.0.4 release tag, so we decided to scuttle that release and push 1.0.5 instead with a fix for that issue.

SECURITY:

* dns: Updated DNS vendor library to pick up bug fix in the DNS server where an open idle connection blocks the accept loop. [[GH-3859](https://github.com/hashicorp/consul/issues/3859)]

FEATURES:

* agent: Added support for gRPC health checks that probe the standard gRPC health endpoint. [[GH-3073](https://github.com/hashicorp/consul/issues/3073)]

IMPROVEMENTS:

* agent: (Consul Enterprise) The `disable_update_check` option to disable Checkpoint now defaults to `true` (this is only in the Enterprise version).
* build: Bumped Go version to 1.9.3. [[GH-3837](https://github.com/hashicorp/consul/issues/3837)]

BUG FIXES:

* agent: (Consul Enterprise) Fixed an issue where the snapshot agent's HTTP client config was being ignored in favor of the HTTP command-line flags.
* agent: Fixed an issue where health checks added to services with tags would cause extra periodic writes to the Consul servers, even if nothing had changed. This could cause extra churn on downstream applications like consul-template or Fabio. [[GH-3845](https://github.com/hashicorp/consul/issues/3845)]
* agent: Fixed several areas where reading from catalog, health, or agent HTTP endpoints could make unintended mofidications to Consul's state in a way that would cause unnecessary anti-entropy syncs back to the Consul servers. This could cause extra churn on downstream applications like consul-template or Fabio. [[GH-3867](https://github.com/hashicorp/consul/issues/3867)]
* agent: Fixed an issue where Serf events for failed Consul servers weren't being proactively processed by the RPC router. This would prvent Consul from proactively choosing a new server, and would instead wait for a failed RPC request before choosing a new server. This exposed clients to a failed request, when often the proactive switching would avoid that. [[GH-3864](https://github.com/hashicorp/consul/issues/3864)]

## 1.0.3 (January 24, 2018)

SECURITY:

* ui: Patched handlebars JS to escape `=` to prevent potential XSS issues. [[GH-3733](https://github.com/hashicorp/consul/issues/3733)]

BREAKING CHANGES:

* agent: Updated Consul's HTTP server to ban all URLs containing non-printable characters (a bad request status will be returned for these cases). This affects some user-facing areas like key/value entry key names which are carried in URLs. [[GH-3762](https://github.com/hashicorp/consul/issues/3762)]

FEATURES:

* agent: Added retry-join support for Azure Virtual Machine Scale Sets. [[GH-3824](https://github.com/hashicorp/consul/issues/3824)]

IMPROVEMENTS:

* agent: Added agent-side telemetry around Catalog APIs to provide insight on Consul's operation from the user's perspecive. [[GH-3765](https://github.com/hashicorp/consul/issues/3765)]
* agent: Added the `NodeID` field back to the /v1/agent/self endpoint's `Config` block. [[GH-3778](https://github.com/hashicorp/consul/issues/3778)]
* api: Added missing `CheckID` and `Name` fields to API client's `AgentServiceCheck` structure so that IDs and names can be set when registering checks with services. [[GH-3788](https://github.com/hashicorp/consul/issues/3788)]

BUG FIXES:

* agent: Fixed an issue where config file symlinks were not being interpreted correctly. [[GH-3753](https://github.com/hashicorp/consul/issues/3753)]
* agent: Ignore malformed leftover service/check files and warn about them instead of refusing to start. [[GH-1221](https://github.com/hashicorp/consul/issues/1221)]
* agent: Enforce a valid port for the Serf WAN since it can't be disabled. [[GH-3817](https://github.com/hashicorp/consul/issues/3817)]
* agent: Stopped looging messages about zero RTTs when updating network coordinates since they are not harmful to the algorithm. Since we are still trying to find the root cause of these zero measurements, we added new metrics counters so these are still observable. [[GH-3789](https://github.com/hashicorp/consul/issues/3789)]
* server: Fixed a crash when POST-ing an empty body to the /v1/query endpoint. [[GH-3791](https://github.com/hashicorp/consul/issues/3791)]
* server: (Consul Enterprise) Fixed an issue where unhealthy servers were not replaced in a redundancy zone by autopilot (servers previously needed to be removed in order for a replacement to occur).
* ui: Added a URI escape around key/value keys so that it's not possible to create unexpected partial key names when entering characters like `?` inside a key. [[GH-3760](https://github.com/hashicorp/consul/issues/3760)]

## 1.0.2 (December 15, 2017)

IMPROVEMENTS:

* agent: Updated Serf to activate a new feature that resizes its internal message broadcast queue size based on the cluster size. This helps control the amount of memory used by the agent, but prevents spurious warnings about dropped messages in very large Consul clusters. The intent queue warnings have also been disabled since queue telemetry was already available and a simple fixed limit isn't applicable to all clusters, so it could cause a high rate of warnings about intent queue depth that were not useful or indicative of an actual issue. [[GH-3705](https://github.com/hashicorp/consul/issues/3705)]
* agent: Updates posener/complete library to 1.0, which allows autocomplete for flags after an equal sign, and simplifies autocomplete functions. [[GH-3646](https://github.com/hashicorp/consul/issues/3646)]

BUG FIXES:

* agent: Updated memberlist to pull in a fix for negative RTT measurements and their associated log messages about rejected coordinates. [[GH-3704](https://github.com/hashicorp/consul/issues/3704)]
* agent: Fixed an issue where node metadata specified via command line arguments overrode node metadata specified by configuration files, instead of merging as was done in versions of Consul prior to 1.0. [[GH-3716](https://github.com/hashicorp/consul/issues/3716)]
* agent: Fixed an issue with the /v1/session/create API where it wasn't possible to create a session without the `serfHealth` check. This is now possible again by including the `checks` key in the JSON body with an empty list. [[GH-3732](https://github.com/hashicorp/consul/issues/3732)]
* agent: Fixed an issue with anti-entropy syncing where checks for services with tags would cause periodic updates to the catalog, even when nothing had changed, causing the Raft index to grow slowly (~2 minutes per node per check) over time, and causing unnecessary writes and wake ups for blocking queries. [[GH-3642](https://github.com/hashicorp/consul/issues/3642)], [[GH-3259](https://github.com/hashicorp/consul/issues/3259)]
* cli: Added missing support for `-base64` option to `consul kv get` command. [[GH-3736](https://github.com/hashicorp/consul/issues/3736)]
* server: Fixed an issue with KV store tombstone tracking where bin tracking was being confused by monotonic time information carried in time stamps, resulting in many unnecessary bins. [[GH-3670](https://github.com/hashicorp/consul/issues/3670)]
* server: (Consul Enterprise) Fixed an issue with Network Segments where servers would not properly flood-join each other into all segments.
* server: Fixed an issue where it wasn't possible to disable Autopilot's dead server cleanup behavior using configuration files. [[GH-3730](https://github.com/hashicorp/consul/issues/3730)]
* server: Removed the 60 second timeout when restoring snapshots, which could cause large restores to fail on slower servers. [[GH-3326](https://github.com/hashicorp/consul/issues/3326)]
* server: Fixed a goroutine leak during keyring operations when errors are encountered. [[GH-3728](https://github.com/hashicorp/consul/issues/3728)]

## 1.0.1 (November 20, 2017)

FEATURES:

* **New Auto Join Cloud Providers:** Retry join support was added for Aliyun (Alibaba Cloud), Digital Ocean, OpenStack, and Scaleway. Instance metadata can be used with these to make it easy to form Consul clusters. [[GH-3634](https://github.com/hashicorp/consul/issues/3634)]
* **HTTP/2 Support:** If TLS is enabled on a Consul agent it will automatically negotiate to use HTTP/2 for suitably configured clients accessing the client API. This allows clients to multiplex requests over the same TCP connection, such as multiple, simultaneous blocking queries. [[GH-3657](https://github.com/hashicorp/consul/issues/3657)]

IMPROVEMENTS:

* agent: (Consul Enterprise) Added [AWS KMS support](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingKMSEncryption.html) for S3 snapshots using the snapshot agent.
* agent: Watches in the Consul agent can now be configured to invoke an HTTP endpoint instead of an executable. [[GH-3305](https://github.com/hashicorp/consul/issues/3305)]
* agent: Added a new [`-config-format`](https://www.consul.io/docs/agent/options.html#_config_format) command line option which can be set to `hcl` or `json` to specify the format of configuration files. This is useful for cases where the file name cannot be controlled in order to provide the required extension. [[GH-3620](https://github.com/hashicorp/consul/issues/3620)]
* agent: DNS recursors can now be specified as [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template) templates. [[GH-2932](https://github.com/hashicorp/consul/issues/2932)]
* agent: Serf snapshots no longer save network coordinate information. This enables recovery from errors upon agent restart. [[GH-489](https://github.com/hashicorp/serf/issues/489)]
* agent: Added defensive code to prevent out of range ping times from infecting network coordinates. Updates to the coordinate system with negative round trip times or round trip times higher than 10 seconds will log an error but will be ignored.
* agent: The agent now warns when there are extra unparsed command line arguments and refuses to start. [[GH-3397](https://github.com/hashicorp/consul/issues/3397)]
* agent: Updated go-sockaddr library to get CoreOS route detection fixes and the new `mask` functionality. [[GH-3633](https://github.com/hashicorp/consul/issues/3633)]
* agent: Added a new [`enable_agent_tls_for_checks`](https://www.consul.io/docs/agent/options.html#enable_agent_tls_for_checks) configuration option that allows HTTP health checks for services requiring 2-way TLS to be checked using the agent's credentials. [[GH-3364](https://github.com/hashicorp/consul/issues/3364)]
* agent: Made logging of health check status more uniform and moved log entries with full check output from DEBUG to TRACE level for less noise. [[GH-3683](https://github.com/hashicorp/consul/issues/3683)]
* build: Consul is now built with Go 1.9.2. [[GH-3663](https://github.com/hashicorp/consul/issues/3663)]

BUG FIXES:

* agent: Consul 1.0 shipped with an issue where `Args` was erroneously named `ScriptArgs` for health check definitions in the /v1/agent/check/register and /v1/agent/service/register APIs. Added code to accept `Args` so that the JSON format matches that of health checks in configuration files. The `ScriptArgs` form will still be supported for backwards compatibility. [[GH-3587](https://github.com/hashicorp/consul/issues/3587)]
* agent: Docker container checks running on Linux could get into a flapping state because the Docker agent seems to close the connection prematurely even though the body is transferred. This caused a "connection reset by peer" error which put the check into `critical` state. As of Consul 1.0.1 the "connection reset by peer" error is ignored for the `/exec/<execID>/start` command of the Docker API. [[GH-3576](https://github.com/hashicorp/consul/issues/3576)]
* agent: Added new form of `consul.http.*` metrics that were accidentally left out of Consul 1.0. [[GH-3654](https://github.com/hashicorp/consul/issues/3654)]
* agent: Fixed an issue with the server manager where periodic server client connection rebalancing could select a failed server. This affects agents in client mode, as well as servers talking to other servers, including over the WAN. [[GH-3463](https://github.com/hashicorp/consul/issues/3463)]
* agent: IPv6 addresses without port numbers and without surrounding brackets are now properly handled for joins. This affects all join types, but in particular this was discovered with AWS joins where the APIs return addresses formatted this way. [[GH-3671](https://github.com/hashicorp/consul/issues/3671)]
* agent: Fixed a rare startup panic of the Consul agent related to the LAN Serf instance ordering with the router manager. [[GH-3680](https://github.com/hashicorp/consul/issues/3680)]
* agent: Added back an exception for the `snapshot_agent` config key so that those configs can again live alongside Consul's configs. [[GH-3678](https://github.com/hashicorp/consul/issues/3678)]
* dns: Fixed an issue were components of a host name near the datacenter could be quietly ignored (eg. `foo.service.dc1.extra.consul` would silently ignore `.extra`); now an `NXDOMAIN` error will be returned. [[GH-3200](https://github.com/hashicorp/consul/issues/3200)]
* server: Fixed an issue where performing rolling updates of Consul servers could result in an outage from old servers remaining in the cluster. Consul's Autopilot would normally remove old servers when new ones come online, but it was also waiting to promote servers to voters in pairs to maintain an odd quorum size. The pairwise promotion feature was removed so that servers become voters as soon as they are stable, allowing Autopilot to remove old servers in a safer way. When upgrading from Consul 1.0, you may need to manually force-leave old servers as part of a rolling update to Consul 1.0.1. [[GH-3611](https://github.com/hashicorp/consul/issues/3611)]
* server: Fixed a deadlock where tombstone garbage collection for the KV store could block other KV operations, stalling writes on the leader. [[GH-3700](https://github.com/hashicorp/consul/issues/3700)]

## 1.0.0 (October 16, 2017)

SECURITY:

* ui: Fixed an XSS issue with Consul's built-in web UI where node names were not being properly escaped. [[GH-3578](https://github.com/hashicorp/consul/issues/3578)]

BREAKING CHANGES:

* **Raft Protocol Now Defaults to 3:** The [`-raft-protocol`](https://www.consul.io/docs/agent/options.html#_raft_protocol) default has been changed from 2 to 3, enabling all [Autopilot](https://www.consul.io/docs/guides/autopilot.html) features by default. Version 3 requires Consul running 0.8.0 or newer on all servers in order to work, so if you are upgrading with older servers in a cluster then you will need to set this back to 2 in order to upgrade. See [Raft Protocol Version Compatibility](https://www.consul.io/docs/upgrade-specific.html#raft-protocol-version-compatibility) for more details. Also the format of `peers.json` used for outage recovery is different when running with the lastest Raft protocol. See [Manual Recovery Using peers.json](https://www.consul.io/docs/guides/outage.html#manual-recovery-using-peers-json) for a description of the required format. [[GH-3477](https://github.com/hashicorp/consul/issues/3477)]
* **Config Files Require an Extension:** As part of supporting the [HCL](https://github.com/hashicorp/hcl#syntax) format for Consul's config files, an `.hcl` or `.json` extension is required for all config files loaded by Consul, even when using the [`-config-file`](https://www.consul.io/docs/agent/options.html#_config_file) argument to specify a file directly. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]
* **Deprecated Options Have Been Removed:** All of Consul's previously deprecated command line flags and config options have been removed, so these will need to be mapped to their equivalents before upgrading. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]

    <details><summary>Detailed List of Removed Options and their Equivalents</summary>

    | Removed Option | Equivalent |
    | -------------- | ---------- |
    | `-atlas` | None, Atlas is no longer supported. |
    | `-atlas-token`| None, Atlas is no longer supported. |
    | `-atlas-join` | None, Atlas is no longer supported. |
    | `-atlas-endpoint` | None, Atlas is no longer supported. |
    | `-dc` | [`-datacenter`](https://www.consul.io/docs/agent/options.html#_datacenter) |
    | `-retry-join-azure-tag-name` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#microsoft-azure) |
    | `-retry-join-azure-tag-value` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#microsoft-azure) |
    | `-retry-join-ec2-region` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `-retry-join-ec2-tag-key` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `-retry-join-ec2-tag-value` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `-retry-join-gce-credentials-file` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `-retry-join-gce-project-name` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `-retry-join-gce-tag-name` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `-retry-join-gce-zone-pattern` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `addresses.rpc` | None, the RPC server for CLI commands is no longer supported. |
    | `advertise_addrs` | [`ports`](https://www.consul.io/docs/agent/options.html#ports) with [`advertise_addr`](https://www.consul/io/docs/agent/options.html#advertise_addr) and/or [`advertise_addr_wan`](https://www.consul.io/docs/agent/options.html#advertise_addr_wan) |
    | `atlas_infrastructure` | None, Atlas is no longer supported. |
    | `atlas_token` | None, Atlas is no longer supported. |
    | `atlas_acl_token` | None, Atlas is no longer supported. |
    | `atlas_join` | None, Atlas is no longer supported. |
    | `atlas_endpoint` | None, Atlas is no longer supported. |
    | `dogstatsd_addr` | [`telemetry.dogstatsd_addr`](https://www.consul.io/docs/agent/options.html#telemetry-dogstatsd_addr) |
    | `dogstatsd_tags` | [`telemetry.dogstatsd_tags`](https://www.consul.io/docs/agent/options.html#telemetry-dogstatsd_tags) |
    | `http_api_response_headers` | [`http_config.response_headers`](https://www.consul.io/docs/agent/options.html#response_headers) |
    | `ports.rpc` | None, the RPC server for CLI commands is no longer supported. |
    | `recursor` | [`recursors`](https://github.com/hashicorp/consul/blob/main/website/source/docs/agent/options.html.md#recursors) |
    | `retry_join_azure` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#microsoft-azure) |
    | `retry_join_ec2` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `retry_join_gce` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `statsd_addr` | [`telemetry.statsd_address`](https://github.com/hashicorp/consul/blob/main/website/source/docs/agent/options.html.md#telemetry-statsd_address) |
    | `statsite_addr` | [`telemetry.statsite_address`](https://github.com/hashicorp/consul/blob/main/website/source/docs/agent/options.html.md#telemetry-statsite_address) |
    | `statsite_prefix` | [`telemetry.metrics_prefix`](https://www.consul.io/docs/agent/options.html#telemetry-metrics_prefix) |
    | `telemetry.statsite_prefix` | [`telemetry.metrics_prefix`](https://www.consul.io/docs/agent/options.html#telemetry-metrics_prefix) |
    | (service definitions) `serviceid` | [`service_id`](https://www.consul.io/docs/agent/services.html) |
    | (service definitions) `dockercontainerid` | [`docker_container_id`](https://www.consul.io/docs/agent/services.html) |
    | (service definitions) `tlsskipverify` | [`tls_skip_verify`](https://www.consul.io/docs/agent/services.html) |
    | (service definitions) `deregistercriticalserviceafter` | [`deregister_critical_service_after`](https://www.consul.io/docs/agent/services.html) |

    </details>

* **`statsite_prefix` Renamed to `metrics_prefix`:** Since the `statsite_prefix` configuration option applied to all telemetry providers, `statsite_prefix` was renamed to [`metrics_prefix`](https://www.consul.io/docs/agent/options.html#telemetry-metrics_prefix). Configuration files will need to be updated when upgrading to this version of Consul. [[GH-3498](https://github.com/hashicorp/consul/issues/3498)]
* **`advertise_addrs` Removed:** This configuration option was removed since it was redundant with `advertise_addr` and `advertise_addr_wan` in combination with `ports` and also wrongly stated that you could configure both host and port. [[GH-3516](https://github.com/hashicorp/consul/issues/3516)]
* **Escaping Behavior Changed for go-discover Configs:** The format for [`-retry-join`](https://www.consul.io/docs/agent/options.html#retry-join) and [`-retry-join-wan`](https://www.consul.io/docs/agent/options.html#retry-join-wan) values that use [go-discover](https://github.com/hashicorp/go-discover) Cloud auto joining has changed. Values in `key=val` sequences must no longer be URL encoded and can be provided as literals as long as they do not contain spaces, backslashes `\` or double quotes `"`. If values contain these characters then use double quotes as in `"some key"="some value"`. Special characters within a double quoted string can be escaped with a backslash `\`. [[GH-3417](https://github.com/hashicorp/consul/issues/3417)]
* **HTTP Verbs are Enforced in Many HTTP APIs:** Many endpoints in the HTTP API that previously took any HTTP verb now check for specific HTTP verbs and enforce them. This may break clients relying on the old behavior. [[GH-3405](https://github.com/hashicorp/consul/issues/3405)]

    <details><summary>Detailed List of Updated Endpoints and Required HTTP Verbs</summary>

    | Endpoint | Required HTTP Verb |
    | -------- | ------------------ |
    | /v1/acl/info | GET |
    | /v1/acl/list | GET |
    | /v1/acl/replication | GET |
    | /v1/agent/check/deregister | PUT |
    | /v1/agent/check/fail | PUT |
    | /v1/agent/check/pass | PUT |
    | /v1/agent/check/register | PUT |
    | /v1/agent/check/warn | PUT |
    | /v1/agent/checks | GET |
    | /v1/agent/force-leave | PUT |
    | /v1/agent/join | PUT |
    | /v1/agent/members | GET |
    | /v1/agent/metrics | GET |
    | /v1/agent/self | GET |
    | /v1/agent/service/register | PUT |
    | /v1/agent/service/deregister | PUT |
    | /v1/agent/services | GET |
    | /v1/catalog/datacenters | GET |
    | /v1/catalog/deregister | PUT |
    | /v1/catalog/node | GET |
    | /v1/catalog/nodes | GET |
    | /v1/catalog/register | PUT |
    | /v1/catalog/service | GET |
    | /v1/catalog/services | GET |
    | /v1/coordinate/datacenters | GET |
    | /v1/coordinate/nodes | GET |
    | /v1/health/checks | GET |
    | /v1/health/node | GET |
    | /v1/health/service | GET |
    | /v1/health/state | GET |
    | /v1/internal/ui/node | GET |
    | /v1/internal/ui/nodes | GET |
    | /v1/internal/ui/services | GET |
    | /v1/session/info | GET |
    | /v1/session/list | GET |
    | /v1/session/node | GET |
    | /v1/status/leader | GET |
    | /v1/status/peers | GET |
    | /v1/operator/area/:uuid/members | GET |
    | /v1/operator/area/:uuid/join | PUT |

    </details>

* **Unauthorized KV Requests Return 403:** When ACLs are enabled, reading a key with an unauthorized token returns a 403. This previously returned a 404 response.
* **Config Section of Agent Self Endpoint has Changed:** The /v1/agent/self endpoint's `Config` section has often been in flux as it was directly returning one of Consul's internal data structures. This configuration structure has been moved under `DebugConfig`, and is documents as for debugging use and subject to change, and a small set of elements of `Config` have been maintained and documented. See [Read Configuration](https://www.consul.io/api/agent.html#read-configuration) endpoint documentation for details. [[GH-3532](https://github.com/hashicorp/consul/issues/3532)]
* **Deprecated `configtest` Command Removed:** The `configtest` command was deprecated and has been superseded by the `validate` command.
* **Undocumented Flags in `validate` Command Removed:** The `validate` command supported the `-config-file` and `-config-dir` command line flags but did not document them. This support has been removed since the flags are not required.
* **Metric Names Updated:** Metric names no longer start with `consul.consul`. To help with transitioning dashboards and other metric consumers, the field `enable_deprecated_names` has been added to the telemetry section of the config, which will enable metrics with the old naming scheme to be sent alongside the new ones. [[GH-3535](https://github.com/hashicorp/consul/issues/3535)]

    <details><summary>Detailed List of Affected Metrics by Prefix</summary>

    | Prefix |
    | ------ |
    | consul.consul.acl |
    | consul.consul.autopilot |
    | consul.consul.catalog |
    | consul.consul.fsm |
    | consul.consul.health |
    | consul.consul.http |
    | consul.consul.kvs |
    | consul.consul.leader |
    | consul.consul.prepared-query |
    | consul.consul.rpc |
    | consul.consul.session |
    | consul.consul.session_ttl |
    | consul.consul.txn |

    </details>

* **Checks Validated On Agent Startup:** Consul agents now validate health check definitions in their configuration and will fail at startup if any checks are invalid. In previous versions of Consul, invalid health checks would get skipped. [[GH-3559](https://github.com/hashicorp/consul/issues/3559)]

FEATURES:

* **Support for HCL Config Files:** Consul now supports HashiCorp's [HCL](https://github.com/hashicorp/hcl#syntax) format for config files. This is easier to work with than JSON and supports comments. As part of this change, all config files will need to have either an `.hcl` or `.json` extension in order to specify their format. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]
* **Support for Binding to Multiple Addresses:** Consul now supports binding to multiple addresses for its HTTP, HTTPS, and DNS services. You can provide a space-separated list of addresses to [`-client`](https://www.consul.io/docs/agent/options.html#_client) and [`addresses`](https://www.consul.io/docs/agent/options.html#addresses) configurations, or specify a [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template) template that resolves to multiple addresses. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]
* **Support for RFC1464 DNS TXT records:** Consul DNS responses now contain the node meta data encoded according to RFC1464 as TXT records. [[GH-3343](https://github.com/hashicorp/consul/issues/3343)]
* **Support for Running Subproccesses Directly Without a Shell:** Consul agent checks and watches now support an `args` configuration which is a list of arguments to run for the subprocess, which runs the subprocess directly without a shell. The old `script` and `handler` configurations are now deprecated (specify a shell explicitly if you require one). A `-shell=false` option is also available on `consul lock`, `consul watch`, and `consul exec` to run the subprocesses associated with those without a shell. [[GH-3509](https://github.com/hashicorp/consul/issues/3509)]
* **Sentinel Integration:** (Consul Enterprise) Consul's ACL system integrates with [Sentinel](https://www.consul.io/docs/guides/sentinel.html) to enable code policies that apply to KV writes.

IMPROVEMENTS:

* agent: Added support to detect public IPv4 and IPv6 addresses on AWS. [[GH-3471](https://github.com/hashicorp/consul/issues/3471)]
* agent: Improved /v1/operator/raft/configuration endpoint which allows Consul to avoid an extra agent RPC call for the `consul operator raft list-peers` command. [[GH-3449](https://github.com/hashicorp/consul/issues/3449)]
* agent: Improved ACL system for the KV store to support list permissions. This behavior can be opted in. For more information, see the [ACL Guide](https://www.consul.io/docs/guides/acl.html#list-policy-for-keys). [[GH-3511](https://github.com/hashicorp/consul/issues/3511)]
* agent: Updates miekg/dns library to later version to pick up bug fixes and improvements. [[GH-3547](https://github.com/hashicorp/consul/issues/3547)]
* agent: Added automatic retries to the RPC path, and a brief RPC drain time when servers leave. These changes make Consul more robust during graceful leaves of Consul servers, such as during upgrades, and help shield applications from "no leader" errors. These are configured with new [`performance`](https://www.consul.io/docs/agent/options.html#performance) options. [[GH-3514](https://github.com/hashicorp/consul/issues/3514)]
* agent: Added a new `discard_check_output` agent-level configuration option that can be used to trade off write load to the Consul servers vs. visibility of health check output. This is reloadable so it can be toggled without fully restarting the agent. [[GH-3562](https://github.com/hashicorp/consul/issues/3562)]
* api: Updated the API client to ride out network errors when monitoring locks and semaphores. [[GH-3553](https://github.com/hashicorp/consul/issues/3553)]
* build: Updated Go toolchain to version 1.9.1. [[GH-3537](https://github.com/hashicorp/consul/issues/3537)]
* cli: `consul lock` and `consul watch` commands will forward `TERM` and `KILL` signals to their child subprocess. [[GH-3509](https://github.com/hashicorp/consul/issues/3509)]
* cli: Added support for [autocompletion](https://www.consul.io/docs/commands/index.html#autocompletion). [[GH-3412](https://github.com/hashicorp/consul/issues/3412)]
* server: Updated BoltDB to final version 1.3.1. [[GH-3502](https://github.com/hashicorp/consul/issues/3502)]
* server: Improved dead member reap algorithm to fix edge cases where servers could get left behind. [[GH-3452](https://github.com/hashicorp/consul/issues/3452)]

BUG FIXES:

* agent: Fixed an issue where disabling both the http and https interfaces would cause a watch-related error on agent startup, even when no watches were defined. [[GH-3425](https://github.com/hashicorp/consul/issues/3425)]
* agent: Added an additional step to kill health check scripts that timeout on all platforms except Windows, and added a wait so that it's not possible to run multiple instances of the same health check script at the same time. [[GH-3565](https://github.com/hashicorp/consul/issues/3565)]
* cli: If the `consul operator raft list-peers` command encounters an error it will now exit with a non-zero exit code. [[GH-3513](https://github.com/hashicorp/consul/issues/3513)]
* cli: CLI commands will now show help for all of their arguments. [[GH-3536](https://github.com/hashicorp/consul/issues/3536)]
* server: Fixed an issue where the leader server could get into a state where it was no longer performing the periodic leader loop duties and unable to serve consistent reads after a barrier timeout error. [[GH-3545](https://github.com/hashicorp/consul/issues/3545)]

## 0.9.4 (November 27, 2018)

SECURITY:

 * agent: backported enable_local_script_checks feature from 1.3.0. [Announcement](https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations) [[GH-4711](https://github.com/hashicorp/consul/issues/4711)]

## 0.9.3 (September 8, 2017)

FEATURES:
* **LAN Network Segments:** (Consul Enterprise) Added a new [Network Segments](https://www.consul.io/docs/guides/segments.html) capability which allows users to configure Consul to support segmented LAN topologies with multiple, distinct gossip pools. [[GH-3431](https://github.com/hashicorp/consul/issues/3431)]
* **WAN Join for Cloud Providers:** Added WAN support for retry join for Cloud providers via go-discover, including Amazon AWS, Microsoft Azure, Google Cloud, and SoftLayer. This uses the same "provider" syntax supported for `-retry-join` via the `-retry-join-wan` configuration. [[GH-3406](https://github.com/hashicorp/consul/issues/3406)]
* **RPC Rate Limiter:** Consul agents in client mode have a new [`limits`](https://www.consul.io/docs/agent/options.html#limits) configuration that enables a rate limit on RPC calls the agent makes to Consul servers. [[GH-3140](https://github.com/hashicorp/consul/issues/3140)]

IMPROVEMENTS:

* agent: Switched to using a read lock for the agent's RPC dispatcher, which prevents RPC calls from getting serialized. [[GH-3376](https://github.com/hashicorp/consul/issues/3376)]
* agent: When joining a cluster, Consul now skips the unique node ID constraint for Consul members running Consul older than 0.8.5. This makes it easier to upgrade to newer versions of Consul in an existing cluster with non-unique node IDs. [[GH-3070](https://github.com/hashicorp/consul/issues/3070)]
* build: Upgraded Go version to 1.9. [[GH-3428](https://github.com/hashicorp/consul/issues/3428)]
* server: Consul servers can re-establish quorum after all of them change their IP addresses upon a restart. [[GH-1580](https://github.com/hashicorp/consul/issues/1580)]
* ui: Changed text area font family to monospace to make it easier to manage complex text blocks. [[GH-3521](https://github.com/hashicorp/consul/issues/3521)]

BUG FIXES:

* agent: Fixed an issue with consul watches not triggering when ACLs are enabled. [[GH-3392](https://github.com/hashicorp/consul/issues/3392)]
* agent: Updated memberlist library for a deadlock fix. [[GH-3396](https://github.com/hashicorp/consul/issues/3396)]
* agent: Fixed a panic when retrieving NS or SOA records on Consul clients (non-servers). This also changed the Consul server list to come from the catalog and not the agent's local state when serving these requests, so the results are consistent across a cluster. [[GH-3407](https://github.com/hashicorp/consul/issues/3407)]
* cli: Updated the CLI library to pull in a fix that prevents all subcommands from being shown when showing the agent's usage list; now just top-level commands are shown. [[GH-3448](https://github.com/hashicorp/consul/issues/3448)]
* server: Fixed an issue with Consul snapshots not saving on Windows because of errors with the `fsync` syscall. [[GH-3409](https://github.com/hashicorp/consul/issues/3409)]

## 0.9.2 (August 9, 2017)

BUG FIXES:

* agent: Fixed an issue where the old `-retry-join-{ec2,azure,gce}` command line flags were not being honored. [[GH-3384](https://github.com/hashicorp/consul/issues/3384)]
* server: Reverted the change that made unauthorized KV queries return 403 instead of 404 because it had a minor bug that affected the operation of Vault, and in addition to fixing the bug, we identified an additional case that needed to be covered. This restores the <= 0.9.0 behavior until we can get a complete fix. [[GH-2637](https://github.com/hashicorp/consul/issues/2637)]

## 0.9.1 (August 9, 2017)

FEATURES:

* **Secure ACL Token Introduction:** It's now possible to manage Consul's ACL tokens without having to place any tokens inside configuration files. This supports introduction of tokens as well as rotating. This is enabled with two new APIs:
    * A new [`/v1/agent/token`](https://www.consul.io/api/agent.html#update-acl-tokens) API allows an agent's ACL tokens to be introduced without placing them into config files, and to update them without restarting the agent. See the [ACL Guide](https://www.consul.io/docs/guides/acl.html#create-an-agent-token) for an example. This was extended to ACL replication as well, along with a new [`enable_acl_replication`](https://www.consul.io/docs/agent/options.html#enable_acl_replication) config option. [GH-3324,GH-3357]
    * A new [`/v1/acl/bootstrap`](https://www.consul.io/api/acl.html#bootstrap-acls) allows a cluster's first management token to be created without using the `acl_master_token` configuration. See the [ACL Guide](https://www.consul.io/docs/guides/acl.html#bootstrapping-acls) for an example. [[GH-3349](https://github.com/hashicorp/consul/issues/3349)]
* **Metrics Viewing Endpoint:** A new [`/v1/agent/metrics`](https://www.consul.io/api/agent.html#view-metrics) API displays the current values of internally tracked metrics. [[GH-3369](https://github.com/hashicorp/consul/issues/3369)]

IMPROVEMENTS:

* agent: Retry Join for Amazon AWS, Microsoft Azure, Google Cloud, and (new) SoftLayer is now handled through the https://github.com/hashicorp/go-discover library. With this all `-retry-join-{ec2,azure,gce}-*` parameters have been deprecated in favor of a unified configuration. See [`-retry-join`](https://www.consul.io/docs/agent/options.html#_retry_join) for details. [GH-3282,GH-3351]
* agent: Reports a more detailed error message if the LAN or WAN Serf instance fails to bind to an address. [[GH-3312](https://github.com/hashicorp/consul/issues/3312)]
* agent: Added NS records and corrected SOA records to allow Consul's DNS interface to work properly with zone delegation. [[GH-1301](https://github.com/hashicorp/consul/issues/1301)]
* agent: Added support for sending metrics with labels/tags to supported backends. [[GH-3369](https://github.com/hashicorp/consul/issues/3369)]
* agent: Added a new `prefix_filter` option in the `telemetry` config to allow fine-grained allowing/blocking the sending of certain metrics by prefix. [[GH-3369](https://github.com/hashicorp/consul/issues/3369)]
* cli: Added a `-child-exit-code` option to `consul lock` so that it propagates an error code of 2 if the child process exits with an error. [[GH-947](https://github.com/hashicorp/consul/issues/947)]
* docs: Added a new [Geo Failover Guide](https://www.consul.io/docs/guides/geo-failover.html) showing how to use prepared queries to implement geo failover policies for services. [[GH-3328](https://github.com/hashicorp/consul/issues/3328)]
* docs: Added a new [Consul with Containers Guide](https://www.consul.io/docs/guides/consul-containers.html) showing critical aspects of operating a Consul cluster that's run inside containers. [[GH-3347](https://github.com/hashicorp/consul/issues/3347)]
* server: Added a `RemoveEmptyTags` option to prepared query templates which will strip out any empty strings in the tags list before executing a query. This is useful when interpolating into tags in a way where the tag is optional, and where searching for an empty tag would yield no results from the query. [[GH-2151](https://github.com/hashicorp/consul/issues/2151)]
* server: Implemented a much faster recursive delete algorithm for the KV store. It has been benchmarked to be up to 100X faster on recursive deletes that affect millions of keys. [GH-1278, GH-3313]

BUG FIXES:

* agent: Clean up temporary files during disk write errors when persisting services and checks. [[GH-3207](https://github.com/hashicorp/consul/issues/3207)]
* agent: Fixed an issue where DNS and client bind address templates were not being parsed via the go-sockaddr library. [[GH-3322](https://github.com/hashicorp/consul/issues/3322)]
* agent: Fixed status code on all KV store operations that fail due to an ACL issue. They now return a 403 status code, rather than a 404. [[GH-2637](https://github.com/hashicorp/consul/issues/2637)]
* agent: Fixed quoting issues in script health check on Windows. [[GH-1875](https://github.com/hashicorp/consul/issues/1875)]
* agent: Fixed an issue where `consul monitor` would exit on any empty log line. [[GH-3253](https://github.com/hashicorp/consul/issues/3253)]
* server: Updated raft library to fix issue with machine crashes causing snapshot files to not get saved to disk [[GH-3362](https://github.com/hashicorp/consul/issues/3362)]

## 0.9.0 (July 20, 2017)

BREAKING CHANGES:

* agent: Added a new [`enable_script_checks`](https://www.consul.io/docs/agent/options.html#_enable_script_checks) configuration option that defaults to `false`, meaning that in order to allow an agent to run health checks that execute scripts, this will need to be configured and set to `true`. This provides a safer out-of-the-box configuration for Consul where operators must opt-in to allow script-based health checks. [[GH-3087](https://github.com/hashicorp/consul/issues/3087)]
* api: Reworked `context` support in the API client to more closely match the Go standard library, and added context support to write requests in addition to read requests. [GH-3273, GH-2992]
* ui: Since the UI is now bundled with the application we no longer provide a separate UI package for downloading. [[GH-3292](https://github.com/hashicorp/consul/issues/3292)]

FEATURES:

* agent: Added a new [`block_endpoints`](https://www.consul.io/docs/agent/options.html#block_endpoints) configuration option that allows blocking HTTP API endpoints by prefix. This allows operators to completely disallow access to specific endpoints on a given agent. [[GH-3252](https://github.com/hashicorp/consul/issues/3252)]
* cli: Added a new [`consul catalog`](https://www.consul.io/docs/commands/catalog.html) command for reading datacenters, nodes, and services from the catalog. [[GH-3204](https://github.com/hashicorp/consul/issues/3204)]
* server: (Consul Enterprise) Added a new [`consul operator area update`](https://www.consul.io/docs/commands/operator/area.html#update) command and corresponding HTTP endpoint to allow for transitioning the TLS setting of network areas at runtime. [[GH-3075](https://github.com/hashicorp/consul/issues/3075)]
* server: (Consul Enterprise) Added a new `UpgradeVersionTag` field to the Autopilot config to allow for using the migration feature to roll out configuration or cluster changes, without having to upgrade Consul itself.

IMPROVEMENTS:

* agent: (Consul Enterprise) Snapshot agent rotation uses S3's pagination API, enabling retaining more than a 100 snapshots.
* agent: Removed registration of the `consul` service from the agent since it's already handled by the leader. This means that Consul servers no longer need to have an `acl_agent_token` with write access to the `consul` service if ACLs are enabled. [[GH-3248](https://github.com/hashicorp/consul/issues/3248)]
* agent: Changed /v1/acl/clone response to 403 (from 404) when trying to clone an ACL that doesn't exist. [[GH-1113](https://github.com/hashicorp/consul/issues/1113)]
* agent: Changed the `consul exec` ACL resolution logic to use the `acl_agent_token` if it's available. This lets operators configure an `acl_agent_token` with the required `write` privilieges to the `_rexec` prefix of the KV store without giving this to the `acl_token`, which would expose those privileges to users as well. [[GH-3160](https://github.com/hashicorp/consul/issues/3160)]
* agent: Updated memberlist to get latest LAN gossip tuning based on the [Lifeguard paper published by Hashicorp Research](https://www.hashicorp.com/blog/making-gossip-more-robust-with-lifeguard/). [[GH-3287](https://github.com/hashicorp/consul/issues/3287)]
* api: Added the ability to pass in a `context` as part of the `QueryOptions` during a request. This provides a way to cancel outstanding blocking queries. [[GH-3195](https://github.com/hashicorp/consul/issues/3195)]
* api: Changed signature for "done" channels on `agent.Monitor()` and `session.RenewPeriodic` methods to make them more compatible with `context`. [[GH-3271](https://github.com/hashicorp/consul/issues/3271)]
* docs: Added a complete end-to-end example of ACL bootstrapping in the [ACL Guide](https://www.consul.io/docs/guides/acl.html#bootstrapping-acls). [[GH-3248](https://github.com/hashicorp/consul/issues/3248)]
* vendor: Updated golang.org/x/sys/unix to support IBM s390 platforms. [[GH-3240](https://github.com/hashicorp/consul/issues/3240)]
* agent: rewrote Docker health checks without using the Docker client and its dependencies. [[GH-3270](https://github.com/hashicorp/consul/issues/3270)]

BUG FIXES:

* agent: Fixed an issue where watch plans would take up to 10 minutes to close their connections and give up their file descriptors after reloading Consul. [[GH-3018](https://github.com/hashicorp/consul/issues/3018)]
* agent: (Consul Enterprise) Fixed an issue with the snapshot agent where it could get stuck trying to obtain the leader lock after an extended server outage.
* agent: Fixed HTTP health checks to allow them to set the `Host` header correctly on outgoing requests. [[GH-3203](https://github.com/hashicorp/consul/issues/3203)]
* agent: Serf snapshots can now auto recover from disk write errors without needing a restart. [[GH-1744](https://github.com/hashicorp/consul/issues/1744)]
* agent: Fixed log redacting code to properly remove tokens from log lines with ACL tokens in the URL itself: `/v1/acl/clone/:uuid`, `/v1/acl/destroy/:uuid`, `/v1/acl/info/:uuid`. [[GH-3276](https://github.com/hashicorp/consul/issues/3276)]
* agent: Fixed an issue in the Docker client where Docker checks would get EOF errors trying to connect to a volume-mounted Docker socket. [[GH-3254](https://github.com/hashicorp/consul/issues/3254)]
* agent: Fixed a crash when using Azure auto discovery. [[GH-3193](https://github.com/hashicorp/consul/issues/3193)]
* agent: Added `node` read privileges to the `acl_agent_master_token` by default so it can see all nodes, which enables it to be used with operations like `consul members`. [[GH-3113](https://github.com/hashicorp/consul/issues/3113)]
* agent: Fixed an issue where enabling [`-disable-keyring-file`](https://www.consul.io/docs/agent/options.html#_disable_keyring_file) would cause gossip encryption to be disabled. [[GH-3243](https://github.com/hashicorp/consul/issues/3243)]
* agent: Fixed a race condition where checks that are not associated with any existing services were allowed to persist. [[GH-3297](https://github.com/hashicorp/consul/issues/3297)]
* agent: Stop docker checks on service deregistration and on shutdown. [GH-3265, GH-3295]
* server: Updated the Raft library to pull in a fix where servers that are very far behind in replication can get stuck in a loop trying to install snapshots. [[GH-3201](https://github.com/hashicorp/consul/issues/3201)]
* server: Fixed a rare but serious deadlock where the Consul leader routine could get stuck with the Raft internal leader routine while waiting for the initial barrier after a leader election. [[GH-3230](https://github.com/hashicorp/consul/issues/3230)]
* server: Added automatic cleanup of failed Raft snapshots. [[GH-3258](https://github.com/hashicorp/consul/issues/3258)]
* server: (Consul Enterprise) Fixed an issue where networks areas would not be able to be added when the server restarts if the Raft log contained a specific sequence of adds and deletes for network areas with the same peer datacenter.
* ui: Provided a path to reset the ACL token when the current token is invalid. Previously, the UI would get stuck on the error page and it wasn't possible to get back to the settings. [[GH-2370](https://github.com/hashicorp/consul/issues/2370)]
* ui: Removed an extra fetch of the nodes resource when loading the UI. [[GH-3245](https://github.com/hashicorp/consul/issues/3245)]
* ui: Changed default ACL token type to "client" when creating ACLs. [[GH-3246](https://github.com/hashicorp/consul/issues/3246)]
* ui: Display a 404 error instead of a 200 when trying to load a nonexistent node. [[GH-3251](https://github.com/hashicorp/consul/issues/3251)]

## 0.8.5 (June 27, 2017)

BREAKING CHANGES:

* agent: Parse values given to `?passing` for health endpoints. Previously Consul only checked for the existence of the querystring, not the value. That means using `?passing=false` would actually still include passing values. Consul now parses the value given to passing as a boolean. If no value is provided, the old behavior remains. This may be a breaking change for some users, but the old experience was incorrect and caused enough confusion to warrant changing it. [GH-2212, GH-3136]
* agent: The default value of [`-disable-host-node-id`](https://www.consul.io/docs/agent/options.html#_disable_host_node_id) has been changed from false to true. This means you need to opt-in to host-based node IDs and by default Consul will generate a random node ID. A high number of users struggled to deploy newer versions of Consul with host-based IDs because of various edge cases of how the host IDs work in Docker, on specially-provisioned machines, etc. so changing this from opt-out to opt-in will ease operations for many Consul users. [[GH-3171](https://github.com/hashicorp/consul/issues/3171)]

IMPROVEMENTS:

* agent: Added a `-disable-keyring-file` option to prevent writing keyring data to disk. [[GH-3145](https://github.com/hashicorp/consul/issues/3145)]
* agent: Added automatic notify to systemd on Linux after LAN join is complete, which makes it easier to order services that depend on Consul being available. [[GH-2121](https://github.com/hashicorp/consul/issues/2121)]
* agent: The `http_api_response_headers` config has been moved into a new `http_config` struct, so the old form is still supported but is deprecated. [[GH-3142](https://github.com/hashicorp/consul/issues/3142)]
* dns: Added support for EDNS(0) size adjustments if set in the request frame. This allows DNS responses via UDP which are larger than the standard 512 bytes max if the requesting client can support it. [GH-1980, GH-3131]
* server: Added a startup warning for servers when expecting to bootstrap with an even number of nodes. [[GH-1282](https://github.com/hashicorp/consul/issues/1282)]
* agent: (Consul Enterprise) Added support for non rotating, statically named snapshots for S3 snapshots using the snapshot agent.

BUG FIXES:

* agent: Fixed a regression where configuring -1 for the port was no longer disabling the DNS server. [[GH-3135](https://github.com/hashicorp/consul/issues/3135)]
* agent: Fix `consul leave` shutdown race. When shutting down an agent via the `consul leave` command on the command line the output would be `EOF` instead of `Graceful leave completed` [[GH-2880](https://github.com/hashicorp/consul/issues/2880)]
* agent: Show a better error message than 'EOF' when attempting to join with the wrong gossip key. [[GH-1013](https://github.com/hashicorp/consul/issues/1013)]
* agent: Fixed an issue where the `Method` and `Header` features of HTTP health checks were not being applied. [[GH-3178](https://github.com/hashicorp/consul/issues/3178)]
* agent: Fixed an issue where internally-configured watches were not working because of an incorrect protocol error, and unified internal watch handling during reloads of the Consul agent. [[GH-3177](https://github.com/hashicorp/consul/issues/3177)]
* server: Fixed an issue where the leader could return stale data duing queries as it is starting up. [[GH-2644](https://github.com/hashicorp/consul/issues/2644)]

## 0.8.4 (June 9, 2017)

FEATURES:

* agent: Added a method for [transitioning to gossip encryption on an existing cluster](https://www.consul.io/docs/agent/encryption.html#configuring-gossip-encryption-on-an-existing-cluster). [[GH-3079](https://github.com/hashicorp/consul/issues/3079)]
* agent: Added a method for [transitioning to TLS on an existing cluster](https://www.consul.io/docs/agent/encryption.html#configuring-tls-on-an-existing-cluster). [[GH-1705](https://github.com/hashicorp/consul/issues/1705)]
* agent: Added support for [RetryJoin on Azure](https://www.consul.io/docs/agent/options.html#retry_join_azure). [[GH-2978](https://github.com/hashicorp/consul/issues/2978)]
* agent: (Consul Enterprise) Added [AWS server side encryption support](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingServerSideEncryption.html) for S3 snapshots using the snapshot agent.

IMPROVEMENTS:

* agent: Added a check which prevents advertising or setting a service to a zero address (`0.0.0.0`, `[::]`, `::`). [[GH-2961](https://github.com/hashicorp/consul/issues/2961)]
* agent: Allow binding to any public IPv6 address with `::` [[GH-2285](https://github.com/hashicorp/consul/issues/2285)]
* agent: Removed SCADA-related code for Atlas and deprecated all Atlas-related configuration options. [[GH-3032](https://github.com/hashicorp/consul/issues/3032)]
* agent: Added support for custom check id and name when registering checks along with a service. [[GH-3047](https://github.com/hashicorp/consul/issues/3047)]
* agent: Updated [go-sockaddr](https://github.com/hashicorp/go-sockaddr) library to add support for new helper functions in bind address templates (`GetPrivateIPs`, `GetPublicIPs`), new math functions, and to pick up fixes for issues with detecting addresses on multi-homed hosts. [[GH-3068](https://github.com/hashicorp/consul/issues/3068)]
* agent: Watches now reset their index back to zero after an error, or if the index goes backwards, which allows watches to recover after a server restart with fresh state. [[GH-2621](https://github.com/hashicorp/consul/issues/2621)]
* agent: HTTP health checks now support custom method and headers. [[GH-1184](https://github.com/hashicorp/consul/issues/1184)], [[GH-2474](https://github.com/hashicorp/consul/issues/2474)], [[GH-2657](https://github.com/hashicorp/consul/issues/2657)], [[GH-3106](https://github.com/hashicorp/consul/issues/3106)]
* agent: Increased the graceful leave timeout from 5 to 15 seconds. [[GH-3121](https://github.com/hashicorp/consul/issues/3121)]
* agent: Added additional logging when the agent handles signals and when it exits. [[GH-3124](https://github.com/hashicorp/consul/issues/3124)]
* build: Added support for linux/arm64 binaries. [[GH-3042](https://github.com/hashicorp/consul/issues/3042)]
* build: Consul now builds with Go 1.8.3. [[GH-3074](https://github.com/hashicorp/consul/issues/3074)]
* ui: Added a sticky scroll to the KV side panel so the KV edit box always stays in place. [[GH-2812](https://github.com/hashicorp/consul/issues/2812)]

BUG FIXES:

* agent: Added defensive code to prevent agents from infecting the network coordinates with `NaN` or `Inf` values, and added code to clean up in environments where this has happened. [[GH-3023](https://github.com/hashicorp/consul/issues/3023)]
* api: Added code to always read from the body of a request so that connections will always be returned to the pool. [[GH-2850](https://github.com/hashicorp/consul/issues/2850)]
* build: Added a vendor fix to allow compilation on Illumos. [[GH-3024](https://github.com/hashicorp/consul/issues/3024)]
* cli: Fixed an issue where `consul exec` would return a 0 exit code, even when there were nodes that didn't respond. [[GH-2757](https://github.com/hashicorp/consul/issues/2757)]

## 0.8.3 (May 12, 2017)

BUG FIXES:

* agent: Fixed an issue where NAT-configured agents with a non-routable advertise address would refuse to make RPC connections to Consul servers. This was a regression related to GH-2822 in Consul 0.8.2. [[GH-3028](https://github.com/hashicorp/consul/issues/3028)]

## 0.8.2 (May 9, 2017)

BREAKING CHANGES:

* api: HttpClient now defaults to nil in the client config and will be generated if left blank. A NewHttpClient function has been added for creating an HttpClient with a custom Transport or TLS config. [[GH-2922](https://github.com/hashicorp/consul/issues/2922)]

IMPROVEMENTS:

* agent: Added an error at agent startup time if both `-ui` and `-ui-dir` are configured together. [[GH-2576](https://github.com/hashicorp/consul/issues/2576)]
* agent: Added the datacenter of a node to the catalog, health, and query API endpoints which contain a Node structure. [[GH-2713](https://github.com/hashicorp/consul/issues/2713)]
* agent: Added the `ca_path`, `tls_cipher_suites`, and `tls_prefer_server_cipher_suites` options to give more flexibility around configuring TLS. [[GH-2963](https://github.com/hashicorp/consul/issues/2963)]
* agent: Reduced the timeouts for the `-dev` server mode so that the development server starts up almost instantly. [[GH-2984](https://github.com/hashicorp/consul/issues/2984)]
* agent: Added `verify_incoming_rpc` and `verify_incoming_https` options for more granular control over incoming TLS enforcement. [[GH-2974](https://github.com/hashicorp/consul/issues/2974)]
* agent: Use bind address as source for outgoing connections. [[GH-2822](https://github.com/hashicorp/consul/issues/2822)]
* api: Added the ACL replication status endpoint to the Go API client library. [[GH-2947](https://github.com/hashicorp/consul/issues/2947)]
* cli: Added Raft protocol version to output of `operator raft list-peers` command.[[GH-2929](https://github.com/hashicorp/consul/issues/2929)]
* ui: Added optional JSON validation when editing KV entries in the web UI. [[GH-2712](https://github.com/hashicorp/consul/issues/2712)]
* ui: Updated ACL guide links and made guides open in a new tab. [[GH-3010](https://github.com/hashicorp/consul/issues/3010)]

BUG FIXES:

* server: Fixed a panic when the tombstone garbage collector was stopped. [[GH-2087](https://github.com/hashicorp/consul/issues/2087)]
* server: Fixed a panic in Autopilot that could occur when a node is elected but cannot complete leader establishment and steps back down. [[GH-2980](https://github.com/hashicorp/consul/issues/2980)]
* server: Added a new peers.json format that allows outage recovery when using Raft protocol version 3 and higher. Previously, you'd have to set the Raft protocol version back to 2 in order to manually recover a cluster. See https://www.consul.io/docs/guides/outage.html#manual-recovery-using-peers-json for more details. [[GH-3003](https://github.com/hashicorp/consul/issues/3003)]
* ui: Add and update favicons [[GH-2945](https://github.com/hashicorp/consul/issues/2945)]

## 0.8.1 (April 17, 2017)

IMPROVEMENTS:

* agent: Node IDs derived from host information are now hashed to prevent things like common server hardware from generating IDs with a common prefix across nodes. [[GH-2884](https://github.com/hashicorp/consul/issues/2884)]
* agent: Added new `-disable-host-node-id` CLI flag and `disable_host_node_id` config option to the Consul agent to prevent it from using information from the host when generating a node ID. This will result in a random node ID, which is useful when running multiple Consul agents on the same host for testing purposes. Having this built-in eases configuring a random node ID when running in containers. [[GH-2877](https://github.com/hashicorp/consul/issues/2877)]
* agent: Removed useless "==> Caught signal: broken pipe" logging since that often results from problems sending telemetry or broken incoming client connections; operators don't need to be alerted to these. [[GH-2768](https://github.com/hashicorp/consul/issues/2768)]
* cli: Added TLS options for setting the client/CA certificates to use when communicating with Consul. These can be provided through environment variables or command line flags. [[GH-2914](https://github.com/hashicorp/consul/issues/2914)]
* build: Consul is now built with Go 1.8.1. [[GH-2888](https://github.com/hashicorp/consul/issues/2888)]
* ui: Updates Consul assets to new branding. [[GH-2898](https://github.com/hashicorp/consul/issues/2898)]

BUG FIXES:

* api: Added missing Raft index fields to AgentService and Node structures. [[GH-2882](https://github.com/hashicorp/consul/issues/2882)]
* server: Fixed an issue where flood joins would not work with IPv6 addresses. [[GH-2878](https://github.com/hashicorp/consul/issues/2878)]
* server: Fixed an issue where electing a 0.8.x leader during an upgrade would cause a panic in older servers. [[GH-2889](https://github.com/hashicorp/consul/issues/2889)]
* server: Fixed an issue where tracking of leadership changes could become incorrect when changes occurred very rapidly. This could manifest as a panic in Autopilot, but could have caused other issues with multiple leader management routines running simultaneously. [[GH-2896](https://github.com/hashicorp/consul/issues/2896)]
* server: Fixed a panic when checking ACLs on a session that doesn't exist. [[GH-2624](https://github.com/hashicorp/consul/issues/2624)]

## 0.8.0 (April 5, 2017)

BREAKING CHANGES:

* **Command-Line Interface RPC Deprecation:** The RPC client interface has been removed. All CLI commands that used RPC and the `-rpc-addr` flag to communicate with Consul have been converted to use the HTTP API and the appropriate flags for it, and the `rpc` field has been removed from the port and address binding configs. You will need to remove these fields from your config files and update any scripts that passed a custom `-rpc-addr` to the following commands: `force-leave`, `info`,  `join`, `keyring`, `leave`, `members`, `monitor`, `reload`

* **Version 8 ACLs Are Now Opt-Out:** The [`acl_enforce_version_8`](https://www.consul.io/docs/agent/options.html#acl_enforce_version_8) configuration now defaults to `true` to enable [full version 8 ACL support](https://www.consul.io/docs/internals/acl.html#version_8_acls) by default. If you are upgrading an existing cluster with ACLs enabled, you will need to set this to `false` during the upgrade on **both Consul agents and Consul servers**. Version 8 ACLs were also changed so that [`acl_datacenter`](https://www.consul.io/docs/agent/options.html#acl_datacenter) must be set on agents in order to enable the agent-side enforcement of ACLs. This makes for a smoother experience in clusters where ACLs aren't enabled at all, but where the agents would have to wait to contact a Consul server before learning that. [[GH-2844](https://github.com/hashicorp/consul/issues/2844)]

* **Remote Exec Is Now Opt-In:** The default for [`disable_remote_exec`](https://www.consul.io/docs/agent/options.html#disable_remote_exec) was changed to "true", so now operators need to opt-in to having agents support running commands remotely via [`consul exec`](/docs/commands/exec.html). [[GH-2854](https://github.com/hashicorp/consul/issues/2854)]

* **Raft Protocol Compatibility:** When upgrading to Consul 0.8.0 from a version lower than 0.7.0, users will need to
set the [`-raft-protocol`](https://www.consul.io/docs/agent/options.html#_raft_protocol) option to 1 in order to maintain backwards compatibility with the old servers during the upgrade. See [Upgrading Specific Versions](https://www.consul.io/docs/upgrade-specific.html) guide for more details.

FEATURES:

* **Autopilot:** A set of features has been added to allow for automatic operator-friendly management of Consul servers. For more information about Autopilot, see the [Autopilot Guide](https://www.consul.io/docs/guides/autopilot.html).
  - **Dead Server Cleanup:** Dead servers will periodically be cleaned up and removed from the Raft peer set, to prevent them from interfering with the quorum size and leader elections.
  - **Server Health Checking:** An internal health check has been added to track the stability of servers. The thresholds of this health check are tunable as part of the [Autopilot configuration](https://www.consul.io/docs/agent/options.html#autopilot) and the status can be viewed through the [`/v1/operator/autopilot/health`](https://www.consul.io/docs/agent/http/operator.html#autopilot-health) HTTP endpoint.
  - **New Server Stabilization:** When a new server is added to the cluster, there will be a waiting period where it must be healthy and stable for a certain amount of time before being promoted to a full, voting member. This threshold can be configured using the new [`server_stabilization_time`](https://www.consul.io/docs/agent/options.html#server_stabilization_time) setting.
  - **Advanced Redundancy:** (Consul Enterprise) A new [`-non-voting-server`](https://www.consul.io/docs/agent/options.html#_non_voting_server) option flag has been added for Consul servers to configure a server that does not participate in the Raft quorum. This can be used to add read scalability to a cluster in cases where a high volume of reads to servers are needed, but non-voting servers can be lost without causing an outage. There's also a new [`redundancy_zone_tag`](https://www.consul.io/docs/agent/options.html#redundancy_zone_tag) configuration that allows Autopilot to manage separating servers into zones for redundancy. Only one server in each zone can be a voting member at one time. This helps when Consul servers are managed with automatic replacement with a system like a resource scheduler or auto-scaling group. Extra non-voting servers in each zone will be available as hot standbys (that help with read-scaling) that can be quickly promoted into service when the voting server in a zone fails.
  - **Upgrade Orchestration:** (Consul Enterprise) Autopilot will automatically orchestrate an upgrade strategy for Consul servers where it will initially add newer versions of Consul servers as non-voters, wait for a full set of newer versioned servers to be added, and then gradually swap into service as voters and swap out older versioned servers to non-voters. This allows operators to safely bring up new servers, wait for the upgrade to be complete, and then terminate the old servers.
* **Network Areas:** (Consul Enterprise) A new capability has been added which allows operators to define network areas that join together two Consul datacenters. Unlike Consul's WAN feature, network areas use just the server RPC port for communication, and pairwise relationships can be made between arbitrary datacenters, so not all servers need to be fully connected. This allows for complex topologies among Consul datacenters like hub/spoke and more general trees. See the [Network Areas Guide](https://www.consul.io/docs/guides/areas.html) for more details.
* **WAN Soft Fail:** Request routing between servers in the WAN is now more robust by treating Serf failures as advisory but not final. This means that if there are issues between some subset of the servers in the WAN, Consul will still be able to route RPC requests as long as RPCs are actually still working. Prior to WAN Soft Fail, any datacenters having connectivity problems on the WAN would mean that all DCs might potentially stop sending RPCs to those datacenters. [[GH-2801](https://github.com/hashicorp/consul/issues/2801)]
* **WAN Join Flooding:** A new routine was added that looks for Consul servers in the LAN and makes sure that they are joined into the WAN as well. This catches up up newly-added servers onto the WAN as soon as they join the LAN, keeping them in sync automatically. [[GH-2801](https://github.com/hashicorp/consul/issues/2801)]
* **Validate command:** To provide consistency across our products, the `configtest` command has been deprecated and replaced with the `validate` command (to match Nomad and Terraform). The `configtest` command will be removed in Consul 0.9. [[GH-2732](https://github.com/hashicorp/consul/issues/2732)]

IMPROVEMENTS:

* agent: Fixed a missing case where gossip would stop flowing to dead nodes for a short while. [[GH-2722](https://github.com/hashicorp/consul/issues/2722)]
* agent: Changed agent to seed Go's random number generator. [[GH-2722](https://github.com/hashicorp/consul/issues/2722)]
* agent: Serf snapshots no longer have the executable bit set on the file. [[GH-2722](https://github.com/hashicorp/consul/issues/2722)]
* agent: Consul is now built with Go 1.8. [[GH-2752](https://github.com/hashicorp/consul/issues/2752)]
* agent: Updated aws-sdk-go version (used for EC2 auto join) for Go 1.8 compatibility. [[GH-2755](https://github.com/hashicorp/consul/issues/2755)]
* agent: User-supplied node IDs are now normalized to lower-case. [[GH-2798](https://github.com/hashicorp/consul/issues/2798)]
* agent: Added checks to enforce uniqueness of agent node IDs at cluster join time and when registering with the catalog. [[GH-2832](https://github.com/hashicorp/consul/issues/2832)]
* cli: Standardized handling of CLI options for connecting to the Consul agent. This makes sure that the same set of flags and environment variables works in all CLI commands (see https://www.consul.io/docs/commands/index.html#environment-variables). [[GH-2717](https://github.com/hashicorp/consul/issues/2717)]
* cli: Updated go-cleanhttp library for better HTTP connection handling between CLI commands and the Consul agent (tunes reuse settings). [[GH-2735](https://github.com/hashicorp/consul/issues/2735)]
* cli: The `operator raft` subcommand has had its two modes split into the `list-peers` and `remove-peer` subcommands. The old flags for these will continue to work for backwards compatibility, but will be removed in Consul 0.9.
* cli: Added an `-id` flag to the `operator raft remove-peer` command to allow removing a peer by ID. [[GH-2847](https://github.com/hashicorp/consul/issues/2847)]
* dns: Allows the `.service` tag to be optional in RFC 2782 lookups. [[GH-2690](https://github.com/hashicorp/consul/issues/2690)]
* server: Changed the internal `EnsureRegistration` RPC endpoint to prevent registering checks that aren't associated with the top-level node being registered. [[GH-2846](https://github.com/hashicorp/consul/issues/2846)]

BUG FIXES:

* agent: Fixed an issue with `consul watch` not working when http was listening on a unix socket. [[GH-2385](https://github.com/hashicorp/consul/issues/2385)]
* agent: Fixed an issue where checks and services could not sync deregister operations back to the catalog when version 8 ACL support is enabled. [[GH-2818](https://github.com/hashicorp/consul/issues/2818)]
* agent: Fixed an issue where agents could use the ACL token registered with a service when registering checks for the same service that were registered with a different ACL token. [[GH-2829](https://github.com/hashicorp/consul/issues/2829)]
* cli: Fixed `consul kv` commands not reading the `CONSUL_HTTP_TOKEN` environment variable. [[GH-2566](https://github.com/hashicorp/consul/issues/2566)]
* cli: Fixed an issue where prefixing an address with a protocol (such as 'http://' or 'https://') in `-http-addr` or `CONSUL_HTTP_ADDR` would give an error.
* cli: Fixed an issue where error messages would get printed to stdout instead of stderr. [[GH-2548](https://github.com/hashicorp/consul/issues/2548)]
* server: Fixed an issue with version 8 ACLs where servers couldn't deregister nodes from the catalog during reconciliation. [[GH-2792](https://github.com/hashicorp/consul/issues/2792)] This fix was generalized and applied to registering nodes as well. [[GH-2826](https://github.com/hashicorp/consul/issues/2826)]
* server: Fixed an issue where servers could temporarily roll back changes to a node's metadata or tagged addresses when making updates to the node's health checks. [[GH-2826](https://github.com/hashicorp/consul/issues/2826)]
* server: Fixed an issue where the service name `consul` was not subject to service ACL policies with version 8 ACLs enabled. [[GH-2816](https://github.com/hashicorp/consul/issues/2816)]

## 0.7.5 (February 15, 2017)

BUG FIXES:

* server: Fixed a rare but serious issue where Consul servers could panic when performing a large delete operation followed by a specific sequence of other updates to related parts of the state store (affects KV, sessions, prepared queries, and the catalog). [[GH-2724](https://github.com/hashicorp/consul/issues/2724)]

## 0.7.4 (February 6, 2017)

IMPROVEMENTS:

* agent: Integrated gopsutil library to use built in host UUID as node ID, if available, instead of a randomly generated UUID. This makes it easier for other applications on the same host to generate the same node ID without coordinating with Consul. [[GH-2697](https://github.com/hashicorp/consul/issues/2697)]
* agent: Added a configuration option, `tls_min_version`, for setting the minimum allowed TLS version used for the HTTP API and RPC. [[GH-2699](https://github.com/hashicorp/consul/issues/2699)]
* agent: Added a `relay-factor` option to keyring operations to allow nodes to relay their response through N randomly-chosen other nodes in the cluster. [[GH-2704](https://github.com/hashicorp/consul/issues/2704)]
* build: Consul is now built with Go 1.7.5. [[GH-2682](https://github.com/hashicorp/consul/issues/2682)]
* dns: Add ability to lookup Consul agents by either their Node ID or Node Name through the node interface (e.g. DNS `(node-id|node-name).node.consul`). [[GH-2702](https://github.com/hashicorp/consul/issues/2702)]

BUG FIXES:

* dns: Fixed an issue where SRV lookups for services on a node registered with non-IP addresses were missing the CNAME record in the additional section of the response. [[GH-2695](https://github.com/hashicorp/consul/issues/2695)]

## 0.7.3 (January 26, 2017)

FEATURES:

* **KV Import/Export CLI:** `consul kv export` and `consul kv import` can be used to move parts of the KV tree between disconnected consul clusters, using JSON as the intermediate representation. [[GH-2633](https://github.com/hashicorp/consul/issues/2633)]
* **Node Metadata:** Support for assigning user-defined metadata key/value pairs to nodes has been added. This can be viewed when looking up node info, and can be used to filter the results of various catalog and health endpoints. For more information, see the [Catalog](https://www.consul.io/docs/agent/http/catalog.html), [Health](https://www.consul.io/docs/agent/http/health.html), and [Prepared Query](https://www.consul.io/docs/agent/http/query.html) endpoint documentation, as well as the [Node Meta](https://www.consul.io/docs/agent/options.html#_node_meta) section of the agent configuration. [[GH-2654](https://github.com/hashicorp/consul/issues/2654)]
* **Node Identifiers:** Consul agents can now be configured with a unique identifier, or they will generate one at startup that will persist across agent restarts. This identifier is designed to represent a node across all time, even if the name or address of the node changes. Identifiers are currently only exposed in node-related endpoints, but they will be used in future versions of Consul to help manage Consul servers and the Raft quorum in a more robust manner, as the quorum is currently tracked via addresses, which can change. [[GH-2661](https://github.com/hashicorp/consul/issues/2661)]
* **Improved Blocking Queries:** Consul's [blocking query](https://www.consul.io/api/index.html#blocking-queries) implementation was improved to provide a much more fine-grained mechanism for detecting changes. For example, in previous versions of Consul blocking to wait on a change to a specific service would result in a wake up if any service changed. Now, wake ups are scoped to the specific service being watched, if possible. This support has been added to all endpoints that support blocking queries, nothing new is required to take advantage of this feature. [[GH-2671](https://github.com/hashicorp/consul/issues/2671)]
* **GCE auto-discovery:** New `-retry-join-gce` configuration options added to allow bootstrapping by automatically discovering Google Cloud instances with a given tag at startup. [[GH-2570](https://github.com/hashicorp/consul/issues/2570)]

IMPROVEMENTS:

* build: Consul is now built with Go 1.7.4. [[GH-2676](https://github.com/hashicorp/consul/issues/2676)]
* cli: `consul kv get` now has a `-base64` flag to base 64 encode the value. [[GH-2631](https://github.com/hashicorp/consul/issues/2631)]
* cli: `consul kv put` now has a `-base64` flag for setting values which are base 64 encoded. [[GH-2632](https://github.com/hashicorp/consul/issues/2632)]
* ui: Added a notice that JS is required when viewing the web UI with JS disabled. [[GH-2636](https://github.com/hashicorp/consul/issues/2636)]

BUG FIXES:

* agent: Redacted the AWS access key and secret key ID from the /v1/agent/self output so they are not disclosed. [[GH-2677](https://github.com/hashicorp/consul/issues/2677)]
* agent: Fixed a rare startup panic due to a Raft/Serf race condition. [[GH-1899](https://github.com/hashicorp/consul/issues/1899)]
* cli: Fixed a panic when an empty quoted argument was given to `consul kv put`. [[GH-2635](https://github.com/hashicorp/consul/issues/2635)]
* tests: Fixed a race condition with check mock's map usage. [[GH-2578](https://github.com/hashicorp/consul/issues/2578)]

## 0.7.2 (December 19, 2016)

FEATURES:

* **Keyring API:** A new `/v1/operator/keyring` HTTP endpoint was added that allows for performing operations such as list, install, use, and remove on the encryption keys in the gossip keyring. See the [Keyring Endpoint](https://www.consul.io/docs/agent/http/operator.html#keyring) for more details. [[GH-2509](https://github.com/hashicorp/consul/issues/2509)]
* **Monitor API:** A new `/v1/agent/monitor` HTTP endpoint was added to allow for viewing streaming log output from the agent, similar to the `consul monitor` command. See the [Monitor Endpoint](https://www.consul.io/docs/agent/http/agent.html#agent_monitor) for more details. [[GH-2511](https://github.com/hashicorp/consul/issues/2511)]
* **Reload API:** A new `/v1/agent/reload` HTTP endpoint was added for triggering a reload of the agent's configuration. See the [Reload Endpoint](https://www.consul.io/docs/agent/http/agent.html#agent_reload) for more details. [[GH-2516](https://github.com/hashicorp/consul/issues/2516)]
* **Leave API:** A new `/v1/agent/leave` HTTP endpoint was added for causing an agent to gracefully shutdown and leave the cluster (previously, only `force-leave` was present in the HTTP API). See the [Leave Endpoint](https://www.consul.io/docs/agent/http/agent.html#agent_leave) for more details. [[GH-2516](https://github.com/hashicorp/consul/issues/2516)]
* **Bind Address Templates (beta):** Consul agents now allow [go-sockaddr/template](https://godoc.org/github.com/hashicorp/go-sockaddr/template) syntax to be used for any bind address configuration (`advertise_addr`, `bind_addr`, `client_addr`, and others). This allows for easy creation of immutable images for Consul that can fetch their own address based on an interface name, network CIDR, address family from an actual RFC number, and many other possible schemes. This feature is in beta and we may tweak the template syntax before final release, but we encourage the community to try this and provide feedback. [[GH-2563](https://github.com/hashicorp/consul/issues/2563)]
* **Complete ACL Coverage (beta):** Consul 0.8 will feature complete ACL coverage for all of Consul. To ease the transition to the new policies, a beta version of complete ACL support was added to help with testing and migration to the new features. Please see the [ACLs Internals Guide](https://www.consul.io/docs/internals/acl.html#version_8_acls) for more details. [GH-2594, GH-2592, GH-2590]

IMPROVEMENTS:

* agent: Defaults to `?pretty` JSON for HTTP API requests when in `-dev` mode. [[GH-2518](https://github.com/hashicorp/consul/issues/2518)]
* agent: Updated Circonus metrics library and added new Circonus configration options for Consul for customizing check display name and tags. [[GH-2555](https://github.com/hashicorp/consul/issues/2555)]
* agent: Added a checksum to UDP gossip messages to guard against packet corruption. [[GH-2574](https://github.com/hashicorp/consul/issues/2574)]
* agent: Check whether a snapshot needs to be taken more often (every 5 seconds instead of 2 minutes) to keep the raft file smaller and to avoid doing huge truncations when writing lots of entries very quickly. [[GH-2591](https://github.com/hashicorp/consul/issues/2591)]
* agent: Allow gossiping to suspected/recently dead nodes. [[GH-2593](https://github.com/hashicorp/consul/issues/2593)]
* agent: Changed the gossip suspicion timeout to grow smoothly as the number of nodes grows. [[GH-2593](https://github.com/hashicorp/consul/issues/2593)]
* agent: Added a deprecation notice for Atlas features to the CLI and docs. [[GH-2597](https://github.com/hashicorp/consul/issues/2597)]
* agent: Give a better error message when the given data-dir is not a directory. [[GH-2529](https://github.com/hashicorp/consul/issues/2529)]

BUG FIXES:

* agent: Fixed a panic when SIGPIPE signal was received. [[GH-2404](https://github.com/hashicorp/consul/issues/2404)]
* api: Added missing Raft index fields to `CatalogService` structure. [[GH-2366](https://github.com/hashicorp/consul/issues/2366)]
* api: Added missing notes field to `AgentServiceCheck` structure. [[GH-2336](https://github.com/hashicorp/consul/issues/2336)]
* api: Changed type of `AgentServiceCheck.TLSSkipVerify` from `string` to `bool`. [[GH-2530](https://github.com/hashicorp/consul/issues/2530)]
* api: Added new `HealthChecks.AggregatedStatus()` method that makes it easy get an overall health status from a list of checks. [[GH-2544](https://github.com/hashicorp/consul/issues/2544)]
* api: Changed type of `KVTxnOp.Verb` from `string` to `KVOp`. [[GH-2531](https://github.com/hashicorp/consul/issues/2531)]
* cli: Fixed an issue with the `consul kv put` command where a negative value would be interpreted as an argument to read from standard input. [[GH-2526](https://github.com/hashicorp/consul/issues/2526)]
* ui: Fixed an issue where extra commas would be shown around service tags. [[GH-2340](https://github.com/hashicorp/consul/issues/2340)]
* ui: Customized Bootstrap config to avoid missing font file references. [[GH-2485](https://github.com/hashicorp/consul/issues/2485)]
* ui: Removed "Deregister" button as removing nodes from the catalog isn't a common operation and leads to lots of user confusion. [[GH-2541](https://github.com/hashicorp/consul/issues/2541)]

## 0.7.1 (November 10, 2016)

BREAKING CHANGES:

* Child process reaping support has been removed, along with the `reap` configuration option. Reaping is also done via [dumb-init](https://github.com/Yelp/dumb-init) in the [Consul Docker image](https://github.com/hashicorp/docker-consul), so removing it from Consul itself simplifies the code and eases future maintainence for Consul. If you are running Consul as PID 1 in a container you will need to arrange for a wrapper process to reap child processes. [[GH-1988](https://github.com/hashicorp/consul/issues/1988)]
* The default for `max_stale` has been increased to a near-indefinite threshold (10 years) to allow DNS queries to continue to be served in the event of a long outage with no leader. A new telemetry counter has also been added at `consul.dns.stale_queries` to track when agents serve DNS queries that are over a certain staleness (>5 seconds). [[GH-2481](https://github.com/hashicorp/consul/issues/2481)]
* The api package's `PreparedQuery.Delete()` method now takes `WriteOptions` instead of `QueryOptions`. [[GH-2417](https://github.com/hashicorp/consul/issues/2417)]

FEATURES:

* **Key/Value Store Command Line Interface:** New `consul kv` commands were added for easy access to all basic key/value store operations. [[GH-2360](https://github.com/hashicorp/consul/issues/2360)]
* **Snapshot/Restore:** A new /v1/snapshot HTTP endpoint and corresponding set of `consul snapshot` commands were added for easy point-in-time snapshots for disaster recovery. Snapshots include all state managed by Consul's Raft [consensus protocol](/docs/internals/consensus.html), including Key/Value Entries, Service Catalog, Prepared Queries, Sessions, and ACLs. Snapshots can be restored on the fly into a completely fresh cluster. [[GH-2396](https://github.com/hashicorp/consul/issues/2396)]
* **AWS auto-discovery:** New `-retry-join-ec2` configuration options added to allow bootstrapping by automatically discovering AWS instances with a given tag key/value at startup. [[GH-2459](https://github.com/hashicorp/consul/issues/2459)]

IMPROVEMENTS:

* api: All session options can now be set when using `api.Lock()`. [[GH-2372](https://github.com/hashicorp/consul/issues/2372)]
* agent: Added the ability to bind Serf WAN and LAN to different interfaces than the general bind address. [[GH-2007](https://github.com/hashicorp/consul/issues/2007)]
* agent: Added a new `tls_skip_verify` configuration option for HTTP checks. [[GH-1984](https://github.com/hashicorp/consul/issues/1984)]
* build: Consul is now built with Go 1.7.3. [[GH-2281](https://github.com/hashicorp/consul/issues/2281)]

BUG FIXES:

* agent: Fixed a Go race issue with log buffering at startup. [[GH-2262](https://github.com/hashicorp/consul/issues/2262)]
* agent: Fixed a panic during anti-entropy sync for services and checks. [[GH-2125](https://github.com/hashicorp/consul/issues/2125)]
* agent: Fixed an issue on Windows where "wsarecv" errors were logged when CLI commands accessed the RPC interface. [[GH-2356](https://github.com/hashicorp/consul/issues/2356)]
* agent: Syslog initialization will now retry on errors for up to 60 seconds to avoid a race condition at system startup. [[GH-1610](https://github.com/hashicorp/consul/issues/1610)]
* agent: Fixed a panic when both -dev and -bootstrap-expect flags were provided. [[GH-2464](https://github.com/hashicorp/consul/issues/2464)]
* agent: Added a retry with backoff when a session fails to invalidate after expiring. [[GH-2435](https://github.com/hashicorp/consul/issues/2435)]
* agent: Fixed an issue where Consul would fail to start because of leftover malformed check/service state files. [[GH-1221](https://github.com/hashicorp/consul/issues/1221)]
* agent: Fixed agent crashes on macOS Sierra by upgrading Go. [GH-2407, GH-2281]
* agent: Log a warning instead of success when attempting to deregister a nonexistent service. [[GH-2492](https://github.com/hashicorp/consul/issues/2492)]
* api: Trim leading slashes from keys/prefixes when querying KV endpoints to avoid a bug with redirects in Go 1.7 (golang/go#4800). [[GH-2403](https://github.com/hashicorp/consul/issues/2403)]
* dns: Fixed external services that pointed to consul addresses (CNAME records) not resolving to A-records. [[GH-1228](https://github.com/hashicorp/consul/issues/1228)]
* dns: Fixed an issue with SRV lookups where the service address was different from the node's. [[GH-832](https://github.com/hashicorp/consul/issues/832)]
* dns: Fixed an issue where truncated records from a recursor query were improperly reported as errors. [[GH-2384](https://github.com/hashicorp/consul/issues/2384)]
* server: Fixed the port numbers in the sample JSON inside peers.info. [[GH-2391](https://github.com/hashicorp/consul/issues/2391)]
* server: Squashes ACL datacenter name to lower case and checks for proper formatting at startup. [GH-2059, GH-1778, GH-2478]
* ui: Fixed an XSS issue with the display of sessions and ACLs in the web UI. [[GH-2456](https://github.com/hashicorp/consul/issues/2456)]

## 0.7.0 (September 14, 2016)

BREAKING CHANGES:

* The default behavior of `leave_on_terminate` and `skip_leave_on_interrupt` are now dependent on whether or not the agent is acting as a server or client. When Consul is started as a server the defaults for these are `false` and `true`, respectively, which means that you have to explicitly configure a server to leave the cluster. When Consul is started as a client the defaults are the opposite, which means by default, clients will leave the cluster if shutdown or interrupted. [[GH-1909](https://github.com/hashicorp/consul/issues/1909)] [[GH-2320](https://github.com/hashicorp/consul/issues/2320)]
* The `allow_stale` configuration for DNS queries to the Consul agent now defaults to `true`, allowing for better utilization of available Consul servers and higher throughput at the expense of weaker consistency. This is almost always an acceptable tradeoff for DNS queries, but this can be reconfigured to use the old default behavior if desired. [[GH-2315](https://github.com/hashicorp/consul/issues/2315)]
* Output from HTTP checks is truncated to 4k when stored on the servers, similar to script check output. [[GH-1952](https://github.com/hashicorp/consul/issues/1952)]
* Consul's Go API client will now send ACL tokens using HTTP headers instead of query parameters, requiring Consul 0.6.0 or later. [[GH-2233](https://github.com/hashicorp/consul/issues/2233)]
* Removed support for protocol version 1, so Consul 0.7 is no longer compatible with Consul versions prior to 0.3. [[GH-2259](https://github.com/hashicorp/consul/issues/2259)]
* The Raft peers information in `consul info` has changed format and includes information about the suffrage of a server, which will be used in future versions of Consul. [[GH-2222](https://github.com/hashicorp/consul/issues/2222)]
* New [`translate_wan_addrs`](https://www.consul.io/docs/agent/options.html#translate_wan_addrs) behavior from [[GH-2118](https://github.com/hashicorp/consul/issues/2118)] translates addresses in HTTP responses and could break clients that are expecting local addresses. A new `X-Consul-Translate-Addresses` header was added to allow clients to detect if translation is enabled for HTTP responses, and a "lan" tag was added to `TaggedAddresses` for clients that need the local address regardless of translation. [[GH-2280](https://github.com/hashicorp/consul/issues/2280)]
* The behavior of the `peers.json` file is different in this version of Consul. This file won't normally be present and is used only during outage recovery. Be sure to read the updated [Outage Recovery Guide](https://www.consul.io/docs/guides/outage.html) for details. [[GH-2222](https://github.com/hashicorp/consul/issues/2222)]
* Consul's default Raft timing is now set to work more reliably on lower-performance servers, which allows small clusters to use lower cost compute at the expense of reduced performance for failed leader detection and leader elections. You will need to configure Consul to get the same performance as before. See the new [Server Performance](https://www.consul.io/docs/guides/performance.html) guide for more details. [[GH-2303](https://github.com/hashicorp/consul/issues/2303)]

FEATURES:

* **Transactional Key/Value API:** A new `/v1/txn` API was added that allows for atomic updates to and fetches from multiple entries in the key/value store inside of an atomic transaction. This includes conditional updates based on obtaining locks, and all other key/value store operations. See the [Key/Value Store Endpoint](https://www.consul.io/docs/agent/http/kv.html#txn) for more details. [[GH-2028](https://github.com/hashicorp/consul/issues/2028)]
* **Native ACL Replication:** Added a built-in full replication capability for ACLs. Non-ACL datacenters can now replicate the complete ACL set locally to their state store and fall back to that if there's an outage. Additionally, this provides a good way to make a backup ACL datacenter, or to migrate the ACL datacenter to a different one. See the [ACL Internals Guide](https://www.consul.io/docs/internals/acl.html#replication) for more details. [[GH-2237](https://github.com/hashicorp/consul/issues/2237)]
* **Server Connection Rebalancing:** Consul agents will now periodically reconnect to available Consul servers in order to redistribute their RPC query load. Consul clients will, by default, attempt to establish a new connection every 120s to 180s unless the size of the cluster is sufficiently large. The rate at which agents begin to query new servers is proportional to the size of the Consul cluster (servers should never receive more than 64 new connections per second per Consul server as a result of rebalancing). Clusters in stable environments who use `allow_stale` should see a more even distribution of query load across all of their Consul servers. [[GH-1743](https://github.com/hashicorp/consul/issues/1743)]
* **Raft Updates and Consul Operator Interface:** This version of Consul upgrades to "stage one" of the v2 HashiCorp Raft library. This version offers improved handling of cluster membership changes and recovery after a loss of quorum. This version also provides a foundation for new features that will appear in future Consul versions once the remainder of the v2 library is complete. [[GH-2222](https://github.com/hashicorp/consul/issues/2222)] <br> Consul's default Raft timing is now set to work more reliably on lower-performance servers, which allows small clusters to use lower cost compute at the expense of reduced performance for failed leader detection and leader elections. You will need to configure Consul to get the same performance as before. See the new [Server Performance](https://www.consul.io/docs/guides/performance.html) guide for more details. [[GH-2303](https://github.com/hashicorp/consul/issues/2303)] <br> Servers will now abort bootstrapping if they detect an existing cluster with configured Raft peers. This will help prevent safe but spurious leader elections when introducing new nodes with `bootstrap_expect` enabled into an existing cluster. [[GH-2319](https://github.com/hashicorp/consul/issues/2319)] <br> Added new `consul operator` command, HTTP endpoint, and associated ACL to allow Consul operators to view and update the Raft configuration. This allows a stale server to be removed from the Raft peers without requiring downtime and peers.json recovery file use. See the new [Consul Operator Command](https://www.consul.io/docs/commands/operator.html) and the [Consul Operator Endpoint](https://www.consul.io/docs/agent/http/operator.html) for details, as well as the updated [Outage Recovery Guide](https://www.consul.io/docs/guides/outage.html). [[GH-2312](https://github.com/hashicorp/consul/issues/2312)]
* **Serf Lifeguard Updates:** Implemented a new set of feedback controls for the gossip layer that help prevent degraded nodes that can't meet the soft real-time requirements from erroneously causing `serfHealth` flapping in other, healthy nodes. This feature tunes itself automatically and requires no configuration. [[GH-2101](https://github.com/hashicorp/consul/issues/2101)]
* **Prepared Query Near Parameter:** Prepared queries support baking in a new `Near` sorting parameter. This allows results to be sorted by network round trip time based on a static node, or based on the round trip time from the Consul agent where the request originated. This can be used to find a co-located service instance is one is available, with a transparent fallback to the next best alternate instance otherwise. [[GH-2137](https://github.com/hashicorp/consul/issues/2137)]
* **Automatic Service Deregistration:** Added a new `deregister_critical_service_after` timeout field for health checks which will cause the service associated with that check to get deregistered if the check is critical for longer than the timeout. This is useful for cleanup of health checks registered natively by applications, or in other situations where services may not always be cleanly shutdown. [[GH-679](https://github.com/hashicorp/consul/issues/679)]
* **WAN Address Translation Everywhere:** Extended the [`translate_wan_addrs`](https://www.consul.io/docs/agent/options.html#translate_wan_addrs) config option to also translate node addresses in HTTP responses, making it easy to use this feature from non-DNS clients. [[GH-2118](https://github.com/hashicorp/consul/issues/2118)]
* **RPC Retries:** Consul will now retry RPC calls that result in "no leader" errors for up to 5 seconds. This allows agents to ride out leader elections with a delayed response vs. an error. [[GH-2175](https://github.com/hashicorp/consul/issues/2175)]
* **Circonus Telemetry Support:** Added support for Circonus as a telemetry destination. [[GH-2193](https://github.com/hashicorp/consul/issues/2193)]

IMPROVEMENTS:

* agent: Reap time for failed nodes is now configurable via new `reconnect_timeout` and `reconnect_timeout_wan` config options ([use with caution](https://www.consul.io/docs/agent/options.html#reconnect_timeout)). [[GH-1935](https://github.com/hashicorp/consul/issues/1935)]
* agent: Joins based on a DNS lookup will use TCP and attempt to join with the full list of returned addresses. [[GH-2101](https://github.com/hashicorp/consul/issues/2101)]
* agent: Consul will now refuse to start with a helpful message if the same UNIX socket is used for more than one listening endpoint. [[GH-1910](https://github.com/hashicorp/consul/issues/1910)]
* agent: Removed an obsolete warning message when Consul starts on Windows. [[GH-1920](https://github.com/hashicorp/consul/issues/1920)]
* agent: Defaults bind address to 127.0.0.1 when running in `-dev` mode. [[GH-1878](https://github.com/hashicorp/consul/issues/1878)]
* agent: Added version information to the log when Consul starts up. [[GH-1404](https://github.com/hashicorp/consul/issues/1404)]
* agent: Added timing metrics for HTTP requests in the form of `consul.http.<verb>.<path>`. [[GH-2256](https://github.com/hashicorp/consul/issues/2256)]
* build: Updated all vendored dependencies. [[GH-2258](https://github.com/hashicorp/consul/issues/2258)]
* build: Consul releases are now built with Go 1.6.3. [[GH-2260](https://github.com/hashicorp/consul/issues/2260)]
* checks: Script checks now support an optional `timeout` parameter. [[GH-1762](https://github.com/hashicorp/consul/issues/1762)]
* checks: HTTP health checks limit saved output to 4K to avoid performance issues. [[GH-1952](https://github.com/hashicorp/consul/issues/1952)]
* cli: Added a `-stale` mode for watchers to allow them to pull data from any Consul server, not just the leader. [[GH-2045](https://github.com/hashicorp/consul/issues/2045)] [[GH-917](https://github.com/hashicorp/consul/issues/917)]
* dns: Consul agents can now limit the number of UDP answers returned via the DNS interface. The default number of UDP answers is `3`, however by adjusting the `dns_config.udp_answer_limit` configuration parameter, it is now possible to limit the results down to `1`. This tunable provides environments where RFC3484 section 6, rule 9 is enforced with an important workaround in order to preserve the desired behavior of randomized DNS results. Most modern environments will not need to adjust this setting as this RFC was made obsolete by RFC 6724\. See the [agent options](https://www.consul.io/docs/agent/options.html#udp_answer_limit) documentation for additional details for when this should be used. [[GH-1712](https://github.com/hashicorp/consul/issues/1712)]
* dns: Consul now compresses all DNS responses by default. This prevents issues when recursing records that were originally compressed, where Consul would sometimes generate an invalid, uncompressed response that was too large. [[GH-2266](https://github.com/hashicorp/consul/issues/2266)]
* dns: Added a new `recursor_timeout` configuration option to set the timeout for Consul's internal DNS client that's used for recursing queries to upstream DNS servers. [[GH-2321](https://github.com/hashicorp/consul/issues/2321)]
* dns: Added a new `-dns-port` command line option so this can be set without a config file. [[GH-2263](https://github.com/hashicorp/consul/issues/2263)]
* ui: Added a new network tomography visualization to the UI. [[GH-2046](https://github.com/hashicorp/consul/issues/2046)]

BUG FIXES:

* agent: Fixed an issue where a health check's output never updates if the check status doesn't change after the Consul agent starts. [[GH-1934](https://github.com/hashicorp/consul/issues/1934)]
* agent: External services can now be registered with ACL tokens. [[GH-1738](https://github.com/hashicorp/consul/issues/1738)]
* agent: Fixed an issue where large events affecting many nodes could cause infinite intent rebroadcasts, leading to many log messages about intent queue overflows. [[GH-1062](https://github.com/hashicorp/consul/issues/1062)]
* agent: Gossip encryption keys are now validated before being made persistent in the keyring, avoiding delayed feedback at runtime. [[GH-1299](https://github.com/hashicorp/consul/issues/1299)]
* dns: Fixed an issue where DNS requests for SRV records could be incorrectly trimmed, resulting in an ADDITIONAL section that was out of sync with the ANSWER. [[GH-1931](https://github.com/hashicorp/consul/issues/1931)]
* dns: Fixed two issues where DNS requests for SRV records on a prepared query that failed over would report the wrong domain and fail to translate addresses. [[GH-2218](https://github.com/hashicorp/consul/issues/2218)] [[GH-2220](https://github.com/hashicorp/consul/issues/2220)]
* server: Fixed a deadlock related to sorting the list of available datacenters by round trip time. [[GH-2130](https://github.com/hashicorp/consul/issues/2130)]
* server: Fixed an issue with the state store's immutable radix tree that would prevent it from using cached modified objects during transactions, leading to extra copies and increased memory / GC pressure. [[GH-2106](https://github.com/hashicorp/consul/issues/2106)]
* server: Upgraded Bolt DB to v1.2.1 to fix an issue on Windows where Consul would sometimes fail to start due to open user-mapped sections. [[GH-2203](https://github.com/hashicorp/consul/issues/2203)]

OTHER CHANGES:

* build: Switched from Godep to govendor. [[GH-2252](https://github.com/hashicorp/consul/issues/2252)]

## 0.6.4 (March 16, 2016)

BACKWARDS INCOMPATIBILITIES:

* Added a new `query` ACL type to manage prepared query names, and stopped capturing
  ACL tokens by default when prepared queries are created. This won't affect existing
  queries and how they are executed, but this will affect how they are managed. Now
  management of prepared queries can be delegated within an organization. If you use
  prepared queries, you'll need to read the
  [Consul 0.6.4 upgrade instructions](https://www.consul.io/docs/upgrade-specific.html)
  before upgrading to this version of Consul. [[GH-1748](https://github.com/hashicorp/consul/issues/1748)]
* Consul's Go API client now pools connections by default, and requires you to manually
  opt-out of this behavior. Previously, idle connections were supported and their
  lifetime was managed by a finalizer, but this wasn't reliable in certain situations.
  If you reuse an API client object during the lifetime of your application, then there's
  nothing to do. If you have short-lived API client objects, you may need to configure them
  using the new `api.DefaultNonPooledConfig()` method to avoid leaking idle connections. [[GH-1825](https://github.com/hashicorp/consul/issues/1825)]
* Consul's Go API client's `agent.UpdateTTL()` function was updated in a way that will
  only work with Consul 0.6.4 and later. The `agent.PassTTL()`, `agent.WarnTTL()`, and
  `agent.FailTTL()` functions were not affected and will continue work with older
  versions of Consul. [[GH-1794](https://github.com/hashicorp/consul/issues/1794)]

FEATURES:

* Added new template prepared queries which allow you to define a prefix (possibly even
  an empty prefix) to apply prepared query features like datacenter failover to multiple
  services with a single query definition. This makes it easy to apply a common policy to
  multiple services without having to manage many prepared queries. See
  [Prepared Query Templates](https://www.consul.io/docs/agent/http/query.html#templates)
  for more details. [[GH-1764](https://github.com/hashicorp/consul/issues/1764)]
* Added a new ability to translate address lookups when doing queries of nodes in
  remote datacenters via DNS using a new `translate_wan_addrs` configuration
  option. This allows the node to be reached within its own datacenter using its
  local address, and reached from other datacenters using its WAN address, which is
  useful in hybrid setups with mixed networks. [[GH-1698](https://github.com/hashicorp/consul/issues/1698)]

IMPROVEMENTS:

* Added a new `disable_hostname` configuration option to control whether Consul's
  runtime telemetry gets prepended with the host name. All of the telemetry
  configuration has also been moved to a `telemetry` nested structure, but the old
  format is currently still supported. [[GH-1284](https://github.com/hashicorp/consul/issues/1284)]
* Consul's Go dependencies are now vendored using Godep. [[GH-1714](https://github.com/hashicorp/consul/issues/1714)]
* Added support for `EnableTagOverride` for the catalog in the Go API client. [[GH-1726](https://github.com/hashicorp/consul/issues/1726)]
* Consul now ships built from Go 1.6. [[GH-1735](https://github.com/hashicorp/consul/issues/1735)]
* Added a new `/v1/agent/check/update/<check id>` API for updating TTL checks which
  makes it easier to send large check output as part of a PUT body and not a query
  parameter. [[GH-1785](https://github.com/hashicorp/consul/issues/1785)].
* Added a default set of `Accept` headers for HTTP checks. [[GH-1819](https://github.com/hashicorp/consul/issues/1819)]
* Added support for RHEL7/Systemd in Terraform example. [[GH-1629](https://github.com/hashicorp/consul/issues/1629)]

BUG FIXES:

* Updated the internal web UI (`-ui` option) to latest released build, fixing
  an ACL-related issue and the broken settings icon. [[GH-1619](https://github.com/hashicorp/consul/issues/1619)]
* Fixed an issue where blocking KV reads could miss updates and return stale data
  when another key whose name is a prefix of the watched key was updated. [[GH-1632](https://github.com/hashicorp/consul/issues/1632)]
* Fixed the redirect from `/` to `/ui` when the internal web UI (`-ui` option) is
  enabled. [[GH-1713](https://github.com/hashicorp/consul/issues/1713)]
* Updated memberlist to pull in a fix for leaking goroutines when performing TCP
  fallback pings. This affected users with frequent UDP connectivity problems. [[GH-1802](https://github.com/hashicorp/consul/issues/1802)]
* Added a fix to trim UDP DNS responses so they don't exceed 512 bytes. [[GH-1813](https://github.com/hashicorp/consul/issues/1813)]
* Updated go-dockerclient to fix Docker health checks with Docker 1.10. [[GH-1706](https://github.com/hashicorp/consul/issues/1706)]
* Removed fixed height display of nodes and services in UI, leading to broken displays
  when a node has a lot of services. [[GH-2055](https://github.com/hashicorp/consul/issues/2055)]

## 0.6.3 (January 15, 2016)

BUG FIXES:

* Fixed an issue when running Consul as PID 1 in a Docker container where
  it could consume CPU and show spurious failures for health checks, watch
  handlers, and `consul exec` commands [[GH-1592](https://github.com/hashicorp/consul/issues/1592)]

## 0.6.2 (January 13, 2016)

SECURITY:

* Build against Go 1.5.3 to mitigate a security vulnerability introduced
  in Go 1.5. For more information, please see https://groups.google.com/forum/#!topic/golang-dev/MEATuOi_ei4

This is a security-only release; other than the version number and building
against Go 1.5.3, there are no changes from 0.6.1.

## 0.6.1 (January 6, 2016)

BACKWARDS INCOMPATIBILITIES:

* The new `-monitor-retry` option to `consul lock` defaults to 3. This
  will cause the lock monitor to retry up to 3 times, waiting 1s between
  each attempt if it gets a 500 error from the Consul servers. For the
  vast majority of use cases this is desirable to prevent the lock from
  being given up during a brief period of Consul unavailability. If you
  want to get the previous default behavior you will need to set the
  `-monitor-retry=0` option.

IMPROVEMENTS:

* Consul is now built with Go 1.5.2
* Added source IP address and port information to RPC-related log error
  messages and HTTP access logs [[GH-1513](https://github.com/hashicorp/consul/issues/1513)] [[GH-1448](https://github.com/hashicorp/consul/issues/1448)]
* API clients configured for insecure SSL now use an HTTP transport that's
  set up the same way as the Go default transport [[GH-1526](https://github.com/hashicorp/consul/issues/1526)]
* Added new per-host telemetry on DNS requests [[GH-1537](https://github.com/hashicorp/consul/issues/1537)]
* Added support for reaping child processes which is useful when running
  Consul as PID 1 in Docker containers [[GH-1539](https://github.com/hashicorp/consul/issues/1539)]
* Added new `-ui` command line and `ui` config option that enables a built-in
  Consul web UI, making deployment much simpler [[GH-1543](https://github.com/hashicorp/consul/issues/1543)]
* Added new `-dev` command line option that creates a completely in-memory
  standalone Consul server for development
* Added a Solaris build, now that dependencies have been updated to support
  it [[GH-1568](https://github.com/hashicorp/consul/issues/1568)]
* Added new `-try` option to `consul lock` to allow it to timeout with an error
  if it doesn't acquire the lock [[GH-1567](https://github.com/hashicorp/consul/issues/1567)]
* Added a new `-monitor-retry` option to `consul lock` to help ride out brief
  periods of Consul unavailabily without causing the lock to be given up [[GH-1567](https://github.com/hashicorp/consul/issues/1567)]

BUG FIXES:

* Fixed broken settings icon in web UI [[GH-1469](https://github.com/hashicorp/consul/issues/1469)]
* Fixed a web UI bug where the supplied token wasn't being passed into
  the internal endpoint, breaking some pages when multiple datacenters
  were present [[GH-1071](https://github.com/hashicorp/consul/issues/1071)]

## 0.6.0 (December 3, 2015)

BACKWARDS INCOMPATIBILITIES:

* A KV lock acquisition operation will now allow the lock holder to
  update the key's contents without giving up the lock by doing another
  PUT with `?acquire=<session>` and providing the same session that
  is holding the lock. Previously, this operation would fail.

FEATURES:

* Service ACLs now apply to service discovery [[GH-1024](https://github.com/hashicorp/consul/issues/1024)]
* Added event ACLs to guard firing user events [[GH-1046](https://github.com/hashicorp/consul/issues/1046)]
* Added keyring ACLs for gossip encryption keyring operations [[GH-1090](https://github.com/hashicorp/consul/issues/1090)]
* Added a new TCP check type that does a connect as a check [[GH-1130](https://github.com/hashicorp/consul/issues/1130)]
* Added new "tag override" feature that lets catalog updates to a
  service's tags flow down to agents [[GH-1187](https://github.com/hashicorp/consul/issues/1187)]
* Ported in-memory database from LMDB to an immutable radix tree to improve
  read throughput, reduce garbage collection pressure, and make Consul 100%
  pure Go [[GH-1291](https://github.com/hashicorp/consul/issues/1291)]
* Added support for sending telemetry to DogStatsD [[GH-1293](https://github.com/hashicorp/consul/issues/1293)]
* Added new network tomography subsystem that estimates the network
  round trip times between nodes and exposes that in raw APIs, as well
  as in existing APIs (find the service node nearest node X); also
  includes a new `consul rtt` command to query interactively [[GH-1331](https://github.com/hashicorp/consul/issues/1331)]
* Consul now builds under Go 1.5.1 by default [[GH-1345](https://github.com/hashicorp/consul/issues/1345)]
* Added built-in support for running health checks inside Docker containers
  [[GH-1343](https://github.com/hashicorp/consul/issues/1343)]
* Added prepared queries which support service health queries with rich
  features such as filters for multiple tags and failover to remote datacenters
  based on network coordinates; these are available via HTTP as well as the
  DNS interface [[GH-1389](https://github.com/hashicorp/consul/issues/1389)]

BUG FIXES:

* Fixed expired certificates in unit tests [[GH-979](https://github.com/hashicorp/consul/issues/979)]
* Allow services with `/` characters in the UI [[GH-988](https://github.com/hashicorp/consul/issues/988)]
* Added SOA/NXDOMAIN records to negative DNS responses per RFC2308 [[GH-995](https://github.com/hashicorp/consul/issues/995)]
  [[GH-1142](https://github.com/hashicorp/consul/issues/1142)] [[GH-1195](https://github.com/hashicorp/consul/issues/1195)] [[GH-1217](https://github.com/hashicorp/consul/issues/1217)]
* Token hiding in HTTP logs bug fixed [[GH-1020](https://github.com/hashicorp/consul/issues/1020)]
* RFC6598 addresses are accepted as private IPs [[GH-1050](https://github.com/hashicorp/consul/issues/1050)]
* Fixed reverse DNS lookups to recursor [[GH-1137](https://github.com/hashicorp/consul/issues/1137)]
* Removes the trailing `/` added by the `consul lock` command [[GH-1145](https://github.com/hashicorp/consul/issues/1145)]
* Fixed bad lock handler execution during shutdown [[GH-1080](https://github.com/hashicorp/consul/issues/1080)] [[GH-1158](https://github.com/hashicorp/consul/issues/1158)] [[GH-1214](https://github.com/hashicorp/consul/issues/1214)]
* Added missing support for AAAA queries for nodes [[GH-1222](https://github.com/hashicorp/consul/issues/1222)]
* Tokens passed from the CLI or API work for maint mode [[GH-1230](https://github.com/hashicorp/consul/issues/1230)]
* Fixed service deregister/reregister flaps that could happen during
  `consul reload` [[GH-1235](https://github.com/hashicorp/consul/issues/1235)]
* Fixed the Go API client to properly distinguish between expired sessions
  and sessions that don't exist [[GH-1041](https://github.com/hashicorp/consul/issues/1041)]
* Fixed the KV section of the UI to work on Safari [[GH-1321](https://github.com/hashicorp/consul/issues/1321)]
* Cleaned up JavaScript for built-in UI with bug fixes [[GH-1338](https://github.com/hashicorp/consul/issues/1338)]

IMPROVEMENTS:

* Added sorting of `consul members` command output [[GH-969](https://github.com/hashicorp/consul/issues/969)]
* Updated AWS templates for RHEL6, CentOS6 [[GH-992](https://github.com/hashicorp/consul/issues/992)] [[GH-1002](https://github.com/hashicorp/consul/issues/1002)]
* Advertised gossip/rpc addresses can now be configured [[GH-1004](https://github.com/hashicorp/consul/issues/1004)]
* Failed lock acquisition handling now responds based on type of failure
  [[GH-1006](https://github.com/hashicorp/consul/issues/1006)]
* Agents now remember check state across restarts [[GH-1009](https://github.com/hashicorp/consul/issues/1009)]
* Always run ACL tests by default in API tests [[GH-1030](https://github.com/hashicorp/consul/issues/1030)]
* Consul now refuses to start if there are multiple private IPs [[GH-1099](https://github.com/hashicorp/consul/issues/1099)]
* Improved efficiency of servers managing incoming connections from agents
  [[GH-1170](https://github.com/hashicorp/consul/issues/1170)]
* Added logging of the DNS client addresses in error messages [[GH-1166](https://github.com/hashicorp/consul/issues/1166)]
* Added `-http-port` option to change the HTTP API port number [[GH-1167](https://github.com/hashicorp/consul/issues/1167)]
* Atlas integration options are reload-able via SIGHUP [[GH-1199](https://github.com/hashicorp/consul/issues/1199)]
* Atlas endpoint is a configurable option and CLI arg [[GH-1201](https://github.com/hashicorp/consul/issues/1201)]
* Added `-pass-stdin` option to `consul lock` command [[GH-1200](https://github.com/hashicorp/consul/issues/1200)]
* Enables the `/v1/internal/ui/*` endpoints, even if `-ui-dir` isn't set
  [[GH-1215](https://github.com/hashicorp/consul/issues/1215)]
* Added HTTP method to Consul's log output for better debugging [[GH-1270](https://github.com/hashicorp/consul/issues/1270)]
* Lock holders can `?acquire=<session>` a key again with the same session
  that holds the lock to update a key's contents without releasing the
  lock [[GH-1291](https://github.com/hashicorp/consul/issues/1291)]
* Improved an O(n^2) algorithm in the agent's catalog sync code [[GH-1296](https://github.com/hashicorp/consul/issues/1296)]
* Switched to net-rpc-msgpackrpc to reduce RPC overhead [[GH-1307](https://github.com/hashicorp/consul/issues/1307)]
* Removed all uses of the http package's default client and transport in
  Consul to avoid conflicts with other packages [[GH-1310](https://github.com/hashicorp/consul/issues/1310)] [[GH-1327](https://github.com/hashicorp/consul/issues/1327)]
* Added new `X-Consul-Token` HTTP header option to avoid passing tokens
  in the query string [[GH-1318](https://github.com/hashicorp/consul/issues/1318)]
* Increased session TTL max to 24 hours (use with caution, see note added
  to the Session HTTP endpoint documentation) [[GH-1412](https://github.com/hashicorp/consul/issues/1412)]
* Added support to the API client for retrying lock monitoring when Consul
  is unavailable, helping prevent false indications of lost locks (eg. apps
  like Vault can avoid failing over when a Consul leader election occurs)
  [[GH-1457](https://github.com/hashicorp/consul/issues/1457)]
* Added reap of receive buffer space for idle streams in the connection
  pool [[GH-1452](https://github.com/hashicorp/consul/issues/1452)]

MISC:

* Lots of docs fixes
* Lots of Vagrantfile cleanup
* Data migrator utility removed to eliminate cgo dependency [[GH-1309](https://github.com/hashicorp/consul/issues/1309)]

UPGRADE NOTES:

* Consul will refuse to start if the data directory contains an "mdb" folder.
  This folder was used in versions of Consul up to 0.5.1. Consul version 0.5.2
  included a baked-in utility to automatically upgrade the data format, but
  this has been removed in Consul 0.6 to eliminate the dependency on cgo.
* New service read, event firing, and keyring ACLs may require special steps to
  perform during an upgrade if ACLs are enabled and set to deny by default.
* Consul will refuse to start if there are multiple private IPs available, so
  if this is the case you will need to configure Consul's advertise or bind
  addresses before upgrading.

See https://www.consul.io/docs/upgrade-specific.html for detailed upgrade
instructions.

## 0.5.2 (May 18, 2015)

FEATURES:

* Include datacenter in the `members` output
* HTTP Health Check sets user agent "Consul Health Check" [[GH-951](https://github.com/hashicorp/consul/issues/951)]

BUG FIXES:

* Fixed memory leak caused by blocking query [[GH-939](https://github.com/hashicorp/consul/issues/939)]

MISC:

* Remove unused constant [[GH-941](https://github.com/hashicorp/consul/issues/941)]

## 0.5.1 (May 13, 2015)

FEATURES:

 * Ability to configure minimum session TTL. [[GH-821](https://github.com/hashicorp/consul/issues/821)]
 * Ability to set the initial state of a health check when registering [[GH-859](https://github.com/hashicorp/consul/issues/859)]
 * New `configtest` sub-command to verify config validity [[GH-904](https://github.com/hashicorp/consul/issues/904)]
 * ACL enforcement is prefix based for service names [[GH-905](https://github.com/hashicorp/consul/issues/905)]
 * ACLs support upsert for simpler restore and external generation [[GH-909](https://github.com/hashicorp/consul/issues/909)]
 * ACL tokens can be provided per-service during registration [[GH-891](https://github.com/hashicorp/consul/issues/891)]
 * Support for distinct LAN and WAN advertise addresses [[GH-816](https://github.com/hashicorp/consul/issues/816)]
 * Migrating Raft log from LMDB to BoltDB [[GH-857](https://github.com/hashicorp/consul/issues/857)]
 * `session_ttl_min` is now configurable to reduce the minimum TTL [[GH-821](https://github.com/hashicorp/consul/issues/821)]
 * Adding `verify_server_hostname` to protect against server forging [[GH-927](https://github.com/hashicorp/consul/issues/927)]

BUG FIXES:

 * Datacenter is lowercased, fixes DNS lookups [[GH-761](https://github.com/hashicorp/consul/issues/761)]
 * Deregister all checks when service is deregistered [[GH-918](https://github.com/hashicorp/consul/issues/918)]
 * Fixing issues with updates of persisted services [[GH-910](https://github.com/hashicorp/consul/issues/910)]
 * Chained CNAME resolution fixes [[GH-862](https://github.com/hashicorp/consul/issues/862)]
 * Tokens are filtered out of log messages [[GH-860](https://github.com/hashicorp/consul/issues/860)]
 * Fixing anti-entropy issue if servers rollback Raft log [[GH-850](https://github.com/hashicorp/consul/issues/850)]
 * Datacenter name is case insensitive for DNS lookups
 * Queries for invalid datacenters do not leak sockets [[GH-807](https://github.com/hashicorp/consul/issues/807)]

IMPROVEMENTS:

 * HTTP health checks more reliable, avoid KeepAlives [[GH-824](https://github.com/hashicorp/consul/issues/824)]
 * Improved protection against a passive cluster merge
 * SIGTERM is properly handled for graceful shutdown [[GH-827](https://github.com/hashicorp/consul/issues/827)]
 * Better staggering of deferred updates to checks [[GH-884](https://github.com/hashicorp/consul/issues/884)]
 * Configurable stats prefix [[GH-902](https://github.com/hashicorp/consul/issues/902)]
 * Raft uses BoltDB as the backend store. [[GH-857](https://github.com/hashicorp/consul/issues/857)]
 * API RenewPeriodic more resilient to transient errors [[GH-912](https://github.com/hashicorp/consul/issues/912)]

## 0.5.0 (February 19, 2015)

FEATURES:

 * Key rotation support for gossip layer. This allows the `encrypt` key
   to be changed globally.  See "keyring" command. [[GH-336](https://github.com/hashicorp/consul/issues/336)]
 * Options to join the WAN pool on start (`start_join_wan`, `retry_join_wan`) [[GH-477](https://github.com/hashicorp/consul/issues/477)]
 * Optional HTTPS interface [[GH-478](https://github.com/hashicorp/consul/issues/478)]
 * Ephemeral keys via "delete" session behavior. This allows keys to be deleted when
   a session is invalidated instead of having the lock released. Adds new "Behavior"
   field to Session which is configurable. [[GH-487](https://github.com/hashicorp/consul/issues/487)]
 * Reverse DNS lookups via PTR for IPv4 and IPv6 [[GH-475](https://github.com/hashicorp/consul/issues/475)]
 * API added checks and services are persisted. This means services and
   checks will survive a crash or restart. [[GH-497](https://github.com/hashicorp/consul/issues/497)]
 * ACLs can now protect service registration. Users in blacklist mode should
   allow registrations before upgrading to prevent a service disruption. [[GH-506](https://github.com/hashicorp/consul/issues/506)] [[GH-465](https://github.com/hashicorp/consul/issues/465)]
 * Sessions support a heartbeat failure detector via use of TTLs. This adds a new
   "TTL" field to Sessions and a `/v1/session/renew` endpoint. Heartbeats act like a
   failure detector (health check), but are managed by the servers. [[GH-524](https://github.com/hashicorp/consul/issues/524)] [[GH-172](https://github.com/hashicorp/consul/issues/172)]
 * Support for service specific IP addresses. This allows the service to advertise an
   address that is different from the agent. [[GH-229](https://github.com/hashicorp/consul/issues/229)] [[GH-570](https://github.com/hashicorp/consul/issues/570)]
 * Support KV Delete with Check-And-Set  [[GH-589](https://github.com/hashicorp/consul/issues/589)]
 * Merge `armon/consul-api` into `api` as official Go client.
 * Support for distributed locks and semaphores in API client [[GH-594](https://github.com/hashicorp/consul/issues/594)] [[GH-600](https://github.com/hashicorp/consul/issues/600)]
 * Support for native HTTP health checks [[GH-592](https://github.com/hashicorp/consul/issues/592)]
 * Support for node and service maintenance modes [[GH-606](https://github.com/hashicorp/consul/issues/606)]
 * Added new "consul maint" command to easily toggle maintenance modes [[GH-625](https://github.com/hashicorp/consul/issues/625)]
 * Added new "consul lock" command for simple highly-available deployments.
   This lets Consul manage the leader election and easily handle N+1 deployments
   without the applications being Consul aware. [[GH-619](https://github.com/hashicorp/consul/issues/619)]
 * Multiple checks can be associated with a service [[GH-591](https://github.com/hashicorp/consul/issues/591)] [[GH-230](https://github.com/hashicorp/consul/issues/230)]

BUG FIXES:

 * Fixed X-Consul-Index calculation for KV ListKeys
 * Fixed errors under extremely high read parallelism
 * Fixed issue causing event watches to not fire reliably [[GH-479](https://github.com/hashicorp/consul/issues/479)]
 * Fixed non-monotonic X-Consul-Index with key deletion [[GH-577](https://github.com/hashicorp/consul/issues/577)] [[GH-195](https://github.com/hashicorp/consul/issues/195)]
 * Fixed use of default instead of custom TLD in some DNS responses [[GH-582](https://github.com/hashicorp/consul/issues/582)]
 * Fixed memory leaks in API client when an error response is returned [[GH-608](https://github.com/hashicorp/consul/issues/608)]
 * Fixed issues with graceful leave in single-node bootstrap cluster [[GH-621](https://github.com/hashicorp/consul/issues/621)]
 * Fixed issue preventing node reaping [[GH-371](https://github.com/hashicorp/consul/issues/371)]
 * Fixed gossip stability at very large scale
 * Fixed string of rpc error: rpc error: ... no known leader. [[GH-611](https://github.com/hashicorp/consul/issues/611)]
 * Fixed panic in `exec` during cancellation
 * Fixed health check state reset caused by SIGHUP [[GH-693](https://github.com/hashicorp/consul/issues/693)]
 * Fixed bug in UI when multiple datacenters exist.

IMPROVEMENTS:

 * Support "consul exec" in foreign datacenter [[GH-584](https://github.com/hashicorp/consul/issues/584)]
 * Improved K/V blocking query performance [[GH-578](https://github.com/hashicorp/consul/issues/578)]
 * CLI respects CONSUL_RPC_ADDR environment variable to load parameter [[GH-542](https://github.com/hashicorp/consul/issues/542)]
 * Added support for multiple DNS recursors [[GH-448](https://github.com/hashicorp/consul/issues/448)]
 * Added support for defining multiple services per configuration file [[GH-433](https://github.com/hashicorp/consul/issues/433)]
 * Added support for defining multiple checks per configuration file [[GH-433](https://github.com/hashicorp/consul/issues/433)]
 * Allow mixing of service and check definitions in a configuration file [[GH-433](https://github.com/hashicorp/consul/issues/433)]
 * Allow notes for checks in service definition file [[GH-449](https://github.com/hashicorp/consul/issues/449)]
 * Random stagger for agent checks to prevent thundering herd [[GH-546](https://github.com/hashicorp/consul/issues/546)]
 * More useful metrics are sent to statsd/statsite
 * Added configuration to set custom HTTP headers (CORS) [[GH-558](https://github.com/hashicorp/consul/issues/558)]
 * Reject invalid configurations to simplify validation [[GH-576](https://github.com/hashicorp/consul/issues/576)]
 * Guard against accidental cluster mixing [[GH-580](https://github.com/hashicorp/consul/issues/580)] [[GH-260](https://github.com/hashicorp/consul/issues/260)]
 * Added option to filter DNS results on warning [[GH-595](https://github.com/hashicorp/consul/issues/595)]
 * Improve write throughput with raft log caching [[GH-604](https://github.com/hashicorp/consul/issues/604)]
 * Added ability to bind RPC and HTTP listeners to UNIX sockets [[GH-587](https://github.com/hashicorp/consul/issues/587)] [[GH-612](https://github.com/hashicorp/consul/issues/612)]
 * K/V HTTP endpoint returns 400 on conflicting flags [[GH-634](https://github.com/hashicorp/consul/issues/634)] [[GH-432](https://github.com/hashicorp/consul/issues/432)]

MISC:

 * UI confirms before deleting key sub-tree [[GH-520](https://github.com/hashicorp/consul/issues/520)]
 * More useful output in "consul version" [[GH-480](https://github.com/hashicorp/consul/issues/480)]
 * Many documentation improvements
 * Reduce log messages when quorum member is logs [[GH-566](https://github.com/hashicorp/consul/issues/566)]

UPGRADE NOTES:

 * If `acl_default_policy` is "deny", ensure tokens are updated to enable
   service registration to avoid a service disruption. The new ACL policy
   can be submitted with 0.4 before upgrading to 0.5 where it will be
   enforced.

 * Servers running 0.5.X cannot be mixed with older servers. (Any client
   version is fine). There is a 15 minute upgrade window where mixed
   versions are allowed before older servers will panic due to an unsupported
   internal command. This is due to the new KV tombstones which are internal
   to servers.

## 0.4.1 (October 20, 2014)

FEATURES:

 * Adding flags for `-retry-join` to attempt a join with
   configurable retry behavior. [[GH-395](https://github.com/hashicorp/consul/issues/395)]

BUG FIXES:

 * Fixed ACL token in UI
 * Fixed ACL reloading in UI [[GH-323](https://github.com/hashicorp/consul/issues/323)]
 * Fixed long session names in UI [[GH-353](https://github.com/hashicorp/consul/issues/353)]
 * Fixed exit code from remote exec [[GH-346](https://github.com/hashicorp/consul/issues/346)]
 * Fixing only a single watch being run by an agent [[GH-337](https://github.com/hashicorp/consul/issues/337)]
 * Fixing potential race in connection multiplexing
 * Fixing issue with Session ID and ACL ID generation. [[GH-391](https://github.com/hashicorp/consul/issues/391)]
 * Fixing multiple headers for /v1/event/list endpoint [[GH-361](https://github.com/hashicorp/consul/issues/361)]
 * Fixing graceful leave of leader causing invalid Raft peers [[GH-360](https://github.com/hashicorp/consul/issues/360)]
 * Fixing bug with closing TLS connection on error
 * Fixing issue with node reaping [[GH-371](https://github.com/hashicorp/consul/issues/371)]
 * Fixing aggressive deadlock time [[GH-389](https://github.com/hashicorp/consul/issues/389)]
 * Fixing syslog filter level [[GH-272](https://github.com/hashicorp/consul/issues/272)]
 * Serf snapshot compaction works on Windows [[GH-332](https://github.com/hashicorp/consul/issues/332)]
 * Raft snapshots work on Windows [[GH-265](https://github.com/hashicorp/consul/issues/265)]
 * Consul service entry clean by clients now possible
 * Fixing improper deserialization

IMPROVEMENTS:

 * Use "critical" health state instead of "unknown" [[GH-341](https://github.com/hashicorp/consul/issues/341)]
 * Consul service can be targeted for exec [[GH-344](https://github.com/hashicorp/consul/issues/344)]
 * Provide debug logging for session invalidation [[GH-390](https://github.com/hashicorp/consul/issues/390)]
 * Added "Deregister" button to UI [[GH-364](https://github.com/hashicorp/consul/issues/364)]
 * Added `enable_truncate` DNS configuration flag [[GH-376](https://github.com/hashicorp/consul/issues/376)]
 * Reduce mmap() size on 32bit systems [[GH-265](https://github.com/hashicorp/consul/issues/265)]
 * Temporary state is cleaned after an abort [[GH-338](https://github.com/hashicorp/consul/issues/338)] [[GH-178](https://github.com/hashicorp/consul/issues/178)]

MISC:

 * Health state "unknown" being deprecated

## 0.4.0 (September 5, 2014)

FEATURES:

 * Fine-grained ACL system to restrict access to KV store. Clients
   use tokens which can be restricted to (read, write, deny) permissions
   using longest-prefix matches.

 * Watch mechanisms added to invoke a handler when data changes in consul.
   Used with the `consul watch` command, or by specifying `watches` in
   an agent configuration.

 * Event system added to support custom user events. Events are fired using
   the `consul event` command. They are handled using a standard watch.

 * Remote execution using `consul exec`. This allows for command execution on remote
   instances mediated through Consul.

 * RFC-2782 style DNS lookups supported

 * UI improvements, including support for ACLs.

IMPROVEMENTS:

  * DNS case-insensitivity [[GH-189](https://github.com/hashicorp/consul/issues/189)]
  * Support for HTTP `?pretty` parameter to pretty format JSON output.
  * Use $SHELL when invoking handlers. [[GH-237](https://github.com/hashicorp/consul/issues/237)]
  * Agent takes the `-encrypt` CLI Flag [[GH-245](https://github.com/hashicorp/consul/issues/245)]
  * New `statsd_add` config for Statsd support. [[GH-247](https://github.com/hashicorp/consul/issues/247)]
  * New `addresses` config for providing an override to `client_addr` for
    DNS, HTTP, or RPC endpoints. [[GH-301](https://github.com/hashicorp/consul/issues/301)] [[GH-253](https://github.com/hashicorp/consul/issues/253)]
  * Support [Checkpoint](http://checkpoint.hashicorp.com) for security bulletins
    and update announcements.

BUG FIXES:

  * Fixed race condition in `-bootstrap-expect` [[GH-254](https://github.com/hashicorp/consul/issues/254)]
  * Require PUT to /v1/session/destroy [[GH-285](https://github.com/hashicorp/consul/issues/285)]
  * Fixed registration race condition [[GH-300](https://github.com/hashicorp/consul/issues/300)] [[GH-279](https://github.com/hashicorp/consul/issues/279)]

UPGRADE NOTES:

  * ACL support should not be enabled until all server nodes are running
  Consul 0.4. Mixed server versions with ACL support enabled may result in
  panics.

## 0.3.1 (July 21, 2014)

FEATURES:

  * Improved bootstrapping process, thanks to @robxu9

BUG FIXES:

  * Fixed issue with service re-registration [[GH-216](https://github.com/hashicorp/consul/issues/216)]
  * Fixed handling of `-rejoin` flag
  * Restored 0.2 TLS behavior, thanks to @nelhage [[GH-233](https://github.com/hashicorp/consul/issues/233)]
  * Fix the statsite flags, thanks to @nelhage [[GH-243](https://github.com/hashicorp/consul/issues/243)]
  * Fixed filters on critical / non-passing checks [[GH-241](https://github.com/hashicorp/consul/issues/241)]
  * Fixed initial log compaction crash [[GH-297](https://github.com/hashicorp/consul/issues/297)]

IMPROVEMENTS:

  * UI Improvements
  * Improved handling of Serf snapshot data
  * Increase reliability of failure detector
  * More useful logging messages


## 0.3.0 (June 13, 2014)

FEATURES:

  * Better, faster, cleaner UI [[GH-194](https://github.com/hashicorp/consul/issues/194)] [[GH-196](https://github.com/hashicorp/consul/issues/196)]
  * Sessions, which  act as a binding layer between
  nodes, checks and KV data. [[GH-162](https://github.com/hashicorp/consul/issues/162)]
  * Key locking. KV data integrates with sessions to
  enable distributed locking. [[GH-162](https://github.com/hashicorp/consul/issues/162)]
  * DNS lookups can do stale reads and TTLs. [[GH-200](https://github.com/hashicorp/consul/issues/200)]
  * Added new /v1/agent/self endpoint [[GH-173](https://github.com/hashicorp/consul/issues/173)]
  * `reload` command can be used to trigger configuration
  reload from the CLI [[GH-142](https://github.com/hashicorp/consul/issues/142)]

IMPROVEMENTS:

  * `members` has a much cleaner output format [[GH-143](https://github.com/hashicorp/consul/issues/143)]
  * `info` includes build version information
  * Sorted results for datacneter list [[GH-198](https://github.com/hashicorp/consul/issues/198)]
  * Switch multiplexing to yamux
  * Allow multiple CA certs in ca_file [[GH-174](https://github.com/hashicorp/consul/issues/174)]
  * Enable logging to syslog. [[GH-105](https://github.com/hashicorp/consul/issues/105)]
  * Allow raw key value lookup [[GH-150](https://github.com/hashicorp/consul/issues/150)]
  * Log encryption enabled [[GH-151](https://github.com/hashicorp/consul/issues/151)]
  * Support `-rejoin` to rejoin a cluster after a previous leave. [[GH-110](https://github.com/hashicorp/consul/issues/110)]
  * Support the "any" wildcard for v1/health/state/ [[GH-152](https://github.com/hashicorp/consul/issues/152)]
  * Defer sync of health check output [[GH-157](https://github.com/hashicorp/consul/issues/157)]
  * Provide output for serfHealth check [[GH-176](https://github.com/hashicorp/consul/issues/176)]
  * Datacenter name is validated [[GH-169](https://github.com/hashicorp/consul/issues/169)]
  * Configurable syslog facilities [[GH-170](https://github.com/hashicorp/consul/issues/170)]
  * Pipelining replication of writes
  * Raft group commits
  * Increased stability of leader terms
  * Prevent previously left nodes from causing re-elections

BUG FIXES:

  * Fixed memory leak in in-memory stats system
  * Fixing race between RPC and Raft init [[GH-160](https://github.com/hashicorp/consul/issues/160)]
  * Server-local RPC is avoids network [[GH-148](https://github.com/hashicorp/consul/issues/148)]
  * Fixing builds for older OSX [[GH-147](https://github.com/hashicorp/consul/issues/147)]

MISC:

  * Fixed missing prefixes on some log messages
  * Removed the `-role` filter of `members` command
  * Lots of docs fixes

## 0.2.1 (May 20, 2014)

IMPROVEMENTS:

  * Improved the URL formatting for the key/value editor in the Web UI.
      Importantly, the editor now allows editing keys with dashes in the
      name. [[GH-119](https://github.com/hashicorp/consul/issues/119)]
  * The web UI now has cancel and delete folder actions in the key/value
      editor. [[GH-124](https://github.com/hashicorp/consul/issues/124)], [[GH-122](https://github.com/hashicorp/consul/issues/122)]
  * Add flag to agent to write pid to a file. [[GH-106](https://github.com/hashicorp/consul/issues/106)]
  * Time out commands if Raft exceeds command enqueue timeout
  * Adding support for the `-advertise` CLI flag. [[GH-156](https://github.com/hashicorp/consul/issues/156)]
  * Fixing potential name conflicts on the WAN gossip ring [[GH-158](https://github.com/hashicorp/consul/issues/158)]
  * /v1/catalog/services returns an empty slice instead of null. [[GH-145](https://github.com/hashicorp/consul/issues/145)]
  * `members` command returns exit code 2 if no results. [[GH-116](https://github.com/hashicorp/consul/issues/116)]

BUG FIXES:

  * Renaming "separator" to "separator". This is the correct spelling,
      but both spellings are respected for backwards compatibility. [[GH-101](https://github.com/hashicorp/consul/issues/101)]
  * Private IP is properly found on Windows clients.
  * Windows agents won't show "failed to decode" errors on every RPC
      request.
  * Fixed memory leak with RPC clients. [[GH-149](https://github.com/hashicorp/consul/issues/149)]
  * Serf name conflict resolution disabled. [[GH-97](https://github.com/hashicorp/consul/issues/97)]
  * Raft deadlock possibility fixed. [[GH-141](https://github.com/hashicorp/consul/issues/141)]

MISC:

  * Updating to latest version of LMDB
  * Reduced the limit of KV entries to 512KB. [[GH-123](https://github.com/hashicorp/consul/issues/123)].
  * Warn if any Raft log exceeds 1MB
  * Lots of docs fixes

## 0.2.0 (May 1, 2014)

FEATURES:

  * Adding Web UI for Consul. This is enabled by providing the `-ui-dir` flag
      with the path to the web directory. The UI is visited at the standard HTTP
      address (Defaults to http://127.0.0.1:8500/). There is a demo
  [available here](http://demo.consul.io).
  * Adding new read consistency modes. `?consistent` can be used for strongly
      consistent reads without caveats. `?stale` can be used for stale reads to
      allow for higher throughput and read scalability. [[GH-68](https://github.com/hashicorp/consul/issues/68)]
  * /v1/health/service/ endpoint can take an optional `?passing` flag
      to filter to only nodes with passing results. [[GH-57](https://github.com/hashicorp/consul/issues/57)]
  * The KV endpoint supports listing keys with the `?keys` query parameter,
      and limited up to a separator using `?separator=`.

IMPROVEMENTS:

  * Health check output goes into separate `Output` field instead
      of overriding `Notes`. [[GH-59](https://github.com/hashicorp/consul/issues/59)]
  * Adding a minimum check interval to prevent checks with extremely
      low intervals fork bombing. [[GH-64](https://github.com/hashicorp/consul/issues/64)]
  * Raft peer set cleared on leave. [[GH-69](https://github.com/hashicorp/consul/issues/69)]
  * Case insensitive parsing checks. [[GH-78](https://github.com/hashicorp/consul/issues/78)]
  * Increase limit of DB size and Raft log on 64bit systems. [[GH-81](https://github.com/hashicorp/consul/issues/81)]
  * Output of health checks limited to 4K. [[GH-83](https://github.com/hashicorp/consul/issues/83)]
  * More warnings if GOMAXPROCS == 1 [[GH-87](https://github.com/hashicorp/consul/issues/87)]
  * Added runtime information to `consul info`

BUG FIXES:

  * Fixed 404 on /v1/agent/service/deregister and
      /v1/agent/check/deregister. [[GH-95](https://github.com/hashicorp/consul/issues/95)]
  * Fixed JSON parsing for /v1/agent/check/register [[GH-60](https://github.com/hashicorp/consul/issues/60)]
  * DNS parser can handler period in a tag name. [[GH-39](https://github.com/hashicorp/consul/issues/39)]
  * "application/json" content-type is sent on HTTP requests. [[GH-45](https://github.com/hashicorp/consul/issues/45)]
  * Work around for LMDB delete issue. [[GH-85](https://github.com/hashicorp/consul/issues/85)]
  * Fixed tag gossip propagation for rapid restart. [[GH-86](https://github.com/hashicorp/consul/issues/86)]

MISC:

  * More conservative timing values for Raft
  * Provide a warning if attempting to commit a very large Raft entry
  * Improved timeliness of registration when server is in bootstrap mode. [[GH-72](https://github.com/hashicorp/consul/issues/72)]

## 0.1.0 (April 17, 2014)

  * Initial release
