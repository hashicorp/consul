package structs

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/mitchellh/hashstructure"
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

func (e *ServiceRouterConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceRouter

	for _, route := range e.Routes {
		if route.Match == nil || route.Match.HTTP == nil {
			continue
		}

		httpMatch := route.Match.HTTP
		if len(httpMatch.Methods) == 0 {
			continue
		}

		for j := 0; j < len(httpMatch.Methods); j++ {
			httpMatch.Methods[j] = strings.ToUpper(httpMatch.Methods[j])
		}
	}

	return nil
}

func (e *ServiceRouterConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
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
		}
	}

	return nil
}

func (e *ServiceRouterConfigEntry) CanRead(rule acl.Authorizer) bool {
	return canReadDiscoveryChain(e, rule)
}

func (e *ServiceRouterConfigEntry) CanWrite(rule acl.Authorizer) bool {
	return canWriteDiscoveryChain(e, rule)
}

func (e *ServiceRouterConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceRouterConfigEntry) ListRelatedServices() []string {
	found := make(map[string]struct{})

	// We always inject a default catch-all route to the same service as the router.
	found[e.Name] = struct{}{}

	for _, route := range e.Routes {
		if route.Destination != nil && route.Destination.Service != "" {
			found[route.Destination.Service] = struct{}{}
		}
	}

	if len(found) == 0 {
		return nil
	}

	out := make([]string, 0, len(found))
	for svc, _ := range found {
		out = append(out, svc)
	}
	sort.Strings(out)
	return out
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
	PathExact  string `json:",omitempty"`
	PathPrefix string `json:",omitempty"`
	PathRegex  string `json:",omitempty"`

	Header     []ServiceRouteHTTPMatchHeader     `json:",omitempty"`
	QueryParam []ServiceRouteHTTPMatchQueryParam `json:",omitempty"`
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
	ServiceSubset string `json:",omitempty"`

	// Namespace is the namespace to resolve the service from instead of the
	// current namespace. If empty the current namespace is assumed.
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	Namespace string `json:",omitempty"`

	// PrefixRewrite allows for the proxied request to have its matching path
	// prefix modified before being sent to the destination. Described more
	// below in the envoy implementation section.
	PrefixRewrite string `json:",omitempty"`

	// RequestTimeout is the total amount of time permitted for the entire
	// downstream request (and retries) to be processed.
	RequestTimeout time.Duration `json:",omitempty"`

	// NumRetries is the number of times to retry the request when a retryable
	// result occurs. This seems fairly proxy agnostic.
	NumRetries uint32 `json:",omitempty"`

	// RetryOnConnectFailure allows for connection failure errors to trigger a
	// retry. This should be expressible in other proxies as it's just a layer
	// 4 failure bubbling up to layer 7.
	RetryOnConnectFailure bool `json:",omitempty"`

	// RetryOnStatusCodes is a flat list of http response status codes that are
	// eligible for retry. This again should be feasible in any sane proxy.
	RetryOnStatusCodes []uint32 `json:",omitempty"`
}

func (e *ServiceRouteDestination) MarshalJSON() ([]byte, error) {
	type Alias ServiceRouteDestination
	exported := &struct {
		RequestTimeout string `json:",omitempty"`
		*Alias
	}{
		RequestTimeout: e.RequestTimeout.String(),
		Alias:          (*Alias)(e),
	}
	if e.RequestTimeout == 0 {
		exported.RequestTimeout = ""
	}

	return json.Marshal(exported)
}

