// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fsm

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func IsEnterpriseData(namespace, partition string) bool {
	if (namespace != "" && namespace != "default") || (partition != "" && partition != "default") {
		return true
	}
	return false
}

var errIncompatibleTenantedData = errors.New("incompatible tenanted data")
var ErrDroppingTenantedReq = errors.New("dropping tenanted request")

func decodeRegistration(buf []byte, req *structs.RegisterRequest) error {
	type serviceRequest struct {
		Namespace string
		Partition string
		*structs.NodeService
	}
	type checkRequest struct {
		Namespace string
		Partition string
		*structs.HealthCheck
	}
	type NewRegReq struct {

		// shadows the Service field from the register request so that we can detect
		// tenanted service registrations for untenanted nodes
		Service *serviceRequest

		// shadows the Check field from the register request so that we can detect
		// tenanted check registrations for untenanted nodes.
		Check *checkRequest

		// shadows the Checks field for the same reasons as the singular version.
		Checks []*checkRequest

		// Allows parsing the namespace of the whole request/node
		Namespace string

		// Allows parsing the partition of the whole request/node
		Partition string
		*structs.RegisterRequest
	}
	var newReq NewRegReq
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	// checks if the node is tenanted
	if IsEnterpriseData(newReq.Namespace, newReq.Partition) {
		// the whole request can be dropped because the node itself is tenanted
		return ErrDroppingTenantedReq
	}

	// check if the service is tenanted
	if newReq.Service != nil && !IsEnterpriseData(newReq.Service.Namespace, newReq.Service.Partition) {
		// copy the shadow service pointer into the real RegisterRequest
		newReq.RegisterRequest.Service = newReq.Service.NodeService
	}

	// check if the singular check is tenanted
	if newReq.Check != nil && !IsEnterpriseData(newReq.Check.Namespace, newReq.Check.Partition) {
		newReq.RegisterRequest.Check = newReq.Check.HealthCheck
	}

	// check for tenanted checks in the slice
	for _, chk := range newReq.Checks {
		if !IsEnterpriseData(chk.Namespace, chk.Partition) {
			newReq.RegisterRequest.Checks = append(newReq.RegisterRequest.Checks, chk.HealthCheck)
		}
	}
	// copy the data to the output request value
	*req = *newReq.RegisterRequest
	return nil
}

func decodeDeregistration(buf []byte, req *structs.DeregisterRequest) error {
	type NewDeRegReq struct {
		Namespace string

		// Allows parsing the partition of the whole request/node
		Partition string

		*structs.DeregisterRequest

		// Allows parsing the namespace of the whole request/node

	}
	var newReq NewDeRegReq
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	// checks if the node is tenanted
	if IsEnterpriseData(newReq.Namespace, newReq.Partition) {
		// the whole request can be dropped because the node itself is tenanted
		return ErrDroppingTenantedReq
	}

	// copy the data to the output request value
	*req = *newReq.DeregisterRequest
	return nil
}

func decodeKVS(buf []byte, req *structs.KVSRequest) error {
	type dirEntryReq struct {
		Namespace string
		Partition string
		*structs.DirEntry
	}
	type NewDirEntReq struct {
		// shadows the DirEnt field from  KVSRequest  so that we can detect
		// tenanted service registrations for untenanted nodes
		DirEnt *dirEntryReq
		*structs.KVSRequest
	}
	var newReq NewDirEntReq
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	if newReq.DirEnt != nil && IsEnterpriseData(newReq.DirEnt.Namespace, newReq.DirEnt.Partition) {
		return ErrDroppingTenantedReq
	}

	newReq.KVSRequest.DirEnt = *newReq.DirEnt.DirEntry
	*req = *newReq.KVSRequest
	return nil
}

func decodeSession(buf []byte, req *structs.SessionRequest) error {
	type sessionReq struct {
		Namespace string
		Partition string
		*structs.Session
	}
	type NewSessionReq struct {
		// shadows the Session field from  SessionRequest  so that we can detect
		// tenanted service registrations for untenanted nodes
		Session *sessionReq
		*structs.SessionRequest
	}
	var newReq NewSessionReq
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	if newReq.Session != nil && IsEnterpriseData(newReq.Session.Namespace, newReq.Session.Partition) {
		return ErrDroppingTenantedReq

	}
	serviceChecks := newReq.Session.ServiceChecks
	newReq.Session.ServiceChecks = nil
	for _, sessionServiceCheck := range serviceChecks {
		if !IsEnterpriseData(sessionServiceCheck.Namespace, "") {
			newReq.Session.ServiceChecks = append(newReq.Session.ServiceChecks, sessionServiceCheck)
		}
	}

	newReq.SessionRequest.Session = *newReq.Session.Session
	*req = *newReq.SessionRequest
	return nil
}

