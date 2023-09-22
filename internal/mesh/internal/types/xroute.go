// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type XRouteData interface {
	proto.Message
	XRouteWithRefs
}

type XRouteWithRefs interface {
	GetParentRefs() []*pbmesh.ParentReference
	GetUnderlyingBackendRefs() []*pbmesh.BackendReference
}

type portedRefKey struct {
	Key  resource.ReferenceKey
	Port string
}

func mutateParentRefs(xrouteTenancy *pbresource.Tenancy, parentRefs []*pbmesh.ParentReference) (changed bool) {
	for _, parent := range parentRefs {
		if parent.Ref == nil {
			continue
		}
		changedThis := mutateXRouteRef(xrouteTenancy, parent.Ref)
		if changedThis {
			changed = true
		}
	}
	return changed
}

func mutateXRouteRef(xrouteTenancy *pbresource.Tenancy, ref *pbresource.Reference) (changed bool) {
	if ref == nil {
		return false
	}
	orig := proto.Clone(ref).(*pbresource.Reference)
	resource.DefaultReferenceTenancy(
		ref,
		xrouteTenancy,
		resource.DefaultNamespacedTenancy(), // All xRoutes are namespace scoped.
	)

	return !proto.Equal(orig, ref)
}

func validateParentRefs(parentRefs []*pbmesh.ParentReference) error {
	var merr error
	if len(parentRefs) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "parent_refs",
			Wrapped: resource.ErrEmpty,
		})
	}

	var (
		seen    = make(map[portedRefKey]struct{})
		seenAny = make(map[resource.ReferenceKey][]string)
	)
	for i, parent := range parentRefs {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "parent_refs",
				Index:   i,
				Wrapped: err,
			}
		}

		wrapRefErr := func(err error) error {
			return wrapErr(resource.ErrInvalidField{
				Name:    "ref",
				Wrapped: err,
			})
		}

		if err := catalog.ValidateLocalServiceRefNoSection(parent.Ref, wrapRefErr); err != nil {
			merr = multierror.Append(merr, err)
		} else {
			prk := portedRefKey{
				Key:  resource.NewReferenceKey(parent.Ref),
				Port: parent.Port,
			}

			_, portExist := seen[prk]

			if parent.Port == "" {
				coveredPorts, exactExists := seenAny[prk.Key]

				if portExist { // check for duplicate wild
					merr = multierror.Append(merr, wrapErr(
						resource.ErrInvalidField{
							Name: "port",
							Wrapped: fmt.Errorf(
								"parent ref %q for wildcard port exists twice",
								resource.ReferenceToString(parent.Ref),
							),
						},
					))
				} else if exactExists { // check for existing exact
					merr = multierror.Append(merr, wrapErr(
						resource.ErrInvalidField{
							Name: "port",
							Wrapped: fmt.Errorf(
								"parent ref %q for ports %v covered by wildcard port already",
								resource.ReferenceToString(parent.Ref),
								coveredPorts,
							),
						},
					))
				} else {
					seen[prk] = struct{}{}
				}

			} else {
				prkWild := prk
				prkWild.Port = ""
				_, wildExist := seen[prkWild]

				if portExist { // check for duplicate exact
					merr = multierror.Append(merr, wrapErr(
						resource.ErrInvalidField{
							Name: "port",
							Wrapped: fmt.Errorf(
								"parent ref %q for port %q exists twice",
								resource.ReferenceToString(parent.Ref),
								parent.Port,
							),
						},
					))
				} else if wildExist { // check for existing wild
					merr = multierror.Append(merr, wrapErr(
						resource.ErrInvalidField{
							Name: "port",
							Wrapped: fmt.Errorf(
								"parent ref %q for port %q covered by wildcard port already",
								resource.ReferenceToString(parent.Ref),
								parent.Port,
							),
						},
					))
				} else {
					seen[prk] = struct{}{}
					seenAny[prk.Key] = append(seenAny[prk.Key], parent.Port)
				}
			}
		}
	}

	return merr
}

func validateBackendRef(backendRef *pbmesh.BackendReference, wrapErr func(error) error) error {
	if backendRef == nil {
		return wrapErr(resource.ErrMissing)
	}

	var merr error

	wrapRefErr := func(err error) error {
		return wrapErr(resource.ErrInvalidField{
			Name:    "ref",
			Wrapped: err,
		})
	}
	if err := catalog.ValidateLocalServiceRefNoSection(backendRef.Ref, wrapRefErr); err != nil {
		merr = multierror.Append(merr, err)
	}
	if backendRef.Datacenter != "" {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "datacenter",
			Wrapped: errors.New("datacenter is not yet supported on backend refs"),
		}))
	}

	return merr
}