func (e *ServiceRouteDestination) UnmarshalJSON(data []byte) error {
	type Alias ServiceRouteDestination
	aux := &struct {
		RequestTimeout string
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.RequestTimeout != "" {
		if e.RequestTimeout, err = time.ParseDuration(aux.RequestTimeout); err != nil {
			return err
		}
	}
	return nil
}

func (d *ServiceRouteDestination) HasRetryFeatures() bool {
	return d.NumRetries > 0 || d.RetryOnConnectFailure || len(d.RetryOnStatusCodes) > 0
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

func (e *ServiceSplitterConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceSplitter

	// This slightly massages inputs to enforce that the smallest representable
	// weight is 1/10000 or .01%

	if len(e.Splits) > 0 {
		for i, split := range e.Splits {
			e.Splits[i].Weight = NormalizeServiceSplitWeight(split.Weight)
		}
	}

	return nil
}

func NormalizeServiceSplitWeight(weight float32) float32 {
	weightScaled := scaleWeight(weight)
	return float32(float32(weightScaled) / 100.0)
}

func (e *ServiceSplitterConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if len(e.Splits) == 0 {
		return fmt.Errorf("no splits configured")
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
				"split destination occurs more than once: service=%q, subset=%q, namespace=%q",
				splitKey.Service, splitKey.ServiceSubset, splitKey.Namespace,
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

func (e *ServiceSplitterConfigEntry) CanRead(rule acl.Authorizer) bool {
	return canReadDiscoveryChain(e, rule)
}

func (e *ServiceSplitterConfigEntry) CanWrite(rule acl.Authorizer) bool {
	return canWriteDiscoveryChain(e, rule)
}

func (e *ServiceSplitterConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceSplitterConfigEntry) ListRelatedServices() []string {
	found := make(map[string]struct{})

	for _, split := range e.Splits {
		if split.Service != "" {
			found[split.Service] = struct{}{}
		}
	}

	if len(found) == 0 {
		return nil
	}

	out := make([]string, 0, len(found))
	for svc, _ := range found {
		out = append(out, svc)
	}
	sort.Strings(out)
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
	ServiceSubset string `json:",omitempty"`

	// Namespace is the namespace to resolve the service from instead of the
	// current namespace. If empty the current namespace is assumed (optional).
	//
	// If this field is specified then this route is ineligible for further
	// splitting.
	Namespace string `json:",omitempty"`
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
	DefaultSubset string `json:",omitempty"`

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
	ConnectTimeout time.Duration `json:",omitempty"`

	RaftIndex
}

func (e *ServiceResolverConfigEntry) MarshalJSON() ([]byte, error) {
	type Alias ServiceResolverConfigEntry
	exported := &struct {
		ConnectTimeout string `json:",omitempty"`
		*Alias
	}{
		ConnectTimeout: e.ConnectTimeout.String(),
		Alias:          (*Alias)(e),
	}
	if e.ConnectTimeout == 0 {
		exported.ConnectTimeout = ""
	}

	return json.Marshal(exported)
}

func (e *ServiceResolverConfigEntry) UnmarshalJSON(data []byte) error {
	type Alias ServiceResolverConfigEntry
	aux := &struct {
		ConnectTimeout string
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.ConnectTimeout != "" {
		if e.ConnectTimeout, err = time.ParseDuration(aux.ConnectTimeout); err != nil {
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
		e.ConnectTimeout == 0
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

func (e *ServiceResolverConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = ServiceResolver

	return nil
}

func (e *ServiceResolverConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if len(e.Subsets) > 0 {
		for name, _ := range e.Subsets {
			if name == "" {
				return fmt.Errorf("Subset defined with empty name")
			}
			if err := validateServiceSubset(name); err != nil {
				return fmt.Errorf("Subset %q is invalid: %v", name, err)
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
		r := e.Redirect

		if len(e.Failover) > 0 {
			return fmt.Errorf("Redirect and Failover cannot both be set")
		}

		// TODO(rb): prevent subsets and default subsets from being defined?

		if r.Service == "" && r.ServiceSubset == "" && r.Namespace == "" && r.Datacenter == "" {
			return fmt.Errorf("Redirect is empty")
		}

		if r.Service == "" {
			if r.ServiceSubset != "" {
				return fmt.Errorf("Redirect.ServiceSubset defined without Redirect.Service")
			}
			if r.Namespace != "" {
				return fmt.Errorf("Redirect.Namespace defined without Redirect.Service")
			}
		} else if r.Service == e.Name {
			if r.ServiceSubset != "" && !isSubset(r.ServiceSubset) {
				return fmt.Errorf("Redirect.ServiceSubset %q is not a valid subset of %q", r.ServiceSubset, r.Service)
			}
		}
	}

	if len(e.Failover) > 0 {
		for subset, f := range e.Failover {
			if subset != "*" && !isSubset(subset) {
				return fmt.Errorf("Bad Failover[%q]: not a valid subset", subset)
			}

			if f.Service == "" && f.ServiceSubset == "" && f.Namespace == "" && len(f.Datacenters) == 0 {
				return fmt.Errorf("Bad Failover[%q] one of Service, ServiceSubset, Namespace, or Datacenters is required", subset)
			}

			if f.ServiceSubset != "" {
				if f.Service == "" || f.Service == e.Name {
					if !isSubset(f.ServiceSubset) {
						return fmt.Errorf("Bad Failover[%q].ServiceSubset %q is not a valid subset of %q", subset, f.ServiceSubset, f.Service)
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

	return nil
}

func (e *ServiceResolverConfigEntry) CanRead(rule acl.Authorizer) bool {
	return canReadDiscoveryChain(e, rule)
}

func (e *ServiceResolverConfigEntry) CanWrite(rule acl.Authorizer) bool {
	return canWriteDiscoveryChain(e, rule)
}

func (e *ServiceResolverConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceResolverConfigEntry) ListRelatedServices() []string {
	found := make(map[string]struct{})

	if e.Redirect != nil {
		if e.Redirect.Service != "" {
			found[e.Redirect.Service] = struct{}{}
		}
	}

	if len(e.Failover) > 0 {
		for _, failover := range e.Failover {
			if failover.Service != "" {
				found[failover.Service] = struct{}{}
			}
		}
	}

	if len(found) == 0 {
		return nil
	}

	out := make([]string, 0, len(found))
	for svc, _ := range found {
		out = append(out, svc)
	}
	sort.Strings(out)
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
	OnlyPassing bool `json:",omitempty"`
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
	ServiceSubset string `json:",omitempty"`

	// Namespace is the namespace to resolve the service from instead of the
	// current one (optional).
	Namespace string `json:",omitempty"`

	// Datacenter is the datacenter to resolve the service from instead of the
	// current one (optional).
	Datacenter string `json:",omitempty"`
}

// There are some restrictions on what is allowed in here:
//
// - Service, ServiceSubset, Namespace, and Datacenters cannot all be
//   empty at once.
//
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
	ServiceSubset string `json:",omitempty"`

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
}

type discoveryChainConfigEntry interface {
	ConfigEntry
	// ListRelatedServices returns a list of other names of services referenced
	// in this config entry.
	ListRelatedServices() []string
}

func canReadDiscoveryChain(entry discoveryChainConfigEntry, rule acl.Authorizer) bool {
	return rule.ServiceRead(entry.GetName())
}

func canWriteDiscoveryChain(entry discoveryChainConfigEntry, rule acl.Authorizer) bool {
	name := entry.GetName()

	if !rule.ServiceWrite(name, nil) {
		return false
	}

	for _, svc := range entry.ListRelatedServices() {
		if svc == name {
			continue
		}

		// You only need read on related services to redirect traffic flow for
		// your own service.
		if !rule.ServiceRead(svc) {
			return false
		}
	}
	return true
}

// DiscoveryChainConfigEntries wraps just the raw cross-referenced config
// entries. None of these are defaulted.
type DiscoveryChainConfigEntries struct {
	Routers     map[string]*ServiceRouterConfigEntry
	Splitters   map[string]*ServiceSplitterConfigEntry
	Resolvers   map[string]*ServiceResolverConfigEntry
	Services    map[string]*ServiceConfigEntry
	GlobalProxy *ProxyConfigEntry
}

func NewDiscoveryChainConfigEntries() *DiscoveryChainConfigEntries {
	return &DiscoveryChainConfigEntries{
		Routers:   make(map[string]*ServiceRouterConfigEntry),
		Splitters: make(map[string]*ServiceSplitterConfigEntry),
		Resolvers: make(map[string]*ServiceResolverConfigEntry),
		Services:  make(map[string]*ServiceConfigEntry),
	}
}

func (e *DiscoveryChainConfigEntries) GetRouter(name string) *ServiceRouterConfigEntry {
	if e.Routers != nil {
		return e.Routers[name]
	}
	return nil
}

func (e *DiscoveryChainConfigEntries) GetSplitter(name string) *ServiceSplitterConfigEntry {
	if e.Splitters != nil {
		return e.Splitters[name]
	}
	return nil
}

func (e *DiscoveryChainConfigEntries) GetResolver(name string) *ServiceResolverConfigEntry {
	if e.Resolvers != nil {
		return e.Resolvers[name]
	}
	return nil
}

func (e *DiscoveryChainConfigEntries) GetService(name string) *ServiceConfigEntry {
	if e.Services != nil {
		return e.Services[name]
	}
	return nil
}

// AddRouters adds router configs. Convenience function for testing.
func (e *DiscoveryChainConfigEntries) AddRouters(entries ...*ServiceRouterConfigEntry) {
	if e.Routers == nil {
		e.Routers = make(map[string]*ServiceRouterConfigEntry)
	}
	for _, entry := range entries {
		e.Routers[entry.Name] = entry
	}
}

// AddSplitters adds splitter configs. Convenience function for testing.
func (e *DiscoveryChainConfigEntries) AddSplitters(entries ...*ServiceSplitterConfigEntry) {
	if e.Splitters == nil {
		e.Splitters = make(map[string]*ServiceSplitterConfigEntry)
	}
	for _, entry := range entries {
		e.Splitters[entry.Name] = entry
	}
}

// AddResolvers adds resolver configs. Convenience function for testing.
func (e *DiscoveryChainConfigEntries) AddResolvers(entries ...*ServiceResolverConfigEntry) {
	if e.Resolvers == nil {
		e.Resolvers = make(map[string]*ServiceResolverConfigEntry)
	}
	for _, entry := range entries {
		e.Resolvers[entry.Name] = entry
	}
}

// AddServices adds service configs. Convenience function for testing.
func (e *DiscoveryChainConfigEntries) AddServices(entries ...*ServiceConfigEntry) {
	if e.Services == nil {
		e.Services = make(map[string]*ServiceConfigEntry)
	}
	for _, entry := range entries {
		e.Services[entry.Name] = entry
	}
}

// AddEntries adds generic configs. Convenience function for testing. Panics on
// operator error.
func (e *DiscoveryChainConfigEntries) AddEntries(entries ...ConfigEntry) {
	for _, entry := range entries {
		switch entry.GetKind() {
		case ServiceRouter:
			e.AddRouters(entry.(*ServiceRouterConfigEntry))
		case ServiceSplitter:
			e.AddSplitters(entry.(*ServiceSplitterConfigEntry))
		case ServiceResolver:
			e.AddResolvers(entry.(*ServiceResolverConfigEntry))
		case ServiceDefaults:
			e.AddServices(entry.(*ServiceConfigEntry))
		case ProxyDefaults:
			if entry.GetName() != ProxyConfigGlobal {
				panic("the only supported proxy-defaults name is '" + ProxyConfigGlobal + "'")
			}
			e.GlobalProxy = entry.(*ProxyConfigEntry)
		default:
			panic("unhandled config entry kind: " + entry.GetKind())
		}
	}
}

func (e *DiscoveryChainConfigEntries) IsEmpty() bool {
	return e.IsChainEmpty() && len(e.Services) == 0 && e.GlobalProxy == nil
}

func (e *DiscoveryChainConfigEntries) IsChainEmpty() bool {
	return len(e.Routers) == 0 && len(e.Splitters) == 0 && len(e.Resolvers) == 0
}

// DiscoveryChainRequest is used when requesting the discovery chain for a
// service.
type DiscoveryChainRequest struct {
	Name                 string
	EvaluateInDatacenter string
	EvaluateInNamespace  string

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
		OverrideMeshGateway    MeshGatewayConfig
		OverrideProtocol       string
		OverrideConnectTimeout time.Duration
	}{
		Name:                   r.Name,
		EvaluateInDatacenter:   r.EvaluateInDatacenter,
		EvaluateInNamespace:    r.EvaluateInNamespace,
		OverrideMeshGateway:    r.OverrideMeshGateway,
		OverrideProtocol:       r.OverrideProtocol,
		OverrideConnectTimeout: r.OverrideConnectTimeout,
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