func decodePreparedQuery(buf []byte, req *structs.PreparedQueryRequest) error {
	type serviceQuery struct {
		Namespace string
		Partition string
		*structs.ServiceQuery
	}
	type prepQuery struct {
		Service *serviceQuery
		*structs.PreparedQuery
	}
	type NewPreparedQueryReq struct {
		Query *prepQuery
		*structs.PreparedQueryRequest
	}
	var newReq NewPreparedQueryReq
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	if newReq.Query != nil && newReq.Query.Service != nil && IsEnterpriseData(newReq.Query.Service.Namespace, newReq.Query.Service.Partition) {
		return ErrDroppingTenantedReq
	}

	newReq.Query.PreparedQuery.Service = *newReq.Query.Service.ServiceQuery
	newReq.PreparedQueryRequest.Query = newReq.Query.PreparedQuery
	*req = *newReq.PreparedQueryRequest
	return nil
}

func decodeTxn(buf []byte, req *structs.TxnRequest) error {
	type dirEntryReq struct {
		Namespace string
		Partition string
		*structs.DirEntry
	}
	type txnKVOp struct {
		DirEnt *dirEntryReq
		*structs.TxnKVOp
	}
	type nodeService struct {
		Namespace string
		Partition string
		*structs.NodeService
	}
	type txnServiceOp struct {
		Service *nodeService
		*structs.TxnServiceOp
	}
	type healthCheck struct {
		Namespace string
		Partition string
		*structs.HealthCheck
	}
	type txnCheckOp struct {
		Check *healthCheck
		*structs.TxnCheckOp
	}
	type session struct {
		Namespace string
		Partition string
		*structs.Session
	}
	type txnSessionOp struct {
		Session *session
		*structs.TxnSessionOp
	}
	// Only one of the types should be filled out per entry.
	type txnOp struct {
		KV      *txnKVOp
		Service *txnServiceOp
		Check   *txnCheckOp
		Session *txnSessionOp
		*structs.TxnOp
	}
	type NewTxnRequest struct {
		Ops []*txnOp
		*structs.TxnRequest
	}
	var newReq NewTxnRequest
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}
	for _, op := range newReq.Ops {
		if op.KV != nil && op.KV.DirEnt != nil && !IsEnterpriseData(op.KV.DirEnt.Namespace, op.KV.DirEnt.Partition) {
			txnOp := &structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb:   op.KV.Verb,
					DirEnt: *op.KV.DirEnt.DirEntry,
				},
			}
			newReq.TxnRequest.Ops = append(newReq.TxnRequest.Ops, txnOp)
			continue
		}

		if op.Service != nil && op.Service.Service != nil && !IsEnterpriseData(op.Service.Service.Namespace, op.Service.Service.Partition) {
			txnOp := &structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb:    op.Service.Verb,
					Node:    op.Service.Node,
					Service: *op.Service.Service.NodeService,
				},
			}
			newReq.TxnRequest.Ops = append(newReq.TxnRequest.Ops, txnOp)
			continue
		}

		if op.Check != nil && op.Check.Check != nil && !IsEnterpriseData(op.Check.Check.Namespace, op.Check.Check.Partition) {
			txnOp := &structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb:  op.Check.Verb,
					Check: *op.Check.Check.HealthCheck,
				},
			}
			newReq.TxnRequest.Ops = append(newReq.TxnRequest.Ops, txnOp)
			continue
		}

		if op.Session != nil && op.Session.Session != nil && !IsEnterpriseData(op.Session.Session.Namespace, op.Session.Session.Partition) {
			txnOp := &structs.TxnOp{
				Session: &structs.TxnSessionOp{
					Verb:    op.Session.Verb,
					Session: *op.Session.Session.Session,
				},
			}
			txnOp.Session.Session.ServiceChecks = nil
			for _, sessionServiceCheck := range op.Session.Session.ServiceChecks {
				if !IsEnterpriseData(sessionServiceCheck.Namespace, "") {
					txnOp.Session.Session.ServiceChecks = append(txnOp.Session.Session.ServiceChecks, sessionServiceCheck)
				}
			}
			newReq.TxnRequest.Ops = append(newReq.TxnRequest.Ops, txnOp)
		}
	}

	*req = *newReq.TxnRequest
	return nil
}