func validateHeaderMatchType(typ pbmesh.HeaderMatchType) error {
	// enumcover:pbmesh.HeaderMatchType
	switch typ {
	case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_UNSPECIFIED:
		return resource.ErrMissing
	case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT:
	case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_REGEX:
	case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_PRESENT:
	case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_PREFIX:
	case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_SUFFIX:
	default:
		return fmt.Errorf("not a supported enum value: %v", typ)
	}
	return nil
}

func errTimeoutCannotBeNegative(d time.Duration) error {
	return fmt.Errorf("timeout cannot be negative: %v", d)
}

func validateHTTPTimeouts(timeouts *pbmesh.HTTPRouteTimeouts) []error {
	if timeouts == nil {
		return nil
	}

	var errs []error

	if timeouts.Request != nil {
		val := timeouts.Request.AsDuration()
		if val < 0 {
			errs = append(errs, resource.ErrInvalidField{
				Name:    "request",
				Wrapped: errTimeoutCannotBeNegative(val),
			})
		}
	}
	if timeouts.Idle != nil {
		val := timeouts.Idle.AsDuration()
		if val < 0 {
			errs = append(errs, resource.ErrInvalidField{
				Name:    "idle",
				Wrapped: errTimeoutCannotBeNegative(val),
			})
		}
	}

	return errs
}

func validateHTTPRetries(retries *pbmesh.HTTPRouteRetries) []error {
	if retries == nil {
		return nil
	}

	var errs []error

	for i, condition := range retries.OnConditions {
		if !isValidRetryCondition(condition) {
			errs = append(errs, resource.ErrInvalidListElement{
				Name:    "on_conditions",
				Index:   i,
				Wrapped: fmt.Errorf("not a valid retry condition: %q", condition),
			})
		}
	}

	return errs
}

func isValidRetryCondition(retryOn string) bool {
	switch retryOn {
	case "5xx",
		"gateway-error",
		"reset",
		"connect-failure",
		"envoy-ratelimited",
		"retriable-4xx",
		"refused-stream",
		"cancelled",
		"deadline-exceeded",
		"internal",
		"resource-exhausted",
		"unavailable":
		return true
	default:
		return false
	}
}

func xRouteACLHooks[R XRouteData]() *resource.ACLHooks {
	hooks := &resource.ACLHooks{
		Read:  aclReadHookXRoute[R],
		Write: aclWriteHookXRoute[R],
		List:  aclListHookXRoute[R],
	}

	return hooks
}

func aclReadHookXRoute[R XRouteData](authorizer acl.Authorizer, _ *acl.AuthorizerContext, _ *pbresource.ID, res *pbresource.Resource) error {
	if res == nil {
		return resource.ErrNeedData
	}

	dec, err := resource.Decode[R](res)
	if err != nil {
		return err
	}

	route := dec.Data

	// Need service:read on ALL of the services this is controlling traffic for.
	for _, parentRef := range route.GetParentRefs() {
		parentAuthzContext := resource.AuthorizerContext(parentRef.Ref.GetTenancy())
		parentServiceName := parentRef.Ref.GetName()

		if err := authorizer.ToAllowAuthorizer().ServiceReadAllowed(parentServiceName, parentAuthzContext); err != nil {
			return err
		}
	}

	return nil
}

func aclWriteHookXRoute[R XRouteData](authorizer acl.Authorizer, _ *acl.AuthorizerContext, res *pbresource.Resource) error {
	dec, err := resource.Decode[R](res)
	if err != nil {
		return err
	}

	route := dec.Data

	// Need service:write on ALL of the services this is controlling traffic for.
	for _, parentRef := range route.GetParentRefs() {
		parentAuthzContext := resource.AuthorizerContext(parentRef.Ref.GetTenancy())
		parentServiceName := parentRef.Ref.GetName()

		if err := authorizer.ToAllowAuthorizer().ServiceWriteAllowed(parentServiceName, parentAuthzContext); err != nil {
			return err
		}
	}

	// Need service:read on ALL of the services this directs traffic at.
	for _, backendRef := range route.GetUnderlyingBackendRefs() {
		backendAuthzContext := resource.AuthorizerContext(backendRef.Ref.GetTenancy())
		backendServiceName := backendRef.Ref.GetName()

		if err := authorizer.ToAllowAuthorizer().ServiceReadAllowed(backendServiceName, backendAuthzContext); err != nil {
			return err
		}
	}

	return nil
}

func aclListHookXRoute[R XRouteData](authorizer acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	// No-op List permission as we want to default to filtering resources
	// from the list using the Read enforcement.
	return nil
}
