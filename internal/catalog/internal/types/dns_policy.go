// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"math"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
)

type DecodedDNSPolicy = resource.DecodedResource[*pbcatalog.DNSPolicy]

func RegisterDNSPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.DNSPolicyType,
		Proto:    &pbcatalog.DNSPolicy{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateDNSPolicy,
		ACLs:     ACLHooksForWorkloadSelectingType[*pbcatalog.DNSPolicy](),
	})
}

var ValidateDNSPolicy = resource.DecodeAndValidate(validateDNSPolicy)

func validateDNSPolicy(res *DecodedDNSPolicy) error {
	var err error
	// Ensure that this resource isn't useless and is attempting to
	// select at least one workload.
	if selErr := ValidateSelector(res.Data.Workloads, false); selErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	// Validate the weights
	if weightErr := validateDNSPolicyWeights(res.Data.Weights); weightErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "weights",
			Wrapped: weightErr,
		})
	}

	return err
}

func validateDNSPolicyWeights(weights *pbcatalog.Weights) error {
	// Non nil weights are required
	if weights == nil {
		return resource.ErrMissing
	}

	var err error
	if weights.Passing < 1 || weights.Passing > math.MaxUint16 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "passing",
			Wrapped: errDNSPassingWeightOutOfRange,
		})
	}

	// Each weight is an unsigned integer so we don't need to
	// check for negative weights.
	if weights.Warning > math.MaxUint16 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "warning",
			Wrapped: errDNSWarningWeightOutOfRange,
		})
	}

	return err
}
