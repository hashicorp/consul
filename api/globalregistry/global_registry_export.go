// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package globalregistry

import (
	"time"
)

const EventTopicGlobalRegistry = "global-registry"

// Virtual table names used in GlobalRegistryDeltaExport for data that does not
// live in memdb but is injected at payload-build time.
const (
	TableWANMembers                   = "wan_members"
	TableAgentSelf                    = "agent_self"
	TableACLAuthMethods               = "acl_auth_methods"
	TableACLStats                     = "acl_stats"
	TableCA                           = "ca"
	GlobalRegistryPayloadTypeSnapshot = "SNAPSHOT"
	GlobalRegistryPayloadTypeDelta    = "DELTA"
)

type GlobalRegistryPayload struct {
	Type         string                        `json:"type"`
	ClusterID    string                        `json:"clusterId"`
	Timestamp    int64                         `json:"timestamp"`
	SnapshotData *GlobalRegistrySnapshotExport `json:"snapshotData"`
	DeltaData    []GlobalRegistryDeltaExport   `json:"deltaData"`
}

type GlobalRegistrySnapshotExport struct {
	Nodes            []GlobalRegistryNodeRecord             `json:"nodes"`
	Services         []GlobalRegistryServiceRecord          `json:"services"`
	Checks           []GlobalRegistryCheckRecord            `json:"checks"`
	GatewayServices  []GlobalRegistryGatewayServiceRecord   `json:"gatewayServices"`
	MeshTopology     []GlobalRegistryMeshTopologyRecord     `json:"meshTopology"`
	ServiceVIPs      []GlobalRegistryServiceVIPRecord       `json:"serviceVIPs"`
	FreeVIPs         []GlobalRegistryFreeVIPRecord          `json:"freeVIPs"`
	KindServiceNames []GlobalRegistryKindServiceNameRecord  `json:"kindServiceNames"`
	DiscoveryChains  []GlobalRegistryDiscoveryChainRecord   `json:"discoveryChains,omitempty"`
	ConfigEntries    []GlobalRegistryConfigEntryRecord      `json:"configEntries"`
	Peerings         []GlobalRegistryPeeringRecord          `json:"peerings"`
	Partitions       []GlobalRegistryPartitionRecord        `json:"partitions"`
	Namespaces       []GlobalRegistryNamespaceRecord        `json:"namespaces"`
	WANMembers       []GlobalRegistryWANMemberRecord        `json:"wanMembers,omitempty"`
	AgentSelf        *GlobalRegistryAgentSelfRecord         `json:"agentSelf,omitempty"`
	ACLAuthMethods   []GlobalRegistryACLAuthMethodTypeCount `json:"aclAuthMethods,omitempty"`
	ACLStats         *GlobalRegistryACLStatsRecord          `json:"aclStats,omitempty"`
	CA               *GlobalRegistryCARecord                `json:"ca,omitempty"`
}

// GlobalRegistryACLAuthMethodTypeCount reports how many auth methods of a
// given type exist across the cluster. Individual names and configs are
// intentionally omitted to avoid leaking secrets.
type GlobalRegistryACLAuthMethodTypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// GlobalRegistryACLStatsRecord holds cluster-wide aggregate counts for the
// three main ACL object types.
//
// Token count excludes the built-in anonymous token and any tokens whose
// ExpirationTime is in the past (expired but not yet reaped).
type GlobalRegistryACLStatsRecord struct {
	PoliciesCount int `json:"policiesCount"`
	TokensCount   int `json:"tokensCount"`
	RolesCount    int `json:"rolesCount"`
}

// GlobalRegistryCARecord carries the Connect CA posture for this cluster.
// Private key material and CA Config/State maps are never included.
type GlobalRegistryCARecord struct {
	// Provider is the active CA backend: "consul", "vault", "aws-pca", etc.
	Provider string `json:"provider,omitempty"`
	// TrustDomain is the SPIFFE trust domain derived from CAConfiguration.ClusterID.
	// Format: "<clusterID>.consul"  e.g. "4f2a1aa8-….consul"
	TrustDomain string `json:"trustDomain,omitempty"`
	// RootExpiresAt is the NotAfter of the currently active root certificate.
	RootExpiresAt *time.Time `json:"rootExpiresAt,omitempty"`
	// RootRotationInProgress is true when more than one root has a zero
	// RotatedOutAt timestamp — i.e. the old root has not yet been retired.
	RootRotationInProgress bool `json:"rootRotationInProgress"`
}