func decodeACLTokenBatchSet(buf []byte, req *structs.ACLTokenBatchSetRequest) error {
	type aclToken struct {
		Namespace string
		Partition string
		*structs.ACLToken
	}
	type NewACLTokenBatchSetRequest struct {
		Tokens []*aclToken
		*structs.ACLTokenBatchSetRequest
	}
	var newReq NewACLTokenBatchSetRequest
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	for _, token := range newReq.Tokens {
		if !IsEnterpriseData(token.Namespace, token.Partition) {
			newReq.ACLTokenBatchSetRequest.Tokens = append(newReq.ACLTokenBatchSetRequest.Tokens, token.ACLToken)
		}
	}

	*req = *newReq.ACLTokenBatchSetRequest
	return nil

}

func decodeACLPolicyBatchSet(buf []byte, req *structs.ACLPolicyBatchSetRequest) error {
	type aclPolicy struct {
		Namespace string
		Partition string
		*structs.ACLPolicy
	}
	type NewACLPolicyBatchSetRequest struct {
		Policies []*aclPolicy
		*structs.ACLPolicyBatchSetRequest
	}
	var newReq NewACLPolicyBatchSetRequest
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}
	if newReq.ACLPolicyBatchSetRequest == nil {
		newReq.ACLPolicyBatchSetRequest = &structs.ACLPolicyBatchSetRequest{}
	}
	for _, policy := range newReq.Policies {
		if !IsEnterpriseData(policy.Namespace, policy.Partition) {
			newReq.ACLPolicyBatchSetRequest.Policies = append(newReq.ACLPolicyBatchSetRequest.Policies, policy.ACLPolicy)
		}
	}

	*req = *newReq.ACLPolicyBatchSetRequest
	return nil

}

func decodeACLRoleBatchSet(buf []byte, req *structs.ACLRoleBatchSetRequest) error {
	type aclRole struct {
		Namespace string
		Partition string
		*structs.ACLRole
	}
	type NewACLRoleBatchSetRequest struct {
		Roles []*aclRole
		*structs.ACLRoleBatchSetRequest
	}
	var newReq NewACLRoleBatchSetRequest
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	for _, role := range newReq.Roles {
		if !IsEnterpriseData(role.Namespace, role.Partition) {
			newReq.ACLRoleBatchSetRequest.Roles = append(newReq.ACLRoleBatchSetRequest.Roles, role.ACLRole)
		}
	}

	*req = *newReq.ACLRoleBatchSetRequest
	return nil
}

func decodeACLBindingRuleBatchSet(buf []byte, req *structs.ACLBindingRuleBatchSetRequest) error {
	type aCLBindingRule struct {
		Namespace string
		Partition string
		*structs.ACLBindingRule
	}
	type NewACLBindingRuleBatchSetRequest struct {
		BindingRules []*aCLBindingRule
		*structs.ACLBindingRuleBatchSetRequest
	}
	var newReq NewACLBindingRuleBatchSetRequest
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}
	if newReq.ACLBindingRuleBatchSetRequest == nil {
		newReq.ACLBindingRuleBatchSetRequest = &structs.ACLBindingRuleBatchSetRequest{}
	}
	for _, rule := range newReq.BindingRules {
		if !IsEnterpriseData(rule.Namespace, rule.Partition) {
			newReq.ACLBindingRuleBatchSetRequest.BindingRules = append(newReq.ACLBindingRuleBatchSetRequest.BindingRules, rule.ACLBindingRule)
		}
	}

	*req = *newReq.ACLBindingRuleBatchSetRequest
	return nil
}

