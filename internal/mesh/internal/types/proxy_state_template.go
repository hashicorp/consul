// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterProxyStateTemplate(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.ProxyStateTemplateType,
		Proto:    &pbmesh.ProxyStateTemplate{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateProxyStateTemplate,
		ACLs: &resource.ACLHooks{
			Read: func(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
				// Check service:read and operator:read permissions.
				// If service:read is not allowed, check operator:read. We want to allow both as this
				// resource is mostly useful for debuggability and we want to cover
				// the most cases that serve that purpose.
				serviceReadErr := authorizer.ToAllowAuthorizer().ServiceReadAllowed(id.Name, authzContext)
				operatorReadErr := authorizer.ToAllowAuthorizer().OperatorReadAllowed(authzContext)

				switch {
				case serviceReadErr != nil:
					return serviceReadErr
				case operatorReadErr != nil:
					return operatorReadErr
				}

				return nil
			},
			Write: func(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, p *pbresource.Resource) error {
				// Require operator:write only for "break-glass" scenarios as this resource should be mostly
				// managed by a controller.
				return authorizer.ToAllowAuthorizer().OperatorWriteAllowed(authzContext)
			},
			List: func(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
				// No-op List permission as we want to default to filtering resources
				// from the list using the Read enforcement.
				return nil
			},
		},
	})
}

func ValidateProxyStateTemplate(res *pbresource.Resource) error {
	// TODO(v2): validate a lot more of this

	var pst pbmesh.ProxyStateTemplate

	if err := res.Data.UnmarshalTo(&pst); err != nil {
		return resource.NewErrDataParse(&pst, err)
	}

	var merr error

	if pst.ProxyState != nil {
		wrapProxyStateErr := func(err error) error {
			return resource.ErrInvalidField{
				Name:    "proxy_state",
				Wrapped: err,
			}
		}
		for name, cluster := range pst.ProxyState.Clusters {
			if name == "" {
				merr = multierror.Append(merr, wrapProxyStateErr(resource.ErrInvalidMapKey{
					Map:     "clusters",
					Key:     name,
					Wrapped: resource.ErrEmpty,
				}))
				continue
			}

			wrapClusterErr := func(err error) error {
				return wrapProxyStateErr(resource.ErrInvalidMapValue{
					Map:     "clusters",
					Key:     name,
					Wrapped: err,
				})
			}

			if name != cluster.Name {
				merr = multierror.Append(merr, wrapClusterErr(resource.ErrInvalidField{
					Name:    "name",
					Wrapped: fmt.Errorf("cluster name %q does not match map key %q", cluster.Name, name),
				}))
			}

			wrapGroupErr := func(err error) error {
				return wrapClusterErr(resource.ErrInvalidField{
					Name:    "group",
					Wrapped: err,
				})
			}

			if cluster.Group == nil {
				merr = multierror.Append(merr, wrapGroupErr(resource.ErrMissing))
			} else {
				switch x := cluster.Group.(type) {
				case *pbproxystate.Cluster_EndpointGroup:
					wrapInnerGroupErr := func(err error) error {
						return wrapGroupErr(resource.ErrInvalidField{
							Name:    "endpoint_group",
							Wrapped: err,
						})
					}

					if x.EndpointGroup == nil {
						merr = multierror.Append(merr, wrapInnerGroupErr(resource.ErrMissing))
						continue
					}

					// The inner name field is optional, but if specified it has to
					// match the enclosing cluster.

					if x.EndpointGroup.Name != "" && x.EndpointGroup.Name != cluster.Name {
						merr = multierror.Append(merr, wrapInnerGroupErr(resource.ErrInvalidField{
							Name: "name",
							Wrapped: fmt.Errorf("optional but %q does not match enclosing cluster name %q",
								x.EndpointGroup.Name, cluster.Name),
						}))
					}

				case *pbproxystate.Cluster_FailoverGroup:
					wrapInnerGroupErr := func(err error) error {
						return wrapGroupErr(resource.ErrInvalidField{
							Name:    "failover_group",
							Wrapped: err,
						})
					}

					if x.FailoverGroup == nil {
						merr = multierror.Append(merr, wrapInnerGroupErr(resource.ErrMissing))
						continue
					}

					if len(x.FailoverGroup.EndpointGroups) == 0 {
						merr = multierror.Append(merr, wrapInnerGroupErr(resource.ErrInvalidField{
							Name:    "endpoint_groups",
							Wrapped: resource.ErrEmpty,
						}))
					}

					for i, eg := range x.FailoverGroup.EndpointGroups {
						wrapFailoverEndpointGroupErr := func(err error) error {
							return wrapInnerGroupErr(resource.ErrInvalidListElement{
								Name:    "endpoint_groups",
								Index:   i,
								Wrapped: err,
							})
						}
						// The inner name field is required and cannot match the enclosing cluster.
						switch {
						case eg.Name == "":
							merr = multierror.Append(merr, wrapFailoverEndpointGroupErr(resource.ErrInvalidField{
								Name:    "name",
								Wrapped: resource.ErrEmpty,
							}))
						case eg.Name == cluster.Name:
							merr = multierror.Append(merr, wrapFailoverEndpointGroupErr(resource.ErrInvalidField{
								Name: "name",
								Wrapped: fmt.Errorf(
									"name cannot be the same as the enclosing cluster %q",
									eg.Name,
								),
							}))
						}
					}
				default:
					merr = multierror.Append(merr, wrapGroupErr(fmt.Errorf("unknown type: %T", cluster.Group)))
				}
			}
		}
	}

	return merr
}
