// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	DestinationPolicyKind = "DestinationPolicy"
)

var (
	DestinationPolicyV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         DestinationPolicyKind,
	}

	DestinationPolicyType = DestinationPolicyV1Alpha1Type
)

func RegisterDestinationPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     DestinationPolicyV1Alpha1Type,
		Proto:    &pbmesh.DestinationPolicy{},
		Validate: ValidateDestinationPolicy,
		Scope:    resource.ScopeNamespace,
	})
}

func ValidateDestinationPolicy(res *pbresource.Resource) error {
	var policy pbmesh.DestinationPolicy

	if err := res.Data.UnmarshalTo(&policy); err != nil {
		return resource.NewErrDataParse(&policy, err)
	}

	var merr error

	if len(policy.PortConfigs) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "port_configs",
			Wrapped: resource.ErrEmpty,
		})
	}

	for port, pc := range policy.PortConfigs {
		wrapErr := func(err error) error {
			return resource.ErrInvalidMapValue{
				Map:     "port_configs",
				Key:     port,
				Wrapped: err,
			}
		}

		if dur := pc.ConnectTimeout.AsDuration(); dur < 0 {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name:    "connect_timeout",
				Wrapped: fmt.Errorf("'%v', must be >= 0", dur),
			}))
		}
		if dur := pc.RequestTimeout.AsDuration(); dur < 0 {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name:    "request_timeout",
				Wrapped: fmt.Errorf("'%v', must be >= 0", dur),
			}))
		}

		if pc.LoadBalancer != nil {
			lb := pc.LoadBalancer
			wrapLBErr := func(err error) error {
				return wrapErr(resource.ErrInvalidField{
					Name:    "load_balancer",
					Wrapped: err,
				})
			}

			switch lb.Policy {
			case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_UNSPECIFIED:
				// means just do the default
			case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RANDOM:
			case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_ROUND_ROBIN:
			case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST:
			case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV:
			case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH:
			default:
				merr = multierror.Append(merr, wrapLBErr(resource.ErrInvalidField{
					Name:    "policy",
					Wrapped: fmt.Errorf("not a supported enum value: %v", lb.Policy),
				}))
			}

			if lb.Policy != pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH && lb.Config != nil {
				if _, ok := lb.Config.(*pbmesh.LoadBalancer_RingHashConfig); ok {
					merr = multierror.Append(merr, wrapLBErr(resource.ErrInvalidField{
						Name:    "config",
						Wrapped: fmt.Errorf("ring_hash_config specified for incompatible load balancing policy %q", lb.Policy),
					}))
				}
			}

			if lb.Policy != pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST && lb.Config != nil {
				if _, ok := lb.Config.(*pbmesh.LoadBalancer_LeastRequestConfig); ok {
					merr = multierror.Append(merr, wrapLBErr(resource.ErrInvalidField{
						Name:    "config",
						Wrapped: fmt.Errorf("least_request_config specified for incompatible load balancing policy %q", lb.Policy),
					}))
				}
			}

			if !lb.Policy.IsHashBased() && len(lb.HashPolicies) > 0 {
				merr = multierror.Append(merr, wrapLBErr(resource.ErrInvalidField{
					Name:    "hash_policies",
					Wrapped: fmt.Errorf("hash_policies specified for non-hash-based policy %q", lb.Policy),
				}))
			}

		LOOP:
			for i, hp := range lb.HashPolicies {
				wrapHPErr := func(err error) error {
					return wrapLBErr(resource.ErrInvalidListElement{
						Name:    "hash_policies",
						Index:   i,
						Wrapped: err,
					})
				}

				var hasField bool
				switch hp.Field {
				case pbmesh.HashPolicyField_HASH_POLICY_FIELD_UNSPECIFIED:
				case pbmesh.HashPolicyField_HASH_POLICY_FIELD_HEADER,
					pbmesh.HashPolicyField_HASH_POLICY_FIELD_COOKIE,
					pbmesh.HashPolicyField_HASH_POLICY_FIELD_QUERY_PARAMETER:
					hasField = true
				default:
					merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
						Name:    "field",
						Wrapped: fmt.Errorf("not a supported enum value: %v", hp.Field),
					}))
					continue LOOP // no need to keep validating
				}

				if hp.SourceIp {
					if hasField {
						merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
							Name:    "field",
							Wrapped: fmt.Errorf("a single hash policy cannot hash both a source address and a %q", hp.Field),
						}))
					}
					if hp.FieldValue != "" {
						merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
							Name:    "field_value",
							Wrapped: errors.New("cannot be specified when hashing source_ip"),
						}))
					}
				}

				if hasField && hp.FieldValue == "" {
					merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
						Name:    "field_value",
						Wrapped: fmt.Errorf("field %q was specified without a field_value", hp.Field),
					}))
				}
				if hp.FieldValue != "" && !hasField {
					merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
						Name:    "field",
						Wrapped: resource.ErrMissing,
					}))
					merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
						Name:    "field_value",
						Wrapped: errors.New("requires a field to apply to"),
					}))
				}
				if hp.CookieConfig != nil {
					if hp.Field != pbmesh.HashPolicyField_HASH_POLICY_FIELD_COOKIE {
						merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
							Name:    "cookie_config",
							Wrapped: fmt.Errorf("incompatible with field %q", hp.Field),
						}))
					}
					if hp.CookieConfig.Session && hp.CookieConfig.Ttl.AsDuration() != 0 {
						merr = multierror.Append(merr, wrapHPErr(resource.ErrInvalidField{
							Name: "cookie_config",
							Wrapped: resource.ErrInvalidField{
								Name:    "ttl",
								Wrapped: fmt.Errorf("a session cookie cannot have an associated TTL"),
							},
						}))
					}
				}
			}
		}

		if pc.LocalityPrioritization != nil {
			lp := pc.LocalityPrioritization
			wrapLPErr := func(err error) error {
				return wrapErr(resource.ErrInvalidField{
					Name:    "locality_prioritization",
					Wrapped: err,
				})
			}

			switch lp.Mode {
			case pbmesh.LocalityPrioritizationMode_LOCALITY_PRIORITIZATION_MODE_UNSPECIFIED:
				// means pbmesh.LocalityPrioritizationMode_LOCALITY_PRIORITIZATION_MODE_NONE
			case pbmesh.LocalityPrioritizationMode_LOCALITY_PRIORITIZATION_MODE_NONE:
			case pbmesh.LocalityPrioritizationMode_LOCALITY_PRIORITIZATION_MODE_FAILOVER:
			default:
				merr = multierror.Append(merr, wrapLPErr(resource.ErrInvalidField{
					Name:    "mode",
					Wrapped: fmt.Errorf("not a supported enum value: %v", lp.Mode),
				}))
			}
		}
	}

	return merr
}