func decodeACLAuthMethodBatchSet(buf []byte, req *structs.ACLAuthMethodBatchSetRequest) error {
	type aCLAuthMethod struct {
		Namespace string
		Partition string
		*structs.ACLAuthMethod
	}
	type NewACLAuthMethodBatchSetRequest struct {
		AuthMethods []*aCLAuthMethod
		*structs.ACLAuthMethodBatchSetRequest
	}
	var newReq NewACLAuthMethodBatchSetRequest
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}
	if newReq.ACLAuthMethodBatchSetRequest == nil {
		newReq.ACLAuthMethodBatchSetRequest = &structs.ACLAuthMethodBatchSetRequest{}
	}
	for _, authMethod := range newReq.AuthMethods {
		if !IsEnterpriseData(authMethod.Namespace, authMethod.Partition) {
			newReq.ACLAuthMethodBatchSetRequest.AuthMethods = append(newReq.ACLAuthMethodBatchSetRequest.AuthMethods, authMethod.ACLAuthMethod)
		}
	}

	*req = *newReq.ACLAuthMethodBatchSetRequest
	return nil
}

func decodeACLAuthMethodBatchDelete(buf []byte, req *structs.ACLAuthMethodBatchDeleteRequest) error {
	type NewACLAuthMethodBatchDeleteRequest struct {
		Namespace string
		Partition string
		*structs.ACLAuthMethodBatchDeleteRequest
	}

	var newReq NewACLAuthMethodBatchDeleteRequest
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	if IsEnterpriseData(newReq.Namespace, newReq.Partition) {
		return ErrDroppingTenantedReq
	}

	*req = *newReq.ACLAuthMethodBatchDeleteRequest
	return nil
}

func decodeServiceVirtualIP(buf []byte, req *state.ServiceVirtualIP) error {
	type serviceName struct {
		Namespace string
		Partition string
		*structs.ServiceName
	}
	type peeredServiceName struct {
		ServiceName *serviceName
		*structs.PeeredServiceName
	}
	type NewServiceVirtualIP struct {
		Service *peeredServiceName
		*state.ServiceVirtualIP
	}
	var newReq NewServiceVirtualIP
	if err := structs.Decode(buf, &newReq); err != nil {
		return err
	}

	if newReq.Service != nil && newReq.Service.ServiceName != nil && IsEnterpriseData(newReq.Service.ServiceName.Namespace, newReq.Service.ServiceName.Partition) {
		return ErrDroppingTenantedReq
	}
	newReq.ServiceVirtualIP.Service.ServiceName = *newReq.Service.ServiceName.ServiceName
	*req = *newReq.ServiceVirtualIP
	return nil
}

func decodePeeringWrite(buf []byte, req *pbpeering.PeeringWriteRequest) error {
	if err := structs.DecodeProto(buf, req); err != nil {
		return err
	}

	if req.Peering != nil && IsEnterpriseData("", req.Peering.Partition) {
		return ErrDroppingTenantedReq
	}

	return nil
}

func decodePeeringDelete(buf []byte, req *pbpeering.PeeringDeleteRequest) error {
	if err := structs.DecodeProto(buf, req); err != nil {
		return err
	}

	if IsEnterpriseData("", req.Partition) {
		return ErrDroppingTenantedReq
	}

	return nil
}

func decodePeeringTrustBundleWrite(buf []byte, req *pbpeering.PeeringTrustBundleWriteRequest) error {
	if err := structs.DecodeProto(buf, req); err != nil {
		return err
	}

	if IsEnterpriseData("", req.PeeringTrustBundle.Partition) {
		return ErrDroppingTenantedReq
	}

	return nil
}

func decodePeeringTrustBundleDelete(buf []byte, req *pbpeering.PeeringTrustBundleDeleteRequest) error {
	if err := structs.DecodeProto(buf, req); err != nil {
		return err
	}

	if IsEnterpriseData("", req.Partition) {
		return ErrDroppingTenantedReq
	}

	return nil
}

func decodeConfigEntryOperation(buf []byte, req *structs.ConfigEntryRequest) error {

	newReq := &ShadowConfigEntryRequest{
		ConfigEntryRequest: req,
	}
	if err := structs.Decode(buf, newReq); err != nil {
		return err
	}
	shadowConfigEntry := newReq.ConfigEntryRequest.Entry.(ShadowConfigentry)
	if err := shadowConfigEntry.CheckEnt(); err != nil {
		return err
	}
	req.Entry = shadowConfigEntry.GetRealConfigEntry()
	return nil
}

type ShadowConfigEntryRequest struct {
	*structs.ConfigEntryRequest
}

