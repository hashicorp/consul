// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	HTTPRouteKind = "HTTPRoute"
)

var (
	HTTPRouteV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         HTTPRouteKind,
	}

	HTTPRouteType = HTTPRouteV1Alpha1Type
)

func RegisterHTTPRoute(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     HTTPRouteV1Alpha1Type,
		Proto:    &pbmesh.HTTPRoute{},
		Mutate:   MutateHTTPRoute,
		Validate: ValidateHTTPRoute,
	})
}

func MutateHTTPRoute(res *pbresource.Resource) error {
	var route pbmesh.HTTPRoute

	if err := res.Data.UnmarshalTo(&route); err != nil {
		return resource.NewErrDataParse(&route, err)
	}

	changed := false

	for _, rule := range route.Rules {
		for _, match := range rule.Matches {
			if match.Method != "" {
				norm := strings.ToUpper(match.Method)
				if match.Method != norm {
					match.Method = norm
					changed = true
				}
			}
		}
	}

	// TODO(rb): normalize parent/backend ref tenancies

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&route)
}

func ValidateHTTPRoute(res *pbresource.Resource) error {
	var route pbmesh.HTTPRoute

	if err := res.Data.UnmarshalTo(&route); err != nil {
		return resource.NewErrDataParse(&route, err)
	}

	var merr error
	if err := validateParentRefs(route.ParentRefs); err != nil {
		merr = multierror.Append(merr, err)
	}

	if len(route.Hostnames) > 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "hostnames",
			Wrapped: errors.New("should not populate hostnames"),
		})
	}

	for i, rule := range route.Rules {
		wrapRuleErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "rules",
				Index:   i,
				Wrapped: err,
			}
		}

		for j, match := range rule.Matches {
			wrapMatchErr := func(err error) error {
				return wrapRuleErr(resource.ErrInvalidListElement{
					Name:    "matches",
					Index:   j,
					Wrapped: err,
				})
			}

			if match.Path != nil {
				wrapMatchPathErr := func(err error) error {
					return wrapMatchErr(resource.ErrInvalidField{
						Name:    "path",
						Wrapped: err,
					})
				}
				switch match.Path.Type {
				case pbmesh.PathMatchType_PATH_MATCH_TYPE_UNSPECIFIED:
					merr = multierror.Append(merr, wrapMatchPathErr(
						resource.ErrInvalidField{
							Name:    "type",
							Wrapped: resource.ErrMissing,
						},
					))
				case pbmesh.PathMatchType_PATH_MATCH_TYPE_EXACT:
					if !strings.HasPrefix(match.Path.Value, "/") {
						merr = multierror.Append(merr, wrapMatchPathErr(
							resource.ErrInvalidField{
								Name:    "value",
								Wrapped: fmt.Errorf("exact patch value does not start with '/': %q", match.Path.Value),
							},
						))
					}
				case pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX:
					if !strings.HasPrefix(match.Path.Value, "/") {
						merr = multierror.Append(merr, wrapMatchPathErr(
							resource.ErrInvalidField{
								Name:    "value",
								Wrapped: fmt.Errorf("prefix patch value does not start with '/': %q", match.Path.Value),
							},
						))
					}
				}
			}

			for k, hdr := range match.Headers {
				wrapMatchHeaderErr := func(err error) error {
					return wrapMatchErr(resource.ErrInvalidListElement{
						Name:    "headers",
						Index:   k,
						Wrapped: err,
					})
				}
				if hdr.Type == pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_UNSPECIFIED {
					merr = multierror.Append(merr, wrapMatchHeaderErr(
						resource.ErrInvalidField{
							Name:    "type",
							Wrapped: resource.ErrMissing,
						}),
					)
				}
				if hdr.Name == "" {
					merr = multierror.Append(merr, wrapMatchHeaderErr(
						resource.ErrInvalidField{
							Name:    "name",
							Wrapped: resource.ErrMissing,
						}),
					)
				}
			}

			for k, qm := range match.QueryParams {
				wrapMatchParamErr := func(err error) error {
					return wrapMatchErr(resource.ErrInvalidListElement{
						Name:    "query_params",
						Index:   k,
						Wrapped: err,
					})
				}
				if qm.Type == pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_UNSPECIFIED {
					merr = multierror.Append(merr, wrapMatchParamErr(
						resource.ErrInvalidField{
							Name:    "type",
							Wrapped: resource.ErrMissing,
						}),
					)
				}
				if qm.Name == "" {
					merr = multierror.Append(merr, wrapMatchParamErr(
						resource.ErrInvalidField{
							Name:    "name",
							Wrapped: resource.ErrMissing,
						}),
					)
				}
			}

			if match.Method != "" && !isValidHTTPMethod(match.Method) {
				merr = multierror.Append(merr, wrapMatchErr(
					resource.ErrInvalidField{
						Name:    "method",
						Wrapped: fmt.Errorf("not a valid http method: %q", match.Method),
					},
				))
			}
		}

		for j, filter := range rule.Filters {
			wrapFilterErr := func(err error) error {
				return wrapRuleErr(resource.ErrInvalidListElement{
					Name:    "filters",
					Index:   j,
					Wrapped: err,
				})
			}
			set := 0
			if filter.RequestHeaderModifier != nil {
				set++
			}
			if filter.ResponseHeaderModifier != nil {
				set++
			}
			if filter.UrlRewrite != nil {
				set++
				if filter.UrlRewrite.PathPrefix == "" {
					merr = multierror.Append(merr, wrapFilterErr(
						resource.ErrInvalidField{
							Name: "url_rewrite",
							Wrapped: resource.ErrInvalidField{
								Name:    "path_prefix",
								Wrapped: errors.New("field should not be empty if enclosing section is set"),
							},
						},
					))
				}
			}
			if set != 1 {
				merr = multierror.Append(merr, wrapFilterErr(
					errors.New("exactly one of request_header_modifier, response_header_modifier, or url_rewrite is required"),
				))
			}
		}

		if len(rule.BackendRefs) == 0 {
			/*
				BackendRefs (optional)¶

				BackendRefs defines API objects where matching requests should be
				sent. If unspecified, the rule performs no forwarding. If
				unspecified and no filters are specified that would result in a
				response being sent, a 404 error code is returned.
			*/
			merr = multierror.Append(merr, wrapRuleErr(
				resource.ErrInvalidField{
					Name:    "backend_refs",
					Wrapped: resource.ErrEmpty,
				},
			))
		}
		for j, hbref := range rule.BackendRefs {
			wrapBackendRefErr := func(err error) error {
				return wrapRuleErr(resource.ErrInvalidListElement{
					Name:    "backend_refs",
					Index:   j,
					Wrapped: err,
				})
			}

			for _, err := range validateBackendRef(hbref.BackendRef) {
				merr = multierror.Append(merr, wrapBackendRefErr(
					resource.ErrInvalidField{
						Name:    "backend_ref",
						Wrapped: err,
					},
				))
			}

			if len(hbref.Filters) > 0 {
				merr = multierror.Append(merr, wrapBackendRefErr(
					resource.ErrInvalidField{
						Name:    "filters",
						Wrapped: errors.New("filters are not supported at this level yet"),
					},
				))
			}
		}

		if rule.Timeouts != nil {
			for _, err := range validateHTTPTimeouts(rule.Timeouts) {
				merr = multierror.Append(merr, wrapRuleErr(
					resource.ErrInvalidField{
						Name:    "timeouts",
						Wrapped: err,
					},
				))
			}
		}
		if rule.Retries != nil {
			for _, err := range validateHTTPRetries(rule.Retries) {
				merr = multierror.Append(merr, wrapRuleErr(
					resource.ErrInvalidField{
						Name:    "retries",
						Wrapped: err,
					},
				))
			}
		}
	}

	return merr
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

func isValidHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return true
	default:
		return false
	}
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
