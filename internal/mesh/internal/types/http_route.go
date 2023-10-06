// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterHTTPRoute(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.HTTPRouteType,
		Proto:    &pbmesh.HTTPRoute{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateHTTPRoute,
		Validate: ValidateHTTPRoute,
		ACLs:     xRouteACLHooks[*pbmesh.HTTPRoute](),
	})
}

func MutateHTTPRoute(res *pbresource.Resource) error {
	var route pbmesh.HTTPRoute

	if err := res.Data.UnmarshalTo(&route); err != nil {
		return resource.NewErrDataParse(&route, err)
	}

	changed := false

	if mutateParentRefs(res.Id.Tenancy, route.ParentRefs) {
		changed = true
	}

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
		for _, backend := range rule.BackendRefs {
			if backend.BackendRef == nil || backend.BackendRef.Ref == nil {
				continue
			}
			if mutateXRouteRef(res.Id.Tenancy, backend.BackendRef.Ref) {
				changed = true
			}
		}
	}

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
				// enumcover:pbmesh.PathMatchType
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
				case pbmesh.PathMatchType_PATH_MATCH_TYPE_REGEX:
					if match.Path.Value == "" {
						merr = multierror.Append(merr, wrapMatchPathErr(
							resource.ErrInvalidField{
								Name:    "value",
								Wrapped: resource.ErrEmpty,
							},
						))
					}
				default:
					merr = multierror.Append(merr, wrapMatchPathErr(
						resource.ErrInvalidField{
							Name:    "type",
							Wrapped: fmt.Errorf("not a supported enum value: %v", match.Path.Type),
						},
					))
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

				if err := validateHeaderMatchType(hdr.Type); err != nil {
					merr = multierror.Append(merr, wrapMatchHeaderErr(
						resource.ErrInvalidField{
							Name:    "type",
							Wrapped: err,
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

				// enumcover:pbmesh.QueryParamMatchType
				switch qm.Type {
				case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_UNSPECIFIED:
					merr = multierror.Append(merr, wrapMatchParamErr(
						resource.ErrInvalidField{
							Name:    "type",
							Wrapped: resource.ErrMissing,
						}),
					)
				case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT:
				case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_REGEX:
				case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_PRESENT:
				default:
					merr = multierror.Append(merr, wrapMatchParamErr(
						resource.ErrInvalidField{
							Name:    "type",
							Wrapped: fmt.Errorf("not a supported enum value: %v", qm.Type),
						},
					))
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

		var (
			hasReqMod     bool
			hasUrlRewrite bool
		)
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
				hasReqMod = true
			}
			if filter.ResponseHeaderModifier != nil {
				set++
			}
			if filter.UrlRewrite != nil {
				set++
				hasUrlRewrite = true
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

		if hasReqMod && hasUrlRewrite {
			merr = multierror.Append(merr, wrapRuleErr(
				errors.New("exactly one of request_header_modifier or url_rewrite can be set at a time"),
			))
		}

		if len(rule.BackendRefs) == 0 {
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

			wrapBackendRefFieldErr := func(err error) error {
				return wrapBackendRefErr(resource.ErrInvalidField{
					Name:    "backend_ref",
					Wrapped: err,
				})
			}
			if err := validateBackendRef(hbref.BackendRef, wrapBackendRefFieldErr); err != nil {
				merr = multierror.Append(merr, err)
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
