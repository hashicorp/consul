// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterDestinationPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.DestinationPolicyType,
		Proto:    &pbmesh.DestinationPolicy{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateDestinationPolicy,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookDestinationPolicy,
			Write: aclWriteHookDestinationPolicy,
			List:  resource.NoOpACLListHook,
		},
	})
}

var ValidateDestinationPolicy = resource.DecodeAndValidate(validateDestinationPolicy)

func validateDestinationPolicy(res *DecodedDestinationPolicy) error {
	var merr error

	if len(res.Data.PortConfigs) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "port_configs",
			Wrapped: resource.ErrEmpty,
		})
	}

	for port, pc := range res.Data.PortConfigs {
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

// SimplifyDestinationPolicy normalizes port references in the DestinationPolicy
// using the provided Service.
func SimplifyDestinationPolicy(svc *pbcatalog.Service, policy *pbmesh.DestinationPolicy) *pbmesh.DestinationPolicy {
	if policy == nil {
		panic("destination policy is required")
	}
	if svc == nil {
		panic("service is required")
	}

	// Copy so we can edit it.
	dup := proto.Clone(policy)
	policy = dup.(*pbmesh.DestinationPolicy)

	if policy.PortConfigs == nil {
		policy.PortConfigs = make(map[string]*pbmesh.DestinationConfig)
	}

	// Normalize all port configs to use the target port of the corresponding service port.
	normalizedPortConfigs := make(map[string]*pbmesh.DestinationConfig)
	for port, pc := range policy.PortConfigs {
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

	policy.PortConfigs = normalizedPortConfigs

	return policy
}

func aclReadHookDestinationPolicy(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	// DestinationPolicy is name-aligned with Service
	serviceName := id.Name

	// Check service:read permissions.
	return authorizer.ToAllowAuthorizer().ServiceReadAllowed(serviceName, authzContext)
}

func aclWriteHookDestinationPolicy(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	// DestinationPolicy is name-aligned with Service
	serviceName := res.Id.Name

	// Check service:write permissions on the service this is controlling.
	return authorizer.ToAllowAuthorizer().ServiceWriteAllowed(serviceName, authzContext)
}
