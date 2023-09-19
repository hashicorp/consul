// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	FailoverPolicyKind = "FailoverPolicy"
)

var (
	FailoverPolicyV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         FailoverPolicyKind,
	}

	FailoverPolicyType = FailoverPolicyV1Alpha1Type
)

func RegisterFailoverPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     FailoverPolicyV1Alpha1Type,
		Proto:    &pbcatalog.FailoverPolicy{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateFailoverPolicy,
		Validate: ValidateFailoverPolicy,
	})
}

func MutateFailoverPolicy(res *pbresource.Resource) error {
	var failover pbcatalog.FailoverPolicy

	if err := res.Data.UnmarshalTo(&failover); err != nil {
		return resource.NewErrDataParse(&failover, err)
	}

	changed := false

	// Handle eliding empty configs.
	if failover.Config != nil && failover.Config.IsEmpty() {
		failover.Config = nil
		changed = true
	}

	if failover.Config != nil {
		if mutateFailoverConfig(res.Id.Tenancy, failover.Config) {
			changed = true
		}
	}

	for port, pc := range failover.PortConfigs {
		if pc.IsEmpty() {
			delete(failover.PortConfigs, port)
			changed = true
		} else {
			if mutateFailoverConfig(res.Id.Tenancy, pc) {
				changed = true
			}
		}
	}
	if len(failover.PortConfigs) == 0 {
		failover.PortConfigs = nil
		changed = true
	}

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&failover)
}

func mutateFailoverConfig(policyTenancy *pbresource.Tenancy, config *pbcatalog.FailoverConfig) (changed bool) {
	if policyTenancy != nil && !isLocalPeer(policyTenancy.PeerName) {
		// TODO(peering/v2): remove this bypass when we know what to do with
		// non-local peer references.
		return false
	}

	for _, dest := range config.Destinations {
		if dest.Ref == nil {
			continue
		}
		if dest.Ref.Tenancy != nil && !isLocalPeer(dest.Ref.Tenancy.PeerName) {
			// TODO(peering/v2): remove this bypass when we know what to do with
			// non-local peer references.
			continue
		}

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
	return p == "local" || p == ""
}

func ValidateFailoverPolicy(res *pbresource.Resource) error {
	var failover pbcatalog.FailoverPolicy

	if err := res.Data.UnmarshalTo(&failover); err != nil {
		return resource.NewErrDataParse(&failover, err)
	}

	var merr error

	if failover.Config == nil && len(failover.PortConfigs) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "config",
			Wrapped: fmt.Errorf("at least one of config or port_configs must be set"),
		})
	}

	if failover.Config != nil {
		wrapConfigErr := func(err error) error {
			return resource.ErrInvalidField{
				Name:    "config",
				Wrapped: err,
			}
		}
		if cfgErr := validateFailoverConfig(failover.Config, false, wrapConfigErr); cfgErr != nil {
			merr = multierror.Append(merr, cfgErr)
		}
	}

	for portName, pc := range failover.PortConfigs {
		wrapConfigErr := func(err error) error {
			return resource.ErrInvalidMapValue{
				Map:     "port_configs",
				Key:     portName,
				Wrapped: err,
			}
		}
		if portNameErr := validatePortName(portName); portNameErr != nil {
			merr = multierror.Append(merr, resource.ErrInvalidMapKey{
				Map:     "port_configs",
				Key:     portName,
				Wrapped: portNameErr,
			})
		}

		if cfgErr := validateFailoverConfig(pc, true, wrapConfigErr); cfgErr != nil {
			merr = multierror.Append(merr, cfgErr)
		}

		// TODO: should sameness group be a ref once that's a resource?
	}

	return merr
}

func validateFailoverConfig(config *pbcatalog.FailoverConfig, ported bool, wrapErr func(error) error) error {
	var merr error

	if (len(config.Destinations) > 0) == (config.SamenessGroup != "") {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name: "destinations",
			// Wrapped: fmt.Errorf("exactly one of destinations or sameness_group should be set"),
			Wrapped: fmt.Errorf("exactly one of destinations or sameness_group should be set: %v || %v",
				(len(config.Destinations) > 0), (config.SamenessGroup != ""),
			),
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

	switch config.Mode {
	case pbcatalog.FailoverMode_FAILOVER_MODE_UNSPECIFIED:
		// means pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL
	case pbcatalog.FailoverMode_FAILOVER_MODE_SEQUENTIAL:
	case pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY:
	default:
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "mode",
			Wrapped: fmt.Errorf("not a supported enum value: %v", config.Mode),
		}))
	}

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
			if portNameErr := validatePortName(dest.Port); portNameErr != nil {
				merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
					Name:    "port",
					Wrapped: portNameErr,
				}))
			}
		} else {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name:    "port",
				Wrapped: fmt.Errorf("ports cannot be specified explicitly for the general failover section since it relies upon port alignment"),
			}))
		}
	}

	hasPeer := false
	if dest.Ref != nil {
		hasPeer = dest.Ref.Tenancy.PeerName != "" && dest.Ref.Tenancy.PeerName != "local"
	}

	if hasPeer && dest.Datacenter != "" {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "datacenter",
			Wrapped: fmt.Errorf("ref.tenancy.peer_name and datacenter are mutually exclusive fields"),
		}))
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
