package structs

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/lib"
)

const (
	// Names of Envoy's LB policies
	LBPolicyMaglev       = "maglev"
	LBPolicyRingHash     = "ring_hash"
	LBPolicyRandom       = "random"
	LBPolicyLeastRequest = "least_request"
	LBPolicyRoundRobin   = "round_robin"

	// Names of Envoy's LB policies
	HashPolicyCookie     = "cookie"
	HashPolicyHeader     = "header"
	HashPolicyQueryParam = "query_parameter"
)

var (
	validLBPolicies = map[string]bool{
		"":                   true,
		LBPolicyRandom:       true,
		LBPolicyRoundRobin:   true,
		LBPolicyLeastRequest: true,
		LBPolicyRingHash:     true,
		LBPolicyMaglev:       true,
	}

	validHashPolicies = map[string]bool{
		HashPolicyHeader:     true,
		HashPolicyCookie:     true,
		HashPolicyQueryParam: true,
	}
)

// ServiceRouterConfigEntry defines L7 (e.g. http) routing rules for a named
// service exposed in Connect.
//
// This config entry represents the topmost part of the discovery chain. Only
// one router config will be used per resolved discovery chain and is not
// otherwise discovered recursively (unlike splitter and resolver config
// entries).
//
// Router config entries will be restricted to only services that define their
// protocol as http-based (in centralized configuration).
type ServiceRouterConfigEntry struct {
	Kind string
	Name string

	// Routes is the list of routes to consider when processing L7 requests.
	// The first rule to match in the list is terminal and stops further
	// evaluation.
	//
	// Traffic that fails to match any of the provided routes will be routed to
	// the default service.
	Routes []ServiceRoute

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *ServiceRouterConfigEntry) GetKind() string {
	return ServiceRouter
}

func (e *ServiceRouterConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *ServiceRouterConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ServiceRouterConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceRouter

	e.EnterpriseMeta.Normalize()

	for _, route := range e.Routes {
		if route.Match == nil || route.Match.HTTP == nil {
			continue
		}

		httpMatch := route.Match.HTTP
		for j := 0; j < len(httpMatch.Methods); j++ {
			httpMatch.Methods[j] = strings.ToUpper(httpMatch.Methods[j])
		}

		if route.Destination != nil && route.Destination.Namespace == "" {
			route.Destination.Namespace = e.EnterpriseMeta.NamespaceOrEmpty()
		}
		if route.Destination != nil && route.Destination.Partition == "" {
			route.Destination.Partition = e.EnterpriseMeta.PartitionOrEmpty()
		}
	}

	return nil
}

func (e *ServiceRouterConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	// Technically you can have no explicit routes at all where just the
	// catch-all is configured for you, but at that point maybe you should just
	// delete it so it will default?

	for i, route := range e.Routes {
		eligibleForPrefixRewrite := false
		if route.Match != nil && route.Match.HTTP != nil {
			pathParts := 0
			if route.Match.HTTP.PathExact != "" {
				eligibleForPrefixRewrite = true
				pathParts++
				if !strings.HasPrefix(route.Match.HTTP.PathExact, "/") {
					return fmt.Errorf("Route[%d] PathExact doesn't start with '/': %q", i, route.Match.HTTP.PathExact)
				}
			}
			if route.Match.HTTP.PathPrefix != "" {
				eligibleForPrefixRewrite = true
				pathParts++
				if !strings.HasPrefix(route.Match.HTTP.PathPrefix, "/") {
					return fmt.Errorf("Route[%d] PathPrefix doesn't start with '/': %q", i, route.Match.HTTP.PathPrefix)
				}
			}
			if route.Match.HTTP.PathRegex != "" {
				pathParts++
			}
			if pathParts > 1 {
				return fmt.Errorf("Route[%d] should only contain at most one of PathExact, PathPrefix, or PathRegex", i)
			}

			for j, hdr := range route.Match.HTTP.Header {
				if hdr.Name == "" {
					return fmt.Errorf("Route[%d] Header[%d] missing required Name field", i, j)
				}
				hdrParts := 0
				if hdr.Present {
					hdrParts++
				}
				if hdr.Exact != "" {
					hdrParts++
				}
				if hdr.Regex != "" {
					hdrParts++
				}
				if hdr.Prefix != "" {
					hdrParts++
				}
				if hdr.Suffix != "" {
					hdrParts++
				}
				if hdrParts != 1 {
					return fmt.Errorf("Route[%d] Header[%d] should only contain one of Present, Exact, Prefix, Suffix, or Regex", i, j)
				}
			}

			for j, qm := range route.Match.HTTP.QueryParam {
				if qm.Name == "" {
					return fmt.Errorf("Route[%d] QueryParam[%d] missing required Name field", i, j)
				}

				qmParts := 0
				if qm.Present {
					qmParts++
				}
				if qm.Exact != "" {
					qmParts++
				}
				if qm.Regex != "" {
					qmParts++
				}
				if qmParts != 1 {
					return fmt.Errorf("Route[%d] QueryParam[%d] should only contain one of Present, Exact, or Regex", i, j)
				}
			}

			if len(route.Match.HTTP.Methods) > 0 {
				found := make(map[string]struct{})
				for _, m := range route.Match.HTTP.Methods {
					if !isValidHTTPMethod(m) {
						return fmt.Errorf("Route[%d] Methods contains an invalid method %q", i, m)
					}
					if _, ok := found[m]; ok {
						return fmt.Errorf("Route[%d] Methods contains %q more than once", i, m)
					}
					found[m] = struct{}{}
				}
			}
		}

		if route.Destination != nil {
			if route.Destination.PrefixRewrite != "" && !eligibleForPrefixRewrite {
				return fmt.Errorf("Route[%d] cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix", i)
			}

			for _, r := range route.Destination.RetryOn {
				if !isValidRetryCondition(r) {
					return fmt.Errorf("Route[%d] contains an invalid retry condition: %q", i, r)
				}
			}
		}
	}

	return nil
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

func (e *ServiceRouterConfigEntry) CanRead(authz acl.Authorizer) error {
	return canReadDiscoveryChain(e, authz)
}

func (e *ServiceRouterConfigEntry) CanWrite(authz acl.Authorizer) error {
	return canWriteDiscoveryChain(e, authz)
}

func (e *ServiceRouterConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceRouterConfigEntry) ListRelatedServices() []ServiceID {
	found := make(map[ServiceID]struct{})

	// We always inject a default catch-all route to the same service as the router.
	svcID := NewServiceID(e.Name, &e.EnterpriseMeta)
	found[svcID] = struct{}{}

	for _, route := range e.Routes {
		if route.Destination != nil {
			destID := NewServiceID(defaultIfEmpty(route.Destination.Service, e.Name), route.Destination.GetEnterpriseMeta(&e.EnterpriseMeta))
			if destID != svcID {
				found[destID] = struct{}{}
			}
		}
	}

	if len(found) == 0 {
		return nil
	}

	out := make([]ServiceID, 0, len(found))
	for svc := range found {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EnterpriseMeta.LessThan(&out[j].EnterpriseMeta) ||
			out[i].ID < out[j].ID
	})
	return out
}