// GlobalRegistryWANMemberRecord represents one member of the WAN Serf pool.
// Table name used in DeltaData: "wan_members".
type GlobalRegistryWANMemberRecord struct {
	Name   string            `json:"name"`
	Addr   string            `json:"addr"`
	Status string            `json:"status"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// GlobalRegistryAgentSelfRecord captures the server's own identity and version,
// equivalent to the Config block returned by /v1/agent/self.
// Table name used in DeltaData: "agent_self".
// Because agent-self data is static config it is emitted on every snapshot and
// as a single UPSERT delta whenever the syncer restarts (i.e. on leader change).
type GlobalRegistryAgentSelfRecord struct {
	Datacenter        string `json:"datacenter"`
	PrimaryDatacenter string `json:"primaryDatacenter,omitempty"`
	NodeName          string `json:"nodeName"`
	NodeID            string `json:"nodeId,omitempty"`
	Build             string `json:"build"`
	Server            bool   `json:"server"`
	// ConnectEnabled is true when Connect (service mesh) is enabled in config.
	ConnectEnabled bool `json:"connectEnabled"`
	// TLSVerifyIncoming is true when the server requires mutual TLS on RPC.
	TLSVerifyIncoming bool `json:"tlsVerifyIncoming"`
	// TLSVerifyOutgoing is true when the server verifies TLS on outgoing RPC.
	TLSVerifyOutgoing bool `json:"tlsVerifyOutgoing"`
	// ACLsEnabled is true when the ACL system is enabled in config.
	ACLsEnabled bool `json:"aclsEnabled"`
}

// GlobalRegistryPeeringRecord is a flat export of a pbpeering.Peering row.
type GlobalRegistryPeeringRecord struct {
	ID                  string            `json:"id,omitempty"`
	Name                string            `json:"name"`
	Partition           string            `json:"partition,omitempty"`
	State               string            `json:"state,omitempty"`
	PeerID              string            `json:"peerId,omitempty"`
	PeerServerName      string            `json:"peerServerName,omitempty"`
	PeerServerAddresses []string          `json:"peerServerAddresses,omitempty"`
	Meta                map[string]string `json:"meta,omitempty"`
	ModifyIndex         uint64            `json:"modifyIndex,omitempty"`
	Obj                 any               `json:"obj,omitempty"`
}

// GlobalRegistryPartitionRecord is a flat export of a pbpartition.Partition row.
// On CE builds this slice is always empty.
type GlobalRegistryPartitionRecord struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ModifyIndex uint64 `json:"modifyIndex,omitempty"`
	Obj         any    `json:"obj,omitempty"`
}

// GlobalRegistryNamespaceRecord is a flat export of a pbnamespace.Namespace row.
// On CE builds this slice is always empty.
type GlobalRegistryNamespaceRecord struct {
	Name        string            `json:"name"`
	Partition   string            `json:"partition,omitempty"`
	Description string            `json:"description,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	ModifyIndex uint64            `json:"modifyIndex,omitempty"`
	Obj         any               `json:"obj,omitempty"`
}

// GlobalRegistryConfigEntryRecord is a generic, flat representation of any
// config entry stored in tableConfigEntries.  The raw JSON of the full entry
// is carried in RawEntry so receivers can decode the kind-specific payload
// without requiring type-switch logic here.
type GlobalRegistryConfigEntryRecord struct {
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Partition   string            `json:"partition,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	ModifyIndex uint64            `json:"modifyIndex,omitempty"`
	Obj         any               `json:"obj,omitempty"`
}

// GlobalRegistryDiscoveryChainRecord is a flattened, JSON-serialisable
// representation of a structs.CompiledDiscoveryChain.
type GlobalRegistryDiscoveryChainRecord struct {
	ServiceName       string         `json:"serviceName"`
	Namespace         string         `json:"namespace,omitempty"`
	Partition         string         `json:"partition,omitempty"`
	Datacenter        string         `json:"datacenter,omitempty"`
	Protocol          string         `json:"protocol,omitempty"`
	Default           bool           `json:"default,omitempty"`
	CustomizationHash string         `json:"customizationHash,omitempty"`
	StartNode         string         `json:"startNode,omitempty"`
	Nodes             map[string]any `json:"nodes,omitempty"`
	Targets           map[string]any `json:"targets,omitempty"`
	AutoVirtualIPs    []string       `json:"autoVirtualIPs,omitempty"`
	ManualVirtualIPs  []string       `json:"manualVirtualIPs,omitempty"`
	Obj               any            `json:"obj,omitempty"`
}

type GlobalRegistryNodeRecord struct {
	ID              string            `json:"id,omitempty"`
	Node            string            `json:"node"`
	Address         string            `json:"address"`
	Datacenter      string            `json:"datacenter,omitempty"`
	Partition       string            `json:"partition,omitempty"`
	PeerName        string            `json:"peerName,omitempty"`
	TaggedAddresses map[string]string `json:"taggedAddresses,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
	ModifyIndex     uint64            `json:"modifyIndex,omitempty"`
	Obj             any               `json:"obj,omitempty"`
}