func (c *ShadowConfigEntryRequest) UnmarshalBinary(data []byte) error {
	// First decode the kind prefix
	var kind string
	dec := codec.NewDecoderBytes(data, structs.MsgpackHandle)
	if err := dec.Decode(&kind); err != nil {
		return err
	}

	// Then decode the real thing with appropriate kind of ConfigEntry
	entry, err := MakeShadowConfigEntry(kind, "")
	if err != nil {
		return err
	}
	c.Entry = entry
	// Alias juggling to prevent infinite recursive calls back to this decode
	// method.
	type Alias structs.ConfigEntryRequest
	as := struct {
		*Alias
	}{
		Alias: (*Alias)(c.ConfigEntryRequest),
	}
	if err := dec.Decode(&as); err != nil {
		return err
	}
	return nil
}
func MakeShadowConfigEntry(kind, name string) (structs.ConfigEntry, error) {
	switch kind {
	case structs.RateLimitIPConfig:
		return nil, ErrDroppingTenantedReq
	case structs.ServiceDefaults:
		return &ShadowServiceConfigEntry{ServiceConfigEntry: &structs.ServiceConfigEntry{Name: name}}, nil
	case structs.ProxyDefaults:
		return &ShadowProxyConfigEntry{ProxyConfigEntry: &structs.ProxyConfigEntry{Name: name}}, nil
	case structs.ServiceRouter:
		return &ShadowServiceRouterConfigEntry{ServiceRouterConfigEntry: &structs.ServiceRouterConfigEntry{Name: name}}, nil
	case structs.ServiceSplitter:
		return &ShadowServiceSplitterConfigEntry{ServiceSplitterConfigEntry: &structs.ServiceSplitterConfigEntry{Name: name}}, nil
	case structs.ServiceResolver:
		return &ShadowServiceResolverConfigEntry{ServiceResolverConfigEntry: &structs.ServiceResolverConfigEntry{Name: name}}, nil
	case structs.IngressGateway:
		return &ShadowIngressGatewayConfigEntry{IngressGatewayConfigEntry: &structs.IngressGatewayConfigEntry{Name: name}}, nil
	case structs.TerminatingGateway:
		return &ShadowTerminatingGatewayConfigEntry{TerminatingGatewayConfigEntry: &structs.TerminatingGatewayConfigEntry{Name: name}}, nil
	case structs.ServiceIntentions:
		return &ShadowServiceIntentionsConfigEntry{ServiceIntentionsConfigEntry: &structs.ServiceIntentionsConfigEntry{Name: name}}, nil
	case structs.MeshConfig:
		return &ShadowMeshConfigEntry{MeshConfigEntry: &structs.MeshConfigEntry{}}, nil
	case structs.ExportedServices:
		return &ShadowExportedServicesConfigEntry{ExportedServicesConfigEntry: &structs.ExportedServicesConfigEntry{Name: name}}, nil
	case structs.SamenessGroup:
		return &ShadowSamenessGroupConfigEntry{SamenessGroupConfigEntry: &structs.SamenessGroupConfigEntry{Name: name}}, nil
	case structs.APIGateway:
		return &ShadowAPIGatewayConfigEntry{APIGatewayConfigEntry: &structs.APIGatewayConfigEntry{Name: name}}, nil
	case structs.BoundAPIGateway:
		return &ShadowBoundAPIGatewayConfigEntry{BoundAPIGatewayConfigEntry: &structs.BoundAPIGatewayConfigEntry{Name: name}}, nil
	case structs.FileSystemCertificate:
		return &ShadowFileSystemCertificateConfigEntry{FileSystemCertificateConfigEntry: &structs.FileSystemCertificateConfigEntry{Name: name}}, nil
	case structs.InlineCertificate:
		return &ShadowInlineCertificateConfigEntry{InlineCertificateConfigEntry: &structs.InlineCertificateConfigEntry{Name: name}}, nil
	case structs.HTTPRoute:
		return &ShadowHTTPRouteConfigEntry{HTTPRouteConfigEntry: &structs.HTTPRouteConfigEntry{Name: name}}, nil
	case structs.TCPRoute:
		return &ShadowTCPRouteConfigEntry{TCPRouteConfigEntry: &structs.TCPRouteConfigEntry{Name: name}}, nil
	case structs.JWTProvider:
		return &ShadowJWTProviderConfigEntry{JWTProviderConfigEntry: &structs.JWTProviderConfigEntry{Name: name}}, nil
	default:
		return nil, fmt.Errorf("invalid config entry kind: %s", kind)
	}
}

