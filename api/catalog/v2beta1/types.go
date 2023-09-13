package v2beta1

import "github.com/hashicorp/consul/proto-public/pbresource"

const (
	GroupName      = "catalog"
	VersionV2Beta1 = "v2beta1"
	CurrentVersion = VersionV2Beta1

	WorkloadKind         = "Workload"
	ServiceKind          = "Service"
	ServiceEndpointsKind = "ServiceEndpoints"
	NodeKind             = "Node"
	HealthStatusKind     = "HealthStatus"
	HealthChecksKind     = "HealthChecks"
	FailoverPolicyKind   = "FailoverPolicy"
	VirtualIPsKind       = "VirtualIPs"
	DNSPolicyKind        = "DNSPolicy"
)

var (
	// Workload
	WorkloadV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         WorkloadKind,
	}

	WorkloadType = WorkloadV2Beta1Type

	// Service
	ServiceV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         ServiceKind,
	}

	ServiceType = ServiceV2Beta1Type

	// ServiceEndpoints
	ServiceEndpointsV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         ServiceEndpointsKind,
	}

	ServiceEndpointsType = ServiceEndpointsV2Beta1Type

	// Node
	NodeV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         NodeKind,
	}

	NodeType = NodeV2Beta1Type

	// HealthStatus
	HealthStatusV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         HealthStatusKind,
	}

	HealthStatusType = HealthStatusV2Beta1Type

	// HealthChecks
	HealthChecksV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         HealthChecksKind,
	}

	HealthChecksType = HealthChecksV2Beta1Type

	// FailoverPolicy
	FailoverPolicyV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         FailoverPolicyKind,
	}

	FailoverPolicyType = FailoverPolicyV2Beta1Type

	// Virtual IPs
	VirtualIPsV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         VirtualIPsKind,
	}

	VirtualIPsType = VirtualIPsV2Beta1Type

	DNSPolicyV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         DNSPolicyKind,
	}

	DNSPolicyType = DNSPolicyV2Beta1Type
)
