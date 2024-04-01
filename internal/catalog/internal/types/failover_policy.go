// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedFailoverPolicy = resource.DecodedResource[*pbcatalog.FailoverPolicy]

func RegisterFailoverPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.FailoverPolicyType,
		Proto:    &pbcatalog.FailoverPolicy{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateFailoverPolicy,
		Validate: ValidateFailoverPolicy,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookFailoverPolicy,
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookFailoverPolicy),
			List:  resource.NoOpACLListHook,
		},
	})
}

var MutateFailoverPolicy = resource.DecodeAndMutate(mutateFailoverPolicy)

func mutateFailoverPolicy(res *DecodedFailoverPolicy) (bool, error) {
	changed := false

	// Handle eliding empty configs.
	if res.Data.Config != nil && res.Data.Config.IsEmpty() {
		res.Data.Config = nil
		changed = true
	}

	if res.Data.Config != nil {
		if mutateFailoverConfig(res.Id.Tenancy, res.Data.Config) {
			changed = true
		}
	}

	for port, pc := range res.Data.PortConfigs {
		if pc.IsEmpty() {
			delete(res.Data.PortConfigs, port)
			changed = true
		} else {
			if mutateFailoverConfig(res.Id.Tenancy, pc) {
				changed = true
			}
		}
	}
	if len(res.Data.PortConfigs) == 0 {
		res.Data.PortConfigs = nil
		changed = true
	}

	return changed, nil
}

func mutateFailoverConfig(policyTenancy *pbresource.Tenancy, config *pbcatalog.FailoverConfig) (changed bool) {
	// TODO(peering/v2): Add something here when we know what to do with non-local peer references

	for _, dest := range config.Destinations {
		if dest.Ref == nil {
			continue
		}

		// TODO(peering/v2): Add something here to handle non-local peer references

		orig := proto.Clone(dest.Ref).(*pbresource.Reference)
		resource.DefaultReferenceTenancy(
			dest.Ref,
			policyTenancy,
			resource.DefaultNamespacedTenancy(), // Services are all namespace scoped.
		)

		if !proto.Equal(orig, dest.Ref) {
			changed = true
		}
	}

	return changed
}

func isLocalPeer(p string) bool {
	return p == resource.DefaultPeerName || p == ""
}

var ValidateFailoverPolicy = resource.DecodeAndValidate(validateFailoverPolicy)

func validateFailoverPolicy(res *DecodedFailoverPolicy) error {
	var merr error

	if res.Data.Config == nil && len(res.Data.PortConfigs) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "config",
			Wrapped: fmt.Errorf("at least one of config or port_configs must be set"),
		})
	}

	if err := validateCommonFailoverConfigs(res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}
	return merr
}

func validateCommonFailoverConfigs(res *pbcatalog.FailoverPolicy) error {
	var merr error
	if res.Config != nil {
		wrapConfigErr := func(err error) error {
			return resource.ErrInvalidField{
				Name:    "config",
				Wrapped: err,
			}
		}
		if cfgErr := validateFailoverConfig(res.Config, false, wrapConfigErr); cfgErr != nil {
			merr = multierror.Append(merr, cfgErr)
		}
	}

	for portId, pc := range res.PortConfigs {
		wrapConfigErr := func(err error) error {
			return resource.ErrInvalidMapValue{
				Map:     "port_configs",
				Key:     portId,
				Wrapped: err,
			}
		}
		if portIdErr := ValidateServicePortID(portId); portIdErr != nil {
			merr = multierror.Append(merr, resource.ErrInvalidMapKey{
				Map:     "port_configs",
				Key:     portId,
				Wrapped: portIdErr,
			})
		}

		if cfgErr := validateFailoverConfig(pc, true, wrapConfigErr); cfgErr != nil {
			merr = multierror.Append(merr, cfgErr)
		}

	}

	return merr
}

func validateFailoverConfig(config *pbcatalog.FailoverConfig, ported bool, wrapErr func(error) error) error {
	var merr error

	if len(config.Regions) > 0 {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "regions",
			Wrapped: fmt.Errorf("not supported in this release"),
		}))
	}

	// TODO(peering/v2): remove this bypass when we know what to do with

	if (len(config.Destinations) > 0) == (config.SamenessGroup != "") {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "destinations",
			Wrapped: fmt.Errorf("exactly one of destinations or sameness_group should be set"),
		}))
	}
	for i, dest := range config.Destinations {
		wrapDestErr := func(err error) error {
			return wrapErr(resource.ErrInvalidListElement{
				Name:    "destinations",
				Index:   i,
				Wrapped: err,
			})
		}
		if destErr := validateFailoverPolicyDestination(dest, ported, wrapDestErr); destErr != nil {
			merr = multierror.Append(merr, destErr)
		}
	}

	if config.Mode != pbcatalog.FailoverMode_FAILOVER_MODE_UNSPECIFIED {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "mode",
			Wrapped: fmt.Errorf("not supported in this release"),
		}))
	}

	// TODO(v2): uncomment after this is supported
	// switch config.Mode {
	// case pbcatalog.FailoverMode_FAILOVER_MODE_UNSPECIFIED:
	// 	// means pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL
	// case pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL:
	// case pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY:
	// default:
	// 	merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
	// 		Name:    "mode",
	// 		Wrapped: fmt.Errorf("not a supported enum value: %v", config.Mode),
	// 	}))
	// }

	// TODO: validate sameness group requirements

	return merr
}