func (e *ServiceRouterConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

// ServiceRoute is a single routing rule that routes traffic to the destination
// when the match criteria applies.
type ServiceRoute struct {
	Match       *ServiceRouteMatch       `json:",omitempty"`
	Destination *ServiceRouteDestination `json:",omitempty"`
}

// ServiceRouteMatch is a set of criteria that can match incoming L7 requests.
type ServiceRouteMatch struct {
	HTTP *ServiceRouteHTTPMatch `json:",omitempty"`

	// If we have non-http match criteria for other protocols in the future
	// (gRPC, redis, etc) they can go here.
}

func (m *ServiceRouteMatch) IsEmpty() bool {
	return m.HTTP == nil || m.HTTP.IsEmpty()
}

// ServiceRouteHTTPMatch is a set of http-specific match criteria.
type ServiceRouteHTTPMatch struct {
	PathExact  string `json:",omitempty" alias:"path_exact"`
	PathPrefix string `json:",omitempty" alias:"path_prefix"`
	PathRegex  string `json:",omitempty" alias:"path_regex"`

	Header     []ServiceRouteHTTPMatchHeader     `json:",omitempty"`
	QueryParam []ServiceRouteHTTPMatchQueryParam `json:",omitempty" alias:"query_param"`
	Methods    []string                          `json:",omitempty"`
}

func (m *ServiceRouteHTTPMatch) IsEmpty() bool {
	return m.PathExact == "" &&
		m.PathPrefix == "" &&
		m.PathRegex == "" &&
		len(m.Header) == 0 &&
		len(m.QueryParam) == 0 &&
		len(m.Methods) == 0
}

type ServiceRouteHTTPMatchHeader struct {
	Name    string
	Present bool   `json:",omitempty"`
	Exact   string `json:",omitempty"`
	Prefix  string `json:",omitempty"`
	Suffix  string `json:",omitempty"`
	Regex   string `json:",omitempty"`
	Invert  bool   `json:",omitempty"`
}

type ServiceRouteHTTPMatchQueryParam struct {
	Name    string
	Present bool   `json:",omitempty"`
	Exact   string `json:",omitempty"`
	Regex   string `json:",omitempty"`
}

// ServiceRouteDestination describes how to proxy the actual matching request
// to a service.
type ServiceRouteDestination struct {
	// Service is the service to resolve instead of the default service. If
	// empty then the default discovery chain service name is used.
	Service string `json:",omitempty"`

	// ServiceSubset is a named subset of the given service to resolve instead
	// of one defined as that service's DefaultSubset. If empty the default
	// subset is used.
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	ServiceSubset string `json:",omitempty" alias:"service_subset"`

	// Namespace is the namespace to resolve the service from instead of the
	// current namespace. If empty the current namespace is assumed.
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	Namespace string `json:",omitempty"`

	// Partition is the partition to resolve the service from instead of the
	// current partition. If empty the current partition is assumed.
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	Partition string `json:",omitempty"`

	// PrefixRewrite allows for the proxied request to have its matching path
	// prefix modified before being sent to the destination. Described more
	// below in the envoy implementation section.
	PrefixRewrite string `json:",omitempty" alias:"prefix_rewrite"`

	// RequestTimeout is the total amount of time permitted for the entire
	// downstream request (and retries) to be processed.
	RequestTimeout time.Duration `json:",omitempty" alias:"request_timeout"`

	// IdleTimeout is The total amount of time permitted for the request stream
	// to be idle
	IdleTimeout time.Duration `json:",omitempty" alias:"idle_timeout"`

	// NumRetries is the number of times to retry the request when a retryable
	// result occurs. This seems fairly proxy agnostic.
	NumRetries uint32 `json:",omitempty" alias:"num_retries"`

	// RetryOnConnectFailure allows for connection failure errors to trigger a
	// retry. This should be expressible in other proxies as it's just a layer
	// 4 failure bubbling up to layer 7.
	RetryOnConnectFailure bool `json:",omitempty" alias:"retry_on_connect_failure"`

	// RetryOn allows setting envoy specific conditions when a request should
	// be automatically retried.
	RetryOn []string `json:",omitempty" alias:"retry_on"`

	// RetryOnStatusCodes is a flat list of http response status codes that are
	// eligible for retry. This again should be feasible in any reasonable proxy.
	RetryOnStatusCodes []uint32 `json:",omitempty" alias:"retry_on_status_codes"`

	// Allow HTTP header manipulation to be configured.
	RequestHeaders  *HTTPHeaderModifiers `json:",omitempty" alias:"request_headers"`
	ResponseHeaders *HTTPHeaderModifiers `json:",omitempty" alias:"response_headers"`
}

func (e *ServiceRouteDestination) MarshalJSON() ([]byte, error) {
	type Alias ServiceRouteDestination
	exported := &struct {
		RequestTimeout string `json:",omitempty"`
		IdleTimeout    string `json:",omitempty"`
		*Alias
	}{
		RequestTimeout: e.RequestTimeout.String(),
		IdleTimeout:    e.IdleTimeout.String(),
		Alias:          (*Alias)(e),
	}
	if e.RequestTimeout == 0 {
		exported.RequestTimeout = ""
	}

	if e.IdleTimeout == 0 {
		exported.IdleTimeout = ""
	}

	return json.Marshal(exported)
}

func (e *ServiceRouteDestination) UnmarshalJSON(data []byte) error {
	type Alias ServiceRouteDestination
	aux := &struct {
		RequestTimeout string
		IdleTimeout    string
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.RequestTimeout != "" {
		if e.RequestTimeout, err = time.ParseDuration(aux.RequestTimeout); err != nil {
			return err
		}
	}
	if aux.IdleTimeout != "" {
		if e.IdleTimeout, err = time.ParseDuration(aux.IdleTimeout); err != nil {
			return err
		}
	}
	return nil
}

func (d *ServiceRouteDestination) HasRetryFeatures() bool {
	return d.NumRetries > 0 || d.RetryOnConnectFailure || len(d.RetryOnStatusCodes) > 0 || len(d.RetryOn) > 0
}

// ServiceSplitterConfigEntry defines how incoming requests are split across
// different subsets of a single service (like during staged canary rollouts),
// or perhaps across different services (like during a v2 rewrite or other type
// of codebase migration).
//
// This config entry represents the next hop of the discovery chain after
// routing. If no splitter config is defined the chain assumes 100% of traffic
// goes to the default service and discovery continues on to the resolution
// hop.
//
// Splitter configs are recursively collected while walking the discovery
// chain.
//
// Splitter config entries will be restricted to only services that define
// their protocol as http-based (in centralized configuration).
type ServiceSplitterConfigEntry struct {
	Kind string
	Name string

	// Splits is the configurations for the details of the traffic splitting.
	//
	// The sum of weights across all splits must add up to 100.
	//
	// If the split is within epsilon of 100 then the remainder is attributed
	// to the FIRST split.
	Splits []ServiceSplit

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *ServiceSplitterConfigEntry) GetKind() string {
	return ServiceSplitter
}

func (e *ServiceSplitterConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *ServiceSplitterConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ServiceSplitterConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceSplitter

	// This slightly massages inputs to enforce that the smallest representable
	// weight is 1/10000 or .01%

	e.EnterpriseMeta.Normalize()

	if len(e.Splits) > 0 {
		for i, split := range e.Splits {
			if split.Namespace == "" {
				split.Namespace = e.EnterpriseMeta.NamespaceOrDefault()
			}
			e.Splits[i].Weight = NormalizeServiceSplitWeight(split.Weight)
		}
	}

	return nil
}

func NormalizeServiceSplitWeight(weight float32) float32 {
	weightScaled := scaleWeight(weight)
	return float32(weightScaled) / 100.0
}

func (e *ServiceSplitterConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if len(e.Splits) == 0 {
		return fmt.Errorf("no splits configured")
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	const maxScaledWeight = 100 * 100

	copyAsKey := func(s ServiceSplit) ServiceSplit {
		s.Weight = 0
		return s
	}

	// Make sure we didn't refer to the same thing twice.
	found := make(map[ServiceSplit]struct{})
	for _, split := range e.Splits {
		splitKey := copyAsKey(split)
		if splitKey.Service == "" {
			splitKey.Service = e.Name
		}
		if _, ok := found[splitKey]; ok {
			return fmt.Errorf(
				"split destination occurs more than once: service=%q, subset=%q, namespace=%q, partition=%q",
				splitKey.Service, splitKey.ServiceSubset, splitKey.Namespace, splitKey.Partition,
			)
		}
		found[splitKey] = struct{}{}
	}

	sumScaled := 0
	for _, split := range e.Splits {
		sumScaled += scaleWeight(split.Weight)
	}

	if sumScaled != maxScaledWeight {
		return fmt.Errorf("the sum of all split weights must be 100, not %f", float32(sumScaled)/100)
	}

	return nil
}

// scaleWeight assumes the input is a value between 0 and 100 representing
// shares out of a percentile range. The function will convert to a unit
// representing 0.01% units in the same manner as you may convert $0.98 to 98
// cents.
func scaleWeight(v float32) int {
	return int(math.Round(float64(v * 100.0)))
}

func (e *ServiceSplitterConfigEntry) CanRead(authz acl.Authorizer) error {
	return canReadDiscoveryChain(e, authz)
}

func (e *ServiceSplitterConfigEntry) CanWrite(authz acl.Authorizer) error {
	return canWriteDiscoveryChain(e, authz)
}

func (e *ServiceSplitterConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceSplitterConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *ServiceSplitterConfigEntry) ListRelatedServices() []ServiceID {
	found := make(map[ServiceID]struct{})

	svcID := NewServiceID(e.Name, &e.EnterpriseMeta)
	for _, split := range e.Splits {
		splitID := NewServiceID(defaultIfEmpty(split.Service, e.Name), split.GetEnterpriseMeta(&e.EnterpriseMeta))

		if splitID != svcID {
			found[splitID] = struct{}{}
		}
	}

	if len(found) == 0 {
		return nil
	}

	out := make([]ServiceID, 0, len(found))
	for svc := range found {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EnterpriseMeta.LessThan(&out[j].EnterpriseMeta) ||
			out[i].ID < out[j].ID
	})
	return out
}

// ServiceSplit defines how much traffic to send to which set of service
// instances during a traffic split.
type ServiceSplit struct {
	// A value between 0 and 100 reflecting what portion of traffic should be
	// directed to this split.
	//
	// The smallest representable weight is 1/10000 or .01%
	//
	// If the split is within epsilon of 100 then the remainder is attributed
	// to the FIRST split.
	Weight float32

	// Service is the service to resolve instead of the default (optional).
	Service string `json:",omitempty"`

	// ServiceSubset is a named subset of the given service to resolve instead
	// of one defined as that service's DefaultSubset. If empty the default
	// subset is used (optional).
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	ServiceSubset string `json:",omitempty" alias:"service_subset"`

	// Namespace is the namespace to resolve the service from instead of the
	// current namespace. If empty the current namespace is assumed (optional).
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	Namespace string `json:",omitempty"`

	// Partition is the partition to resolve the service from instead of the
	// current partition. If empty the current partition is assumed (optional).
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	Partition string `json:",omitempty"`

	// NOTE: Any configuration added to Splits that needs to be passed to the
	// proxy needs special handling MergeParent below.

	// Allow HTTP header manipulation to be configured.
	RequestHeaders  *HTTPHeaderModifiers `json:",omitempty" alias:"request_headers"`
	ResponseHeaders *HTTPHeaderModifiers `json:",omitempty" alias:"response_headers"`
}

// MergeParent is called by the discovery chain compiler when a split directs to
// another splitter. We refer to the first ServiceSplit as the parent and the
// ServiceSplits of the second splitter as its children. The parent ends up
// "flattened" by the compiler, i.e. replaced with its children recursively with
// the weights modified as necessary.
//
// Since the parent is never included in the output, any request processing
// config attached to it (e.g. header manipulation) would be lost and not take
// affect when splitters direct to other splitters. To avoid that, we define a
// MergeParent operation which is called by the compiler on each child split
// during flattening. It must merge any request processing configuration from
// the passed parent into the child such that the end result is equivalent to a
// request first passing through the parent and then the child. Response
// handling must occur as if the request first passed through the through the
// child to the parent.
//
// MergeDefaults leaves both s and parent unchanged and returns a deep copy to
// avoid confusing issues where config changes after being compiled.
func (s *ServiceSplit) MergeParent(parent *ServiceSplit) (*ServiceSplit, error) {
	if s == nil && parent == nil {
		return nil, nil
	}

	var err error
	var copy ServiceSplit

	if s == nil {
		copy = *parent
		copy.RequestHeaders, err = parent.RequestHeaders.Clone()
		if err != nil {
			return nil, err
		}
		copy.ResponseHeaders, err = parent.ResponseHeaders.Clone()
		if err != nil {
			return nil, err
		}
		return &copy, nil
	} else {
		copy = *s
	}

	var parentReq *HTTPHeaderModifiers
	if parent != nil {
		parentReq = parent.RequestHeaders
	}

	// Merge any request handling from parent _unless_ it's overridden by us.
	copy.RequestHeaders, err = MergeHTTPHeaderModifiers(parentReq, s.RequestHeaders)
	if err != nil {
		return nil, err
	}

	var parentResp *HTTPHeaderModifiers
	if parent != nil {
		parentResp = parent.ResponseHeaders
	}

	// Merge any response handling. Note that we allow parent to override this
	// time since responses flow the other way so the unflattened behavior would
	// be that the parent processing happens _after_ ours potentially overriding
	// it.
	copy.ResponseHeaders, err = MergeHTTPHeaderModifiers(s.ResponseHeaders, parentResp)
	if err != nil {
		return nil, err
	}
	return &copy, nil
}

// ServiceResolverConfigEntry defines which instances of a service should
// satisfy discovery requests for a given named service.
//
// This config entry represents the next hop of the discovery chain after
// splitting. If no resolver config is defined the chain assumes 100% of
// traffic goes to the healthy instances of the default service in the current
// datacenter+namespace and discovery terminates.
//
// Resolver configs are recursively collected while walking the chain.
//
// Resolver config entries will be valid for services defined with any protocol
// (in centralized configuration).
type ServiceResolverConfigEntry struct {
	Kind string
	Name string

	// DefaultSubset is the subset to use when no explicit subset is
	// requested. If empty the unnamed subset is used.
	DefaultSubset string `json:",omitempty" alias:"default_subset"`

	// Subsets is a map of subset name to subset definition for all
	// usable named subsets of this service. The map key is the name
	// of the subset and all names must be valid DNS subdomain elements
	// so they can be used in SNI FQDN headers for the Connect Gateways
	// feature.
	//
	// This may be empty, in which case only the unnamed default subset
	// will be usable.
	Subsets map[string]ServiceResolverSubset `json:",omitempty"`

	// Redirect is a service/subset/datacenter/namespace to resolve
	// instead of the requested service (optional).
	//
	// When configured, all occurrences of this resolver in any discovery
	// chain evaluation will be substituted for the supplied redirect
	// EXCEPT when the redirect has already been applied.
	//
	// When substituting the supplied redirect into the discovery chain
	// all other fields beside Kind/Name/Redirect will be ignored.
	Redirect *ServiceResolverRedirect `json:",omitempty"`

	// Failover controls when and how to reroute traffic to an alternate pool
	// of service instances.
	//
	// The map is keyed by the service subset it applies to, and the special
	// string "*" is a wildcard that applies to any subset not otherwise
	// specified here.
	Failover map[string]ServiceResolverFailover `json:",omitempty"`

	// ConnectTimeout is the timeout for establishing new network connections
	// to this service.
	ConnectTimeout time.Duration `json:",omitempty" alias:"connect_timeout"`

	// RequestTimeout is the timeout for an HTTP request to complete before
	// the connection is automatically terminated. If unspecified, defaults
	// to 15 seconds.
	RequestTimeout time.Duration `json:",omitempty" alias:"request_timeout"`

	// LoadBalancer determines the load balancing policy and configuration for services
	// issuing requests to this upstream service.
	LoadBalancer *LoadBalancer `json:",omitempty" alias:"load_balancer"`

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

func (e *ServiceResolverConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias ServiceResolverConfigEntry
	exported := &struct {
		ConnectTimeout string `json:",omitempty"`
		RequestTimeout string `json:",omitempty"`
		*Alias
	}{
		ConnectTimeout: e.ConnectTimeout.String(),
		RequestTimeout: e.RequestTimeout.String(),
		Alias:          (*Alias)(e),
	}
	if e.ConnectTimeout == 0 {
		exported.ConnectTimeout = ""
	}
	if e.RequestTimeout == 0 {
		exported.RequestTimeout = ""
	}

	return json.Marshal(exported)
}

func (e *ServiceResolverConfigEntry) UnmarshalJSON(data []byte) error {
	type Alias ServiceResolverConfigEntry
	aux := &struct {
		ConnectTimeout string
		RequestTimeout string
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.ConnectTimeout != "" {
		if e.ConnectTimeout, err = time.ParseDuration(aux.ConnectTimeout); err != nil {
			return err
		}
	}
	if aux.RequestTimeout != "" {
		if e.RequestTimeout, err = time.ParseDuration(aux.RequestTimeout); err != nil {
			return err
		}
	}
	return nil
}

func (e *ServiceResolverConfigEntry) SubsetExists(name string) bool {
	if name == "" {
		return true
	}
	if len(e.Subsets) == 0 {
		return false
	}
	_, ok := e.Subsets[name]
	return ok
}

func (e *ServiceResolverConfigEntry) IsDefault() bool {
	return e.DefaultSubset == "" &&
		len(e.Subsets) == 0 &&
		e.Redirect == nil &&
		len(e.Failover) == 0 &&
		e.ConnectTimeout == 0 &&
		e.RequestTimeout == 0 &&
		e.LoadBalancer == nil
}

func (e *ServiceResolverConfigEntry) GetKind() string {
	return ServiceResolver
}

func (e *ServiceResolverConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *ServiceResolverConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ServiceResolverConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceResolver

	e.EnterpriseMeta.Normalize()

	return nil
}

func (e *ServiceResolverConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	if len(e.Subsets) > 0 {
		for name, subset := range e.Subsets {
			if name == "" {
				return fmt.Errorf("Subset defined with empty name")
			}
			if err := validateServiceSubset(name); err != nil {
				return fmt.Errorf("Subset %q is invalid: %v", name, err)
			}
			if subset.Filter != "" {
				if _, err := bexpr.CreateEvaluator(subset.Filter, nil); err != nil {
					return fmt.Errorf("Filter for subset %q is not a valid expression: %v", name, err)
				}
			}
		}
	}

	isSubset := func(subset string) bool {
		if len(e.Subsets) > 0 {
			_, ok := e.Subsets[subset]
			return ok
		}
		return false
	}

	if e.DefaultSubset != "" && !isSubset(e.DefaultSubset) {
		return fmt.Errorf("DefaultSubset %q is not a valid subset", e.DefaultSubset)
	}

	if e.Redirect != nil {
		if !e.InDefaultPartition() && e.Redirect.Datacenter != "" {
			return fmt.Errorf("Cross-datacenter redirect is only supported in the default partition")
		}
		if acl.PartitionOrDefault(e.Redirect.Partition) != e.PartitionOrDefault() && e.Redirect.Datacenter != "" {
			return fmt.Errorf("Cross-datacenter and cross-partition redirect is not supported")
		}

		r := e.Redirect

		if err := r.ValidateEnterprise(); err != nil {
			return fmt.Errorf("Redirect: %s", err.Error())
		}

		if len(e.Failover) > 0 {
			return fmt.Errorf("Redirect and Failover cannot both be set")
		}

		// TODO(rb): prevent subsets and default subsets from being defined?

		if r.isEmpty() {
			return fmt.Errorf("Redirect is empty")
		}

		switch {
		case r.Peer != "" && r.ServiceSubset != "":
			return fmt.Errorf("Redirect.Peer cannot be set with Redirect.ServiceSubset")
		case r.Peer != "" && r.Partition != "":
			return fmt.Errorf("Redirect.Partition cannot be set with Redirect.Peer")
		case r.Peer != "" && r.Datacenter != "":
			return fmt.Errorf("Redirect.Peer cannot be set with Redirect.Datacenter")
		case r.Service == "":
			if r.ServiceSubset != "" {
				return fmt.Errorf("Redirect.ServiceSubset defined without Redirect.Service")
			}
			if r.Namespace != "" {
				return fmt.Errorf("Redirect.Namespace defined without Redirect.Service")
			}
			if r.Partition != "" {
				return fmt.Errorf("Redirect.Partition defined without Redirect.Service")
			}
			if r.Peer != "" {
				return fmt.Errorf("Redirect.Peer defined without Redirect.Service")
			}
		case r.ServiceSubset != "" && (r.Service == "" || r.Service == e.Name):
			if !isSubset(r.ServiceSubset) {
				return fmt.Errorf("Redirect.ServiceSubset %q is not a valid subset of %q", r.ServiceSubset, e.Name)
			}
		}
	}

	if len(e.Failover) > 0 {

		for subset, f := range e.Failover {
			if !e.InDefaultPartition() && len(f.Datacenters) != 0 {
				return fmt.Errorf("Cross-datacenter failover is only supported in the default partition")
			}

			errorPrefix := fmt.Sprintf("Bad Failover[%q]: ", subset)

			if err := f.ValidateEnterprise(); err != nil {
				return fmt.Errorf(errorPrefix + err.Error())
			}

			if subset != "*" && !isSubset(subset) {
				return fmt.Errorf(errorPrefix + "not a valid subset subset")
			}

			if f.isEmpty() {
				return fmt.Errorf(errorPrefix + "one of Service, ServiceSubset, Namespace, Targets, or Datacenters is required")
			}

			if f.ServiceSubset != "" {
				if f.Service == "" || f.Service == e.Name {
					if !isSubset(f.ServiceSubset) {
						return fmt.Errorf("%sServiceSubset %q is not a valid subset of %q", errorPrefix, f.ServiceSubset, f.Service)
					}
				}
			}

			if len(f.Datacenters) != 0 && len(f.Targets) != 0 {
				return fmt.Errorf("Bad Failover[%q]: Targets cannot be set with Datacenters", subset)
			}

			if f.ServiceSubset != "" && len(f.Targets) != 0 {
				return fmt.Errorf("Bad Failover[%q]: Targets cannot be set with ServiceSubset", subset)
			}

			if f.Service != "" && len(f.Targets) != 0 {
				return fmt.Errorf("Bad Failover[%q]: Targets cannot be set with Service", subset)
			}

			for i, target := range f.Targets {
				errorPrefix := fmt.Sprintf("Bad Failover[%q].Targets[%d]: ", subset, i)

				if err := target.ValidateEnterprise(); err != nil {
					return fmt.Errorf(errorPrefix + err.Error())
				}

				switch {
				case target.Peer != "" && target.ServiceSubset != "":
					return fmt.Errorf(errorPrefix + "Peer cannot be set with ServiceSubset")
				case target.Peer != "" && target.Partition != "":
					return fmt.Errorf(errorPrefix + "Partition cannot be set with Peer")
				case target.Peer != "" && target.Datacenter != "":
					return fmt.Errorf(errorPrefix + "Peer cannot be set with Datacenter")
				case target.Partition != "" && target.Datacenter != "":
					return fmt.Errorf(errorPrefix + "Partition cannot be set with Datacenter")
				case target.ServiceSubset != "" && (target.Service == "" || target.Service == e.Name):
					if !isSubset(target.ServiceSubset) {
						return fmt.Errorf("%sServiceSubset %q is not a valid subset of %q", errorPrefix, target.ServiceSubset, e.Name)
					}
				}
			}

			for _, dc := range f.Datacenters {
				if dc == "" {
					return fmt.Errorf("Bad Failover[%q].Datacenters: found empty datacenter", subset)
				}
			}
		}
	}

	if e.ConnectTimeout < 0 {
		return fmt.Errorf("Bad ConnectTimeout '%s', must be >= 0", e.ConnectTimeout)
	}

	if e.RequestTimeout < 0 {
		return fmt.Errorf("Bad RequestTimeout '%s', must be >= 0", e.RequestTimeout)
	}

	if e.LoadBalancer != nil {
		lb := e.LoadBalancer

		if ok := validLBPolicies[lb.Policy]; !ok {
			return fmt.Errorf("Bad LoadBalancer policy: %q is not supported", lb.Policy)
		}

		if lb.Policy != LBPolicyRingHash && lb.RingHashConfig != nil {
			return fmt.Errorf("Bad LoadBalancer configuration. "+
				"RingHashConfig specified for incompatible load balancing policy %q", lb.Policy)
		}
		if lb.Policy != LBPolicyLeastRequest && lb.LeastRequestConfig != nil {
			return fmt.Errorf("Bad LoadBalancer configuration. "+
				"LeastRequestConfig specified for incompatible load balancing policy %q", lb.Policy)
		}
		if !lb.IsHashBased() && len(lb.HashPolicies) > 0 {
			return fmt.Errorf("Bad LoadBalancer configuration: "+
				"HashPolicies specified for non-hash-based Policy: %q", lb.Policy)
		}

		for i, hp := range lb.HashPolicies {
			if ok := validHashPolicies[hp.Field]; hp.Field != "" && !ok {
				return fmt.Errorf("Bad LoadBalancer HashPolicy[%d]: %q is not a supported field", i, hp.Field)
			}

			if hp.SourceIP && hp.Field != "" {
				return fmt.Errorf("Bad LoadBalancer HashPolicy[%d]: "+
					"A single hash policy cannot hash both a source address and a %q", i, hp.Field)
			}
			if hp.SourceIP && hp.FieldValue != "" {
				return fmt.Errorf("Bad LoadBalancer HashPolicy[%d]: "+
					"A FieldValue cannot be specified when hashing SourceIP", i)
			}
			if hp.Field != "" && hp.FieldValue == "" {
				return fmt.Errorf("Bad LoadBalancer HashPolicy[%d]: Field %q was specified without a FieldValue", i, hp.Field)
			}
			if hp.FieldValue != "" && hp.Field == "" {
				return fmt.Errorf("Bad LoadBalancer HashPolicy[%d]: FieldValue requires a Field to apply to", i)
			}
			if hp.CookieConfig != nil {
				if hp.Field != HashPolicyCookie {
					return fmt.Errorf("Bad LoadBalancer HashPolicy[%d]: cookie_config provided for %q", i, hp.Field)
				}
				if hp.CookieConfig.Session && hp.CookieConfig.TTL != 0*time.Second {
					return fmt.Errorf("Bad LoadBalancer HashPolicy[%d]: a session cookie cannot have an associated TTL", i)
				}
			}
		}
	}

	return nil
}

func (e *ServiceResolverConfigEntry) CanRead(authz acl.Authorizer) error {
	return canReadDiscoveryChain(e, authz)
}

func (e *ServiceResolverConfigEntry) CanWrite(authz acl.Authorizer) error {
	return canWriteDiscoveryChain(e, authz)
}

func (e *ServiceResolverConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceResolverConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *ServiceResolverConfigEntry) ListRelatedServices() []ServiceID {
	found := make(map[ServiceID]struct{})

	svcID := NewServiceID(e.Name, &e.EnterpriseMeta)
	if e.Redirect != nil {
		redirectID := NewServiceID(defaultIfEmpty(e.Redirect.Service, e.Name), e.Redirect.GetEnterpriseMeta(&e.EnterpriseMeta))
		if redirectID != svcID {
			found[redirectID] = struct{}{}
		}

	}

	if len(e.Failover) > 0 {
		for _, failover := range e.Failover {
			if len(failover.Targets) == 0 {
				failoverID := NewServiceID(defaultIfEmpty(failover.Service, e.Name), failover.GetEnterpriseMeta(&e.EnterpriseMeta))
				if failoverID != svcID {
					found[failoverID] = struct{}{}
				}
				continue
			}

			for _, target := range failover.Targets {
				// We can't know about related services on cluster peers.
				if target.Peer != "" {
					continue
				}

				failoverID := NewServiceID(defaultIfEmpty(target.Service, e.Name), target.GetEnterpriseMeta(failover.GetEnterpriseMeta(&e.EnterpriseMeta)))
				if failoverID != svcID {
					found[failoverID] = struct{}{}
				}
			}
		}
	}

	if len(found) == 0 {
		return nil
	}

	out := make([]ServiceID, 0, len(found))
	for svc := range found {
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EnterpriseMeta.LessThan(&out[j].EnterpriseMeta) ||
			out[i].ID < out[j].ID
	})
	return out
}

// ServiceResolverSubset defines a way to select a portion of the Consul
// catalog during service discovery. Anything that affects the ultimate catalog
// query performed OR post-processing on the results of that sort of query
// should be defined here.
type ServiceResolverSubset struct {
	// Filter specifies the go-bexpr filter expression to be used for selecting
	// instances of the requested service.
	Filter string `json:",omitempty"`

	// OnlyPassing - Specifies the behavior of the resolver's health check
	// filtering. If this is set to false, the results will include instances
	// with checks in the passing as well as the warning states. If this is set
	// to true, only instances with checks in the passing state will be
	// returned. (behaves identically to the similarly named field on prepared
	// queries).
	OnlyPassing bool `json:",omitempty" alias:"only_passing"`
}

type ServiceResolverRedirect struct {
	// Service is a service to resolve instead of the current service
	// (optional).
	Service string `json:",omitempty"`

	// ServiceSubset is a named subset of the given service to resolve instead
	// of one defined as that service's DefaultSubset If empty the default
	// subset is used (optional).
	//
	// If this is specified at least one of Service, Datacenter, or Namespace
	// should be configured.
	ServiceSubset string `json:",omitempty" alias:"service_subset"`

	// Namespace is the namespace to resolve the service from instead of the
	// current one (optional).
	Namespace string `json:",omitempty"`

	// Partition is the partition to resolve the service from instead of the
	// current one (optional).
	Partition string `json:",omitempty"`

	// Datacenter is the datacenter to resolve the service from instead of the
	// current one (optional).
	Datacenter string `json:",omitempty"`

	// Peer is the name of the cluster peer to resolve the service from instead
	// of the current one (optional).
	Peer string `json:",omitempty"`
}

func (r *ServiceResolverRedirect) ToDiscoveryTargetOpts() DiscoveryTargetOpts {
	return DiscoveryTargetOpts{
		Service:       r.Service,
		ServiceSubset: r.ServiceSubset,
		Namespace:     r.Namespace,
		Partition:     r.Partition,
		Datacenter:    r.Datacenter,
		Peer:          r.Peer,
	}
}

func (r *ServiceResolverRedirect) isEmpty() bool {
	return r.Service == "" && r.ServiceSubset == "" && r.Namespace == "" && r.Partition == "" && r.Datacenter == "" && r.Peer == ""
}

// There are some restrictions on what is allowed in here:
//
// - Service, ServiceSubset, Namespace, Datacenters, and Targets cannot all be
// empty at once. When Targets is defined, the other fields should not be
// populated.
type ServiceResolverFailover struct {
	// Service is the service to resolve instead of the default as the failover
	// group of instances (optional).
	//
	// This is a DESTINATION during failover.
	Service string `json:",omitempty"`

	// ServiceSubset is the named subset of the requested service to resolve as
	// the failover group of instances. If empty the default subset for the
	// requested service is used (optional).
	//
	// This is a DESTINATION during failover.
	ServiceSubset string `json:",omitempty" alias:"service_subset"`

	// Namespace is the namespace to resolve the requested service from to form
	// the failover group of instances. If empty the current namespace is used
	// (optional).
	//
	// This is a DESTINATION during failover.
	Namespace string `json:",omitempty"`

	// Datacenters is a fixed list of datacenters to try. We never try a
	// datacenter multiple times, so those are subtracted from this list before
	// proceeding.
	//
	// This is a DESTINATION during failover.
	Datacenters []string `json:",omitempty"`

	// Targets specifies a fixed list of failover targets to try. We never try a
	// target multiple times, so those are subtracted from this list before
	// proceeding.
	//
	// This is a DESTINATION during failover.
	Targets []ServiceResolverFailoverTarget `json:",omitempty"`
}

func (t *ServiceResolverFailover) ToDiscoveryTargetOpts() DiscoveryTargetOpts {
	return DiscoveryTargetOpts{
		Service:       t.Service,
		ServiceSubset: t.ServiceSubset,
		Namespace:     t.Namespace,
	}
}

func (f *ServiceResolverFailover) isEmpty() bool {
	return f.Service == "" && f.ServiceSubset == "" && f.Namespace == "" && len(f.Datacenters) == 0 && len(f.Targets) == 0
}

type ServiceResolverFailoverTarget struct {
	// Service specifies the name of the service to try during failover.
	Service string `json:",omitempty"`

	// ServiceSubset specifies the service subset to try during failover.
	ServiceSubset string `json:",omitempty" alias:"service_subset"`

	// Partition specifies the partition to try during failover.
	Partition string `json:",omitempty"`

	// Namespace specifies the namespace to try during failover.
	Namespace string `json:",omitempty"`

	// Datacenter specifies the datacenter to try during failover.
	Datacenter string `json:",omitempty"`

	// Peer specifies the name of the cluster peer to try during failover.
	Peer string `json:",omitempty"`
}

func (t *ServiceResolverFailoverTarget) ToDiscoveryTargetOpts() DiscoveryTargetOpts {
	return DiscoveryTargetOpts{
		Service:       t.Service,
		ServiceSubset: t.ServiceSubset,
		Namespace:     t.Namespace,
		Partition:     t.Partition,
		Datacenter:    t.Datacenter,
		Peer:          t.Peer,
	}
}

// LoadBalancer determines the load balancing policy and configuration for services
// issuing requests to this upstream service.
type LoadBalancer struct {
	// Policy is the load balancing policy used to select a host
	Policy string `json:",omitempty"`

	// RingHashConfig contains configuration for the "ring_hash" policy type
	RingHashConfig *RingHashConfig `json:",omitempty" alias:"ring_hash_config"`

	// LeastRequestConfig contains configuration for the "least_request" policy type
	LeastRequestConfig *LeastRequestConfig `json:",omitempty" alias:"least_request_config"`

	// HashPolicies is a list of hash policies to use for hashing load balancing algorithms.
	// Hash policies are evaluated individually and combined such that identical lists
	// result in the same hash.
	// If no hash policies are present, or none are successfully evaluated,
	// then a random backend host will be selected.
	HashPolicies []HashPolicy `json:",omitempty" alias:"hash_policies"`
}

// RingHashConfig contains configuration for the "ring_hash" policy type
type RingHashConfig struct {
	// MinimumRingSize determines the minimum number of entries in the hash ring
	MinimumRingSize uint64 `json:",omitempty" alias:"minimum_ring_size"`

	// MaximumRingSize determines the maximum number of entries in the hash ring
	MaximumRingSize uint64 `json:",omitempty" alias:"maximum_ring_size"`
}

// LeastRequestConfig contains configuration for the "least_request" policy type
type LeastRequestConfig struct {
	// ChoiceCount determines the number of random healthy hosts from which to select the one with the least requests.
	ChoiceCount uint32 `json:",omitempty" alias:"choice_count"`
}

// HashPolicy defines which attributes will be hashed by hash-based LB algorithms
type HashPolicy struct {
	// Field is the attribute type to hash on.
	// Must be one of "header","cookie", or "query_parameter".
	// Cannot be specified along with SourceIP.
	Field string `json:",omitempty"`

	// FieldValue is the value to hash.
	// ie. header name, cookie name, URL query parameter name
	// Cannot be specified along with SourceIP.
	FieldValue string `json:",omitempty" alias:"field_value"`

	// CookieConfig contains configuration for the "cookie" hash policy type.
	CookieConfig *CookieConfig `json:",omitempty" alias:"cookie_config"`

	// SourceIP determines whether the hash should be of the source IP rather than of a field and field value.
	// Cannot be specified along with Field or FieldValue.
	SourceIP bool `json:",omitempty" alias:"source_ip"`

	// Terminal will short circuit the computation of the hash when multiple hash policies are present.
	// If a hash is computed when a Terminal policy is evaluated,
	// then that hash will be used and subsequent hash policies will be ignored.
	Terminal bool `json:",omitempty"`
}

// CookieConfig contains configuration for the "cookie" hash policy type.
// This is specified to have Envoy generate a cookie for a client on its first request.
type CookieConfig struct {
	// Generates a session cookie with no expiration.
	Session bool `json:",omitempty"`

	// TTL for generated cookies. Cannot be specified for session cookies.
	TTL time.Duration `json:",omitempty"`

	// The path to set for the cookie
	Path string `json:",omitempty"`
}

func (lb *LoadBalancer) IsHashBased() bool {
	if lb == nil {
		return false
	}

	switch lb.Policy {
	case LBPolicyMaglev, LBPolicyRingHash:
		return true
	default:
		return false
	}
}

type discoveryChainConfigEntry interface {
	ConfigEntry
	// ListRelatedServices returns a list of other names of services referenced
	// in this config entry.
	ListRelatedServices() []ServiceID
}

func canReadDiscoveryChain(entry discoveryChainConfigEntry, authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	entry.GetEnterpriseMeta().FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().ServiceReadAllowed(entry.GetName(), &authzContext)
}

func canWriteDiscoveryChain(entry discoveryChainConfigEntry, authz acl.Authorizer) error {
	entryID := NewServiceID(entry.GetName(), entry.GetEnterpriseMeta())

	var authzContext acl.AuthorizerContext
	entryID.FillAuthzContext(&authzContext)

	name := entry.GetName()

	if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(name, &authzContext); err != nil {
		return err
	}

	for _, svc := range entry.ListRelatedServices() {
		if entryID == svc {
			continue
		}

		svc.FillAuthzContext(&authzContext)
		// You only need read on related services to redirect traffic flow for
		// your own service.
		if err := authz.ToAllowAuthorizer().ServiceReadAllowed(svc.ID, &authzContext); err != nil {
			return err
		}
	}
	return nil
}

// DiscoveryChainRequest is used when requesting the discovery chain for a
// service.
type DiscoveryChainRequest struct {
	Name                 string
	EvaluateInDatacenter string
	EvaluateInNamespace  string
	EvaluateInPartition  string

	// OverrideMeshGateway allows for the mesh gateway setting to be overridden
	// for any resolver in the compiled chain.
	OverrideMeshGateway MeshGatewayConfig

	// OverrideProtocol allows for the final protocol for the chain to be
	// altered.
	//
	// - If the chain ordinarily would be TCP and an L7 protocol is passed here
	// the chain will not include Routers or Splitters.
	//
	// - If the chain ordinarily would be L7 and TCP is passed here the chain
	// will not include Routers or Splitters.
	OverrideProtocol string

	// OverrideConnectTimeout allows for the ConnectTimeout setting to be
	// overridden for any resolver in the compiled chain.
	OverrideConnectTimeout time.Duration

	Datacenter string // where to route the RPC
	QueryOptions
}

func (r *DiscoveryChainRequest) RequestDatacenter() string {
	return r.Datacenter
}

func (r *DiscoveryChainRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	v, err := hashstructure.Hash(struct {
		Name                   string
		EvaluateInDatacenter   string
		EvaluateInNamespace    string
		EvaluateInPartition    string
		OverrideMeshGateway    MeshGatewayConfig
		OverrideProtocol       string
		OverrideConnectTimeout time.Duration
		Filter                 string
	}{
		Name:                   r.Name,
		EvaluateInDatacenter:   r.EvaluateInDatacenter,
		EvaluateInNamespace:    r.EvaluateInNamespace,
		EvaluateInPartition:    r.EvaluateInPartition,
		OverrideMeshGateway:    r.OverrideMeshGateway,
		OverrideProtocol:       r.OverrideProtocol,
		OverrideConnectTimeout: r.OverrideConnectTimeout,
		Filter:                 r.QueryOptions.Filter,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

type DiscoveryChainResponse struct {
	Chain *CompiledDiscoveryChain
	QueryMeta
}

type ConfigEntryGraphError struct {
	// one of Message or Err should be set
	Message string
	Err     error
}

func (e *ConfigEntryGraphError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

var (
	validServiceSubset     = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	serviceSubsetMaxLength = 63
)

// validateServiceSubset checks if the provided name can be used as an service
// subset. Because these are used in SNI headers they must a DNS label per
// RFC-1035/RFC-1123.
func validateServiceSubset(subset string) error {
	if subset == "" || len(subset) > serviceSubsetMaxLength {
		return fmt.Errorf("must be non-empty and 63 characters or fewer")
	}
	if !validServiceSubset.MatchString(subset) {
		return fmt.Errorf("must be 63 characters or fewer, begin or end with lower case alphanumeric characters, and contain lower case alphanumeric characters or '-' in between")
	}
	return nil
}

func defaultIfEmpty(val, defaultVal string) string {
	if val != "" {
		return val
	}
	return defaultVal
}

func IsProtocolHTTPLike(protocol string) bool {
	switch protocol {
	case "http", "http2", "grpc":
		return true
	default:
		return false
	}
}

// HTTPHeaderModifiers is a set of rules for HTTP header modification that
// should be performed by proxies as the request passes through them. It can
// operate on either request or response headers depending on the context in
// which it is used.
type HTTPHeaderModifiers struct {
	// Add is a set of name -> value pairs that should be appended to the request
	// or response (i.e. allowing duplicates if the same header already exists).
	Add map[string]string `json:",omitempty"`

	// Set is a set of name -> value pairs that should be added to the request or
	// response, overwriting any existing header values of the same name.
	Set map[string]string `json:",omitempty"`

	// Remove is the set of header names that should be stripped from the request
	// or response.
	Remove []string `json:",omitempty"`
}

func (m *HTTPHeaderModifiers) IsZero() bool {
	if m == nil {
		return true
	}
	return len(m.Add) == 0 && len(m.Set) == 0 && len(m.Remove) == 0
}

func (m *HTTPHeaderModifiers) Validate(protocol string) error {
	if m.IsZero() {
		return nil
	}
	if !IsProtocolHTTPLike(protocol) {
		// Non nil but context is not an httpish protocol
		return fmt.Errorf("only valid for http, http2 and grpc protocols")
	}
	return nil
}

// Clone returns a deep-copy of m unless m is nil
func (m *HTTPHeaderModifiers) Clone() (*HTTPHeaderModifiers, error) {
	if m == nil {
		return nil, nil
	}

	cpy, err := copystructure.Copy(m)
	if err != nil {
		return nil, err
	}
	m = cpy.(*HTTPHeaderModifiers)
	return m, nil
}

// MergeHTTPHeaderModifiers takes a base HTTPHeaderModifiers and merges in field
// defined in overrides. Precedence is given to the overrides field if there is
// a collision. The resulting object is returned leaving both base and overrides
// unchanged. The `Add` field in override also replaces same-named keys of base
// since we have no way to express multiple adds to the same key. We could
// change that, but it makes the config syntax more complex for a huge edgecase.
func MergeHTTPHeaderModifiers(base, overrides *HTTPHeaderModifiers) (*HTTPHeaderModifiers, error) {
	if base.IsZero() {
		return overrides.Clone()
	}

	merged, err := base.Clone()
	if err != nil {
		return nil, err
	}

	if overrides.IsZero() {
		return merged, nil
	}

	for k, v := range overrides.Add {
		merged.Add[k] = v
	}
	for k, v := range overrides.Set {
		merged.Set[k] = v
	}

	// Deduplicate removes.
	removed := make(map[string]struct{})
	for _, k := range merged.Remove {
		removed[k] = struct{}{}
	}
	for _, k := range overrides.Remove {
		if _, ok := removed[k]; !ok {
			merged.Remove = append(merged.Remove, k)
		}
	}

	return merged, nil
}
