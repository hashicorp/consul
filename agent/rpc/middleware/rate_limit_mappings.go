// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package middleware

import "github.com/hashicorp/consul/agent/consul/rate"

// Maps each net/rpc endpoint to a read or write operation type
// for rate limiting purposes. Please be sure to update this list
// if a net/rpc endpoint is removed.
var rpcRateLimitSpecs = map[string]rate.OperationType{
	"ACL.AuthMethodDelete":  rate.OperationTypeWrite,
	"ACL.AuthMethodList":    rate.OperationTypeRead,
	"ACL.AuthMethodRead":    rate.OperationTypeRead,
	"ACL.AuthMethodSet":     rate.OperationTypeWrite,
	"ACL.Authorize":         rate.OperationTypeRead,
	"ACL.BindingRuleDelete": rate.OperationTypeWrite,
	"ACL.BindingRuleList":   rate.OperationTypeRead,
	"ACL.BindingRuleRead":   rate.OperationTypeRead,
	"ACL.BindingRuleSet":    rate.OperationTypeWrite,
	"ACL.BootstrapTokens":   rate.OperationTypeRead,
	"ACL.Login":             rate.OperationTypeWrite,
	"ACL.Logout":            rate.OperationTypeWrite,
	"ACL.PolicyBatchRead":   rate.OperationTypeRead,
	"ACL.PolicyDelete":      rate.OperationTypeWrite,
	"ACL.PolicyList":        rate.OperationTypeRead,
	"ACL.PolicyRead":        rate.OperationTypeRead,
	"ACL.PolicyResolve":     rate.OperationTypeRead,
	"ACL.PolicySet":         rate.OperationTypeWrite,
	"ACL.ReplicationStatus": rate.OperationTypeRead,
	"ACL.RoleBatchRead":     rate.OperationTypeRead,
	"ACL.RoleDelete":        rate.OperationTypeWrite,
	"ACL.RoleList":          rate.OperationTypeRead,
	"ACL.RoleRead":          rate.OperationTypeRead,
	"ACL.RoleResolve":       rate.OperationTypeRead,
	"ACL.RoleSet":           rate.OperationTypeWrite,
	"ACL.TokenBatchRead":    rate.OperationTypeRead,
	"ACL.TokenClone":        rate.OperationTypeRead,
	"ACL.TokenDelete":       rate.OperationTypeWrite,
	"ACL.TokenList":         rate.OperationTypeRead,
	"ACL.TokenRead":         rate.OperationTypeRead,
	"ACL.TokenSet":          rate.OperationTypeWrite,

	"AutoConfig.InitialConfiguration": rate.OperationTypeRead,

	"AutoEncrypt.Sign": rate.OperationTypeWrite,

	"Catalog.Deregister":          rate.OperationTypeWrite,
	"Catalog.GatewayServices":     rate.OperationTypeRead,
	"Catalog.ListDatacenters":     rate.OperationTypeRead,
	"Catalog.ListNodes":           rate.OperationTypeRead,
	"Catalog.ListServices":        rate.OperationTypeRead,
	"Catalog.NodeServiceList":     rate.OperationTypeRead,
	"Catalog.NodeServices":        rate.OperationTypeRead,
	"Catalog.Register":            rate.OperationTypeWrite,
	"Catalog.ServiceList":         rate.OperationTypeRead,
	"Catalog.ServiceNodes":        rate.OperationTypeRead,
	"Catalog.VirtualIPForService": rate.OperationTypeRead,

	"ConfigEntry.Apply":                rate.OperationTypeWrite,
	"ConfigEntry.Delete":               rate.OperationTypeWrite,
	"ConfigEntry.Get":                  rate.OperationTypeRead,
	"ConfigEntry.List":                 rate.OperationTypeRead,
	"ConfigEntry.ListAll":              rate.OperationTypeRead,
	"ConfigEntry.ResolveServiceConfig": rate.OperationTypeRead,

	"ConnectCA.ConfigurationGet": rate.OperationTypeRead,
	"ConnectCA.ConfigurationSet": rate.OperationTypeWrite,
	"ConnectCA.Roots":            rate.OperationTypeRead,
	"ConnectCA.Sign":             rate.OperationTypeWrite,
	"ConnectCA.SignIntermediate": rate.OperationTypeWrite,

	"Coordinate.ListDatacenters": rate.OperationTypeRead,
	"Coordinate.ListNodes":       rate.OperationTypeRead,
	"Coordinate.Node":            rate.OperationTypeRead,
	"Coordinate.Update":          rate.OperationTypeWrite,

	"DiscoveryChain.Get": rate.OperationTypeRead,

	"FederationState.Apply":            rate.OperationTypeWrite,
	"FederationState.Delete":           rate.OperationTypeWrite,
	"FederationState.Get":              rate.OperationTypeRead,
	"FederationState.List":             rate.OperationTypeRead,
	"FederationState.ListMeshGateways": rate.OperationTypeRead,

	"Health.ChecksInState": rate.OperationTypeRead,
	"Health.NodeChecks":    rate.OperationTypeRead,
	"Health.ServiceChecks": rate.OperationTypeRead,
	"Health.ServiceNodes":  rate.OperationTypeRead,

	"Intention.Apply": rate.OperationTypeWrite,
	"Intention.Check": rate.OperationTypeRead,
	"Intention.Get":   rate.OperationTypeRead,
	"Intention.List":  rate.OperationTypeRead,
	"Intention.Match": rate.OperationTypeRead,

	"Internal.CatalogOverview":               rate.OperationTypeRead,
	"Internal.EventFire":                     rate.OperationTypeWrite,
	"Internal.ExportedPeeredServices":        rate.OperationTypeRead,
	"Internal.ExportedServicesForPeer":       rate.OperationTypeRead,
	"Internal.GatewayIntentions":             rate.OperationTypeRead,
	"Internal.GatewayServiceDump":            rate.OperationTypeRead,
	"Internal.IntentionUpstreams":            rate.OperationTypeRead,
	"Internal.IntentionUpstreamsDestination": rate.OperationTypeRead,
	"Internal.KeyringOperation":              rate.OperationTypeRead,
	"Internal.NodeDump":                      rate.OperationTypeRead,
	"Internal.NodeInfo":                      rate.OperationTypeRead,
	"Internal.PeeredUpstreams":               rate.OperationTypeRead,
	"Internal.ServiceDump":                   rate.OperationTypeRead,
	"Internal.ServiceGateways":               rate.OperationTypeRead,
	"Internal.ServiceTopology":               rate.OperationTypeRead,

	"KVS.Apply":    rate.OperationTypeWrite,
	"KVS.Get":      rate.OperationTypeRead,
	"KVS.List":     rate.OperationTypeRead,
	"KVS.ListKeys": rate.OperationTypeRead,

	"Operator.AutopilotGetConfiguration": rate.OperationTypeExempt,
	"Operator.AutopilotSetConfiguration": rate.OperationTypeExempt,
	"Operator.AutopilotState":            rate.OperationTypeExempt,
	"Operator.RaftGetConfiguration":      rate.OperationTypeExempt,
	"Operator.RaftRemovePeerByAddress":   rate.OperationTypeExempt,
	"Operator.RaftRemovePeerByID":        rate.OperationTypeExempt,
	"Operator.ServerHealth":              rate.OperationTypeExempt,

	"PreparedQuery.Apply":         rate.OperationTypeWrite,
	"PreparedQuery.Execute":       rate.OperationTypeRead,
	"PreparedQuery.ExecuteRemote": rate.OperationTypeRead,
	"PreparedQuery.Explain":       rate.OperationTypeRead,
	"PreparedQuery.Get":           rate.OperationTypeRead,
	"PreparedQuery.List":          rate.OperationTypeRead,

	"Session.Apply":        rate.OperationTypeWrite,
	"Session.Check":        rate.OperationTypeRead,
	"Session.Get":          rate.OperationTypeRead,
	"Session.List":         rate.OperationTypeRead,
	"Session.NodeSessions": rate.OperationTypeRead,
	"Session.Renew":        rate.OperationTypeWrite,

	"Status.Leader":    rate.OperationTypeExempt,
	"Status.Peers":     rate.OperationTypeExempt,
	"Status.Ping":      rate.OperationTypeExempt,
	"Status.RaftStats": rate.OperationTypeExempt,

	"Txn.Apply": rate.OperationTypeWrite,
	"Txn.Read":  rate.OperationTypeRead,
}
