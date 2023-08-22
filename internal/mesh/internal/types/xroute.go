// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
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
		if parent.Ref == nil {
			merr = multierror.Append(merr, wrapErr(
				resource.ErrInvalidField{
					Name:    "ref",
					Wrapped: resource.ErrMissing,
				},
			))
		} else {
			if !IsServiceType(parent.Ref.Type) {
				merr = multierror.Append(merr, wrapErr(
					resource.ErrInvalidField{
						Name: "ref",
						Wrapped: resource.ErrInvalidReferenceType{
							AllowedType: catalog.ServiceType,
						},
					},
				))
			}
			if parent.Ref.Section != "" {
				merr = multierror.Append(merr, wrapErr(
					resource.ErrInvalidField{
						Name: "ref",
						Wrapped: resource.ErrInvalidField{
							Name:    "section",
							Wrapped: errors.New("section not supported for service parent refs"),
						},
					},
				))
			}

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
							Name: "ref",
							Wrapped: fmt.Errorf(
								"parent ref %q for wildcard port exists twice",
								resource.ReferenceToString(parent.Ref),
							),
						},
					))
				} else if exactExists { // check for existing exact
					merr = multierror.Append(merr, wrapErr(
						resource.ErrInvalidField{
							Name: "ref",
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
							Name: "ref",
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
							Name: "ref",
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

func validateBackendRef(backendRef *pbmesh.BackendReference) []error {
	var errs []error
	if backendRef == nil {
		errs = append(errs, resource.ErrMissing)

	} else if backendRef.Ref == nil {
		errs = append(errs, resource.ErrInvalidField{
			Name:    "ref",
			Wrapped: resource.ErrMissing,
		})

	} else {
		if !IsServiceType(backendRef.Ref.Type) {
			errs = append(errs, resource.ErrInvalidField{
				Name: "ref",
				Wrapped: resource.ErrInvalidReferenceType{
					AllowedType: catalog.ServiceType,
				},
			})
		}

		if backendRef.Ref.Section != "" {
			errs = append(errs, resource.ErrInvalidField{
				Name: "ref",
				Wrapped: resource.ErrInvalidField{
					Name:    "section",
					Wrapped: errors.New("section not supported for service backend refs"),
				},
			})
		}

		if backendRef.Datacenter != "" {
			errs = append(errs, resource.ErrInvalidField{
				Name:    "datacenter",
				Wrapped: errors.New("datacenter is not yet supported on backend refs"),
			})
		}
	}
	return errs
}

func validateHeaderMatchType(typ pbmesh.HeaderMatchType) error {
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
				Wrapped: fmt.Errorf("timeout cannot be negative: %v", val),
			})
		}
	}
	if timeouts.BackendRequest != nil {
		val := timeouts.BackendRequest.AsDuration()
		if val < 0 {
			errs = append(errs, resource.ErrInvalidField{
				Name:    "backend_request",
				Wrapped: fmt.Errorf("timeout cannot be negative: %v", val),
			})
		}
	}
	if timeouts.Idle != nil {
		val := timeouts.Idle.AsDuration()
		if val < 0 {
			errs = append(errs, resource.ErrInvalidField{
				Name:    "idle",
				Wrapped: fmt.Errorf("timeout cannot be negative: %v", val),
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

	if retries.Number < 0 {
		errs = append(errs, resource.ErrInvalidField{
			Name:    "number",
			Wrapped: fmt.Errorf("cannot be negative: %v", retries.Number),
		})
	}

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
