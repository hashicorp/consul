// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
				return wrapErr(resource.ErrInvalidMapValue{
					Map:     "load_balancer",
					Key:     port,
					Wrapped: err,
				})
			}

			if lb.Policy != pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH && lb.Config != nil {
				if _, ok := lb.Config.(*pbmesh.LoadBalancer_RingHashConfig); ok {
					merr = multierror.Append(merr, wrapLBErr(resource.ErrInvalidField{
						Name:    "config",
						Wrapped: fmt.Errorf("RingHashConfig specified for incompatible load balancing policy %q", lb.Policy),
					}))
				}
			}

			if lb.Policy != pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST && lb.Config != nil {
				if _, ok := lb.Config.(*pbmesh.LoadBalancer_LeastRequestConfig); ok {
					merr = multierror.Append(merr, wrapLBErr(resource.ErrInvalidField{
						Name:    "config",
						Wrapped: fmt.Errorf("LeastRequestConfig specified for incompatible load balancing policy %q", lb.Policy),
					}))
				}
			}

			if !lb.Policy.IsHashBased() && len(lb.HashPolicies) > 0 {
				merr = multierror.Append(merr, wrapLBErr(resource.ErrInvalidField{
					Name:    "hash_policies",
					Wrapped: fmt.Errorf("hash_policies specified for non-hash-based policy %q", lb.Policy),
				}))
			}

			for i, hp := range lb.HashPolicies {
				wrapHPErr := func(err error) error {
					return wrapLBErr(resource.ErrInvalidListElement{
						Name:    "hash_policies",
						Index:   i,
						Wrapped: err,
					})
				}

				hasField := (hp.Field != pbmesh.HashPolicyField_HASH_POLICY_FIELD_UNSPECIFIED)

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
	}

	return merr
}