type GlobalRegistryServiceRecord struct {
	Node                     string            `json:"node"`
	NodeID                   string            `json:"nodeId,omitempty"`
	Datacenter               string            `json:"datacenter,omitempty"`
	ServiceID                string            `json:"serviceId"`
	ServiceName              string            `json:"serviceName"`
	ServiceKind              string            `json:"serviceKind,omitempty"`
	ServiceAddress           string            `json:"serviceAddress,omitempty"`
	ServicePort              int               `json:"servicePort,omitempty"`
	ServiceTags              []string          `json:"serviceTags,omitempty"`
	ServiceMeta              map[string]string `json:"serviceMeta,omitempty"`
	Partition                string            `json:"partition,omitempty"`
	Namespace                string            `json:"namespace,omitempty"`
	PeerName                 string            `json:"peerName,omitempty"`
	ProxyDestinationService  string            `json:"proxyDestinationService,omitempty"`
	ConnectNative            bool              `json:"connectNative,omitempty"`
	ServiceEnableTagOverride bool              `json:"serviceEnableTagOverride,omitempty"`
	ModifyIndex              uint64            `json:"modifyIndex,omitempty"`
	Obj                      any               `json:"obj,omitempty"`
}

type GlobalRegistryCheckRecord struct {
	Node        string `json:"node"`
	CheckID     string `json:"checkId"`
	Name        string `json:"name,omitempty"`
	Status      string `json:"status"`
	Output      string `json:"output,omitempty"`
	ServiceID   string `json:"serviceId,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	Type        string `json:"type,omitempty"`
	ExposedPort int    `json:"exposedPort,omitempty"`
	Partition   string `json:"partition,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	PeerName    string `json:"peerName,omitempty"`
	ModifyIndex uint64 `json:"modifyIndex,omitempty"`
	Obj         any    `json:"obj,omitempty"`
}

type GlobalRegistryGatewayServiceRecord struct {
	Gateway         string   `json:"gateway"`
	Service         string   `json:"service"`
	GatewayKind     string   `json:"gatewayKind,omitempty"`
	Port            int      `json:"port,omitempty"`
	Protocol        string   `json:"protocol,omitempty"`
	Hosts           []string `json:"hosts,omitempty"`
	SNI             string   `json:"sni,omitempty"`
	FromWildcard    bool     `json:"fromWildcard,omitempty"`
	ServiceKind     string   `json:"serviceKind,omitempty"`
	AutoHostRewrite bool     `json:"autoHostRewrite,omitempty"`
	ModifyIndex     uint64   `json:"modifyIndex,omitempty"`
	Obj             any      `json:"obj,omitempty"`
}

type GlobalRegistryMeshTopologyRecord struct {
	Upstream    string `json:"upstream"`
	Downstream  string `json:"downstream"`
	ModifyIndex uint64 `json:"modifyIndex,omitempty"`
	Obj         any    `json:"obj,omitempty"`
}

type GlobalRegistryServiceVIPRecord struct {
	Service     string   `json:"service"`
	Peer        string   `json:"peer,omitempty"`
	IP          string   `json:"ip,omitempty"`
	ManualIPs   []string `json:"manualIps,omitempty"`
	ModifyIndex uint64   `json:"modifyIndex,omitempty"`
	Obj         any      `json:"obj,omitempty"`
}

type GlobalRegistryFreeVIPRecord struct {
	IP        string `json:"ip,omitempty"`
	IsCounter bool   `json:"isCounter"`
	Obj       any    `json:"obj,omitempty"`
}

type GlobalRegistryKindServiceNameRecord struct {
	Kind        string `json:"kind"`
	Service     string `json:"service"`
	ModifyIndex uint64 `json:"modifyIndex,omitempty"`
	Obj         any    `json:"obj,omitempty"`
}

type GlobalRegistryDeltaExport struct {
	Action string      `json:"action"`
	Table  string      `json:"table"`
	Record interface{} `json:"record"`
}

// GlobalRegistryAggregates holds the computed aggregate fields that are
// recomputed and injected into every delta payload as well as snapshots.
type GlobalRegistryAggregates struct {
	ACLAuthMethods []GlobalRegistryACLAuthMethodTypeCount `json:"aclAuthMethods,omitempty"`
	ACLStats       *GlobalRegistryACLStatsRecord          `json:"aclStats,omitempty"`
	CA             *GlobalRegistryCARecord                `json:"ca,omitempty"`
}