type ShadowBase struct {
	Namespace string
	Partition string
}

func (s ShadowBase) CheckEnt() error {
	if IsEnterpriseData(s.Namespace, s.Partition) {
		return ErrDroppingTenantedReq
	}
	return nil
}

type ShadowConfigentry interface {
	CheckEnt() error
	GetRealConfigEntry() structs.ConfigEntry
}

type ShadowProxyConfigEntry struct {
	ShadowBase
	*structs.ProxyConfigEntry
}

func (s ShadowProxyConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.ProxyConfigEntry
}

type ShadowServiceResolverConfigEntry struct {
	ShadowBase
	*structs.ServiceResolverConfigEntry
}

func (s ShadowServiceResolverConfigEntry) CheckEnt() error {
	if err := s.ShadowBase.CheckEnt(); err != nil {
		return err
	}
	if s.ServiceResolverConfigEntry.Redirect != nil && (IsEnterpriseData(s.ServiceResolverConfigEntry.Redirect.Namespace, s.ServiceResolverConfigEntry.Redirect.Partition) || s.ServiceResolverConfigEntry.Redirect.SamenessGroup != "") {
		return errIncompatibleTenantedData
	}
	for _, failover := range s.ServiceResolverConfigEntry.Failover {
		if IsEnterpriseData(failover.Namespace, "") || failover.SamenessGroup != "" {
			return errIncompatibleTenantedData
		}
		for _, target := range failover.Targets {
			if IsEnterpriseData(target.Namespace, target.Partition) {
				return errIncompatibleTenantedData
			}
		}
	}
	return nil
}

func (s ShadowServiceResolverConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.ServiceResolverConfigEntry
}

func (e *ShadowProxyConfigEntry) UnmarshalBinary(data []byte) error {
	// The goal here is to add a post-decoding operation to
	// decoding of a ProxyConfigEntry. The cleanest way I could
	// find to do so was to implement the BinaryMarshaller interface
	// and use a type alias to do the original round of decoding,
	// followed by a MapWalk of the Config to coerce everything
	// into JSON compatible types.
	type Alias structs.ProxyConfigEntry
	as := struct {
		*ShadowBase
		*Alias
	}{
		ShadowBase: &e.ShadowBase,
		Alias:      (*Alias)(e.ProxyConfigEntry),
	}
	dec := codec.NewDecoderBytes(data, structs.MsgpackHandle)
	if err := dec.Decode(&as); err != nil {
		return err
	}
	config, err := lib.MapWalk(e.Config)
	if err != nil {
		return err
	}
	e.Config = config
	return nil
}

type ShadowUpstreamConfig struct {
	ShadowBase
	*structs.UpstreamConfig
}
type ShadowUpstreamConfiguration struct {
	Overrides []*ShadowUpstreamConfig
	*structs.UpstreamConfiguration
}
type ShadowServiceConfigEntry struct {
	ShadowBase
	UpstreamConfig *ShadowUpstreamConfiguration
	*structs.ServiceConfigEntry
}

func (s ShadowServiceConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	if s.UpstreamConfig != nil {
		for _, override := range s.UpstreamConfig.Overrides {
			if !IsEnterpriseData(override.Namespace, override.Partition) {
				if s.ServiceConfigEntry.UpstreamConfig == nil {
					s.ServiceConfigEntry.UpstreamConfig = &structs.UpstreamConfiguration{}
				}
				s.ServiceConfigEntry.UpstreamConfig.Overrides = append(s.ServiceConfigEntry.UpstreamConfig.Overrides, override.UpstreamConfig)
			}
		}
	}
	return s.ServiceConfigEntry
}

type ShadowServiceRouterConfigEntry struct {
	ShadowBase
	*structs.ServiceRouterConfigEntry
}

func (s ShadowServiceRouterConfigEntry) CheckEnt() error {
	if err := s.ShadowBase.CheckEnt(); err != nil {
		return err
	}
	for _, route := range s.ServiceRouterConfigEntry.Routes {
		if IsEnterpriseData(route.Destination.Namespace, route.Destination.Partition) {
			return errIncompatibleTenantedData
		}
	}
	return nil
}