func validateFailoverPolicyDestination(dest *pbcatalog.FailoverDestination, ported bool, wrapErr func(error) error) error {
	var merr error

	wrapRefErr := func(err error) error {
		return wrapErr(resource.ErrInvalidField{
			Name:    "ref",
			Wrapped: err,
		})
	}

	if refErr := ValidateLocalServiceRefNoSection(dest.Ref, wrapRefErr); refErr != nil {
		merr = multierror.Append(merr, refErr)
	}

	// NOTE: Destinations here cannot define ports. Port equality is
	// assumed and will be reconciled.
	if dest.Port != "" {
		if ported {
			if portIdErr := ValidateServicePortID(dest.Port); portIdErr != nil {
				merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
					Name:    "port",
					Wrapped: portIdErr,
				}))
			}
		} else {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name:    "port",
				Wrapped: fmt.Errorf("ports cannot be specified explicitly for the general failover section since it relies upon port alignment"),
			}))
		}
	}

	return merr
}

// SimplifyFailoverPolicy fully populates the PortConfigs map and clears the
// Configs map using the provided Service.
func SimplifyFailoverPolicy(svc *pbcatalog.Service, failover *pbcatalog.FailoverPolicy) *pbcatalog.FailoverPolicy {
	if failover == nil {
		panic("failover is required")
	}
	if svc == nil {
		panic("service is required")
	}

	// Copy so we can edit it.
	dup := proto.Clone(failover)
	failover = dup.(*pbcatalog.FailoverPolicy)

	if failover.PortConfigs == nil {
		failover.PortConfigs = make(map[string]*pbcatalog.FailoverConfig)
	}

	// Normalize all port configs to use the target port of the corresponding service port.
	normalizedPortConfigs := make(map[string]*pbcatalog.FailoverConfig)
	for port, pc := range failover.PortConfigs {
		svcPort := svc.FindPortByID(port)

		if svcPort != nil {
			if _, ok := normalizedPortConfigs[svcPort.TargetPort]; ok {
				// This is a duplicate virtual and target port mapping that will be reported as a status condition.
				// Only update if this is the "canonical" mapping; otherwise, it's virtual, and we should ignore.
				if port != svcPort.TargetPort {
					continue
				}
			}
			normalizedPortConfigs[svcPort.TargetPort] = pc
		}
		// Else this is an invalid reference that will be reported as a status condition.
		// Drop for safety and simpler output.
	}

	failover.PortConfigs = normalizedPortConfigs

	for _, port := range svc.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			continue // skip
		}

		if pc, ok := failover.PortConfigs[port.TargetPort]; ok {
			for i, dest := range pc.Destinations {
				// Assume port alignment.
				if dest.Port == "" {
					dest.Port = port.TargetPort
					pc.Destinations[i] = dest
				}
			}
			continue
		}

		if failover.Config != nil {
			// Duplicate because each port will get this uniquely.
			pc2 := proto.Clone(failover.Config).(*pbcatalog.FailoverConfig)
			for _, dest := range pc2.Destinations {
				dest.Port = port.TargetPort
			}
			failover.PortConfigs[port.TargetPort] = pc2
		}
	}

	if failover.Config != nil {
		failover.Config = nil
	}

	return failover
}

func aclReadHookFailoverPolicy(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	// FailoverPolicy is name-aligned with Service
	serviceName := id.Name

	// Check service:read permissions.
	return authorizer.ToAllowAuthorizer().ServiceReadAllowed(serviceName, authzContext)
}

func aclWriteHookFailoverPolicy(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedFailoverPolicy) error {
	// FailoverPolicy is name-aligned with Service
	serviceName := res.Id.Name

	// Check service:write permissions on the service this is controlling.
	if err := authorizer.ToAllowAuthorizer().ServiceWriteAllowed(serviceName, authzContext); err != nil {
		return err
	}

	// Ensure you have service:read on any destination that may be affected by
	// traffic FROM this config change.
	if res.Data.Config != nil {
		for _, dest := range res.Data.Config.Destinations {
			destAuthzContext := resource.AuthorizerContext(dest.Ref.GetTenancy())
			destServiceName := dest.Ref.GetName()
			if err := authorizer.ToAllowAuthorizer().ServiceReadAllowed(destServiceName, destAuthzContext); err != nil {
				return err
			}
		}
	}
	for _, pc := range res.Data.PortConfigs {
		for _, dest := range pc.Destinations {
			destAuthzContext := resource.AuthorizerContext(dest.Ref.GetTenancy())
			destServiceName := dest.Ref.GetName()
			if err := authorizer.ToAllowAuthorizer().ServiceReadAllowed(destServiceName, destAuthzContext); err != nil {
				return err
			}
		}
	}

	return nil

}