func (s ShadowServiceRouterConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.ServiceRouterConfigEntry
}

type ShadowServiceSplitterConfigEntry struct {
	ShadowBase
	*structs.ServiceSplitterConfigEntry
}

func (s ShadowServiceSplitterConfigEntry) CheckEnt() error {
	if err := s.ShadowBase.CheckEnt(); err != nil {
		return err
	}
	for _, split := range s.ServiceSplitterConfigEntry.Splits {
		if IsEnterpriseData(split.Namespace, split.Partition) {
			return errIncompatibleTenantedData
		}
	}
	return nil
}
func (s ShadowServiceSplitterConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.ServiceSplitterConfigEntry
}

type ShadowIngressService struct {
	ShadowBase
	*structs.IngressService
}
type ShadowIngressListener struct {
	Services []ShadowIngressService
	*structs.IngressListener
}
type ShadowIngressGatewayConfigEntry struct {
	ShadowBase
	Listeners []ShadowIngressListener
	*structs.IngressGatewayConfigEntry
}

func (s ShadowIngressGatewayConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	for _, listner := range s.Listeners {
		for _, svc := range listner.Services {
			if !IsEnterpriseData(svc.Namespace, svc.Partition) {
				listner.IngressListener.Services = append(listner.IngressListener.Services, *svc.IngressService)
			}
		}
		if len(listner.IngressListener.Services) == 0 {
			continue
		}
		s.IngressGatewayConfigEntry.Listeners = append(s.IngressGatewayConfigEntry.Listeners, *listner.IngressListener)
	}
	return s.IngressGatewayConfigEntry
}

type ShadowLinkedService struct {
	ShadowBase
	*structs.LinkedService
}

type ShadowTerminatingGatewayConfigEntry struct {
	ShadowBase
	Services []ShadowLinkedService
	*structs.TerminatingGatewayConfigEntry
}

func (s ShadowTerminatingGatewayConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	for _, svc := range s.Services {
		if !IsEnterpriseData(svc.Namespace, svc.Partition) {
			s.TerminatingGatewayConfigEntry.Services = append(s.TerminatingGatewayConfigEntry.Services, *svc.LinkedService)
		}
	}
	return s.TerminatingGatewayConfigEntry
}

type ShadowSourceIntention struct {
	ShadowBase
	*structs.SourceIntention
}
type ShadowServiceIntentionsConfigEntry struct {
	ShadowBase
	Sources []*ShadowSourceIntention
	*structs.ServiceIntentionsConfigEntry
}

func (s ShadowServiceIntentionsConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	for _, source := range s.Sources {
		if !IsEnterpriseData(source.Namespace, source.Partition) && source.SamenessGroup == "" {
			s.ServiceIntentionsConfigEntry.Sources = append(s.ServiceIntentionsConfigEntry.Sources, source.SourceIntention)
		}
	}
	return s.ServiceIntentionsConfigEntry
}

type ShadowMeshConfigEntry struct {
	ShadowBase
	*structs.MeshConfigEntry
}

func (s ShadowMeshConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.MeshConfigEntry
}

type ShadowExportedServicesConfigEntry struct {
	ShadowBase
	*structs.ExportedServicesConfigEntry
}

func (s ShadowExportedServicesConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	services := []structs.ExportedService{}
	for _, svc := range s.ExportedServicesConfigEntry.Services {
		if !IsEnterpriseData(svc.Namespace, "") {
			consumers := []structs.ServiceConsumer{}
			for _, consumer := range svc.Consumers {
				if !IsEnterpriseData("", consumer.Partition) && consumer.SamenessGroup == "" {
					consumers = append(consumers, consumer)
				}
			}
			if len(consumers) == 0 {
				continue
			}
			services = append(services, svc)
		}
	}
	s.ExportedServicesConfigEntry.Services = services
	return s.ExportedServicesConfigEntry
}

type ShadowSamenessGroupConfigEntry struct {
	ShadowBase
	*structs.SamenessGroupConfigEntry
}

func (s ShadowSamenessGroupConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.SamenessGroupConfigEntry
}

type ShadowAPIGatewayConfigEntry struct {
	ShadowBase
	*structs.APIGatewayConfigEntry
}

func (s ShadowAPIGatewayConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.APIGatewayConfigEntry
}

type ShadowBoundAPIGatewayListener struct {
	Routes       []ShadowResourceReference
	Certificates []ShadowResourceReference
	*structs.BoundAPIGatewayListener
}
type ShadowBoundAPIGatewayConfigEntry struct {
	ShadowBase
	Listeners []ShadowBoundAPIGatewayListener
	*structs.BoundAPIGatewayConfigEntry
}

func (s ShadowBoundAPIGatewayConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	for _, listner := range s.Listeners {
		for _, route := range listner.Routes {
			if !IsEnterpriseData(route.Namespace, route.Partition) {
				listner.BoundAPIGatewayListener.Routes = append(listner.BoundAPIGatewayListener.Routes, *route.ResourceReference)
			}
		}
		for _, cf := range listner.Certificates {
			if !IsEnterpriseData(cf.Namespace, cf.Partition) {
				listner.BoundAPIGatewayListener.Certificates = append(listner.BoundAPIGatewayListener.Certificates, *cf.ResourceReference)
			}
		}
		s.BoundAPIGatewayConfigEntry.Listeners = append(s.BoundAPIGatewayConfigEntry.Listeners, *listner.BoundAPIGatewayListener)
	}
	return s.BoundAPIGatewayConfigEntry
}

type ShadowFileSystemCertificateConfigEntry struct {
	ShadowBase
	*structs.FileSystemCertificateConfigEntry
}

func (s ShadowFileSystemCertificateConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.FileSystemCertificateConfigEntry
}

type ShadowInlineCertificateConfigEntry struct {
	ShadowBase
	*structs.InlineCertificateConfigEntry
}

func (s ShadowInlineCertificateConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.InlineCertificateConfigEntry
}

type ShadowHTTPService struct {
	ShadowBase
	*structs.HTTPService
}
type ShadowHTTPRouteRule struct {
	Services []ShadowHTTPService
	*structs.HTTPRouteRule
}
type ShadowResourceReference struct {
	ShadowBase
	*structs.ResourceReference
}
type ShadowHTTPRouteConfigEntry struct {
	ShadowBase
	Parents []ShadowResourceReference
	Rules   []ShadowHTTPRouteRule
	*structs.HTTPRouteConfigEntry
}

func (s ShadowHTTPRouteConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	for _, parent := range s.Parents {
		if !IsEnterpriseData(parent.Namespace, parent.Partition) {
			s.HTTPRouteConfigEntry.Parents = append(s.HTTPRouteConfigEntry.Parents, *parent.ResourceReference)
		}
	}
	for _, rule := range s.Rules {
		for _, svc := range rule.Services {
			if !IsEnterpriseData(svc.Namespace, svc.Partition) {
				rule.HTTPRouteRule.Services = append(rule.HTTPRouteRule.Services, *svc.HTTPService)
			}
		}
		s.HTTPRouteConfigEntry.Rules = append(s.HTTPRouteConfigEntry.Rules, *rule.HTTPRouteRule)
	}
	return s.HTTPRouteConfigEntry
}

type ShadowTCPService struct {
	ShadowBase
	*structs.TCPService
}
type ShadowTCPRouteConfigEntry struct {
	ShadowBase
	Parents  []ShadowResourceReference
	Services []ShadowTCPService
	*structs.TCPRouteConfigEntry
}

func (s ShadowTCPRouteConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	for _, parent := range s.Parents {
		if !IsEnterpriseData(parent.Namespace, parent.Partition) {
			s.TCPRouteConfigEntry.Parents = append(s.TCPRouteConfigEntry.Parents, *parent.ResourceReference)
		}
	}
	for _, svc := range s.Services {
		if !IsEnterpriseData(svc.Namespace, svc.Partition) {
			s.TCPRouteConfigEntry.Services = append(s.TCPRouteConfigEntry.Services, *svc.TCPService)
		}
	}
	return s.TCPRouteConfigEntry
}

type ShadowJWTProviderConfigEntry struct {
	ShadowBase
	*structs.JWTProviderConfigEntry
}

func (s ShadowJWTProviderConfigEntry) GetRealConfigEntry() structs.ConfigEntry {
	return s.JWTProviderConfigEntry
}
