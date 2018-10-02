package services

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

// Domain represents a domain used by users to access their website.
type Domain struct {
	// Specifies the domain used to access the assets on their website, for which
	// a CNAME is given to the CDN provider.
	Domain string `json:"domain" required:"true"`
	// Specifies the protocol used to access the assets on this domain. Only "http"
	// or "https" are currently allowed. The default is "http".
	Protocol string `json:"protocol,omitempty"`
}

func (d Domain) toPatchValue() interface{} {
	r := make(map[string]interface{})
	r["domain"] = d.Domain
	if d.Protocol != "" {
		r["protocol"] = d.Protocol
	}
	return r
}

func (d Domain) appropriatePath() Path {
	return PathDomains
}

func (d Domain) renderRootOr(render func(p Path) string) string {
	return render(d.appropriatePath())
}

// DomainList provides a useful way to perform bulk operations in a single Patch.
type DomainList []Domain

func (list DomainList) toPatchValue() interface{} {
	r := make([]interface{}, len(list))
	for i, domain := range list {
		r[i] = domain.toPatchValue()
	}
	return r
}

func (list DomainList) appropriatePath() Path {
	return PathDomains
}

func (list DomainList) renderRootOr(_ func(p Path) string) string {
	return list.appropriatePath().renderRoot()
}

// OriginRule represents a rule that defines when an origin should be accessed.
type OriginRule struct {
	// Specifies the name of this rule.
	Name string `json:"name" required:"true"`
	// Specifies the request URL this rule should match for this origin to be used. Regex is supported.
	RequestURL string `json:"request_url" required:"true"`
}

// Origin specifies a list of origin domains or IP addresses where the original assets are stored.
type Origin struct {
	// Specifies the URL or IP address to pull origin content from.
	Origin string `json:"origin" required:"true"`
	// Specifies the port used to access the origin. The default is port 80.
	Port int `json:"port,omitempty"`
	// Specifies whether or not to use HTTPS to access the origin. The default
	// is false.
	SSL bool `json:"ssl"`
	// Specifies a collection of rules that define the conditions when this origin
	// should be accessed. If there is more than one origin, the rules parameter is required.
	Rules []OriginRule `json:"rules,omitempty"`
}

func (o Origin) toPatchValue() interface{} {
	r := make(map[string]interface{})
	r["origin"] = o.Origin
	r["port"] = o.Port
	r["ssl"] = o.SSL
	if len(o.Rules) > 0 {
		r["rules"] = make([]map[string]interface{}, len(o.Rules))
		for index, rule := range o.Rules {
			submap := r["rules"].([]map[string]interface{})[index]
			submap["name"] = rule.Name
			submap["request_url"] = rule.RequestURL
		}
	}
	return r
}

func (o Origin) appropriatePath() Path {
	return PathOrigins
}

func (o Origin) renderRootOr(render func(p Path) string) string {
	return render(o.appropriatePath())
}

// OriginList provides a useful way to perform bulk operations in a single Patch.
type OriginList []Origin

func (list OriginList) toPatchValue() interface{} {
	r := make([]interface{}, len(list))
	for i, origin := range list {
		r[i] = origin.toPatchValue()
	}
	return r
}

func (list OriginList) appropriatePath() Path {
	return PathOrigins
}

func (list OriginList) renderRootOr(_ func(p Path) string) string {
	return list.appropriatePath().renderRoot()
}

// TTLRule specifies a rule that determines if a TTL should be applied to an asset.
type TTLRule struct {
	// Specifies the name of this rule.
	Name string `json:"name" required:"true"`
	// Specifies the request URL this rule should match for this TTL to be used. Regex is supported.
	RequestURL string `json:"request_url" required:"true"`
}

// CacheRule specifies the TTL rules for the assets under this service.
type CacheRule struct {
	// Specifies the name of this caching rule. Note: 'default' is a reserved name used for the default TTL setting.
	Name string `json:"name" required:"true"`
	// Specifies the TTL to apply.
	TTL int `json:"ttl,omitempty"`
	// Specifies a collection of rules that determine if this TTL should be applied to an asset.
	Rules []TTLRule `json:"rules,omitempty"`
}

func (c CacheRule) toPatchValue() interface{} {
	r := make(map[string]interface{})
	r["name"] = c.Name
	r["ttl"] = c.TTL
	r["rules"] = make([]map[string]interface{}, len(c.Rules))
	for index, rule := range c.Rules {
		submap := r["rules"].([]map[string]interface{})[index]
		submap["name"] = rule.Name
		submap["request_url"] = rule.RequestURL
	}
	return r
}

func (c CacheRule) appropriatePath() Path {
	return PathCaching
}

func (c CacheRule) renderRootOr(render func(p Path) string) string {
	return render(c.appropriatePath())
}

// CacheRuleList provides a useful way to perform bulk operations in a single Patch.
type CacheRuleList []CacheRule

func (list CacheRuleList) toPatchValue() interface{} {
	r := make([]interface{}, len(list))
	for i, rule := range list {
		r[i] = rule.toPatchValue()
	}
	return r
}

func (list CacheRuleList) appropriatePath() Path {
	return PathCaching
}

func (list CacheRuleList) renderRootOr(_ func(p Path) string) string {
	return list.appropriatePath().renderRoot()
}

// RestrictionRule specifies a rule that determines if this restriction should be applied to an asset.
type RestrictionRule struct {
	// Specifies the name of this rule.
	Name string `json:"name" required:"true"`
	// Specifies the http host that requests must come from.
	Referrer string `json:"referrer,omitempty"`
}

// Restriction specifies a restriction that defines who can access assets (content from the CDN cache).
type Restriction struct {
	// Specifies the name of this restriction.
	Name string `json:"name" required:"true"`
	// Specifies a collection of rules that determine if this TTL should be applied to an asset.
	Rules []RestrictionRule `json:"rules,omitempty"`
}

// Error specifies an error that occurred during the previous service action.
type Error struct {
	// Specifies an error message detailing why there is an error.
	Message string `json:"message"`
}

// Service represents a CDN service resource.
type Service struct {
	// Specifies the service ID that represents distributed content. The value is
	// a UUID, such as 96737ae3-cfc1-4c72-be88-5d0e7cc9a3f0, that is generated by the server.
	ID string `json:"id"`
	// Specifies the name of the service.
	Name string `json:"name"`
	// Specifies a list of domains used by users to access their website.
	Domains []Domain `json:"domains"`
	// Specifies a list of origin domains or IP addresses where the original assets are stored.
	Origins []Origin `json:"origins"`
	// Specifies the TTL rules for the assets under this service. Supports wildcards for fine grained control.
	Caching []CacheRule `json:"caching"`
	// Specifies the restrictions that define who can access assets (content from the CDN cache).
	Restrictions []Restriction `json:"restrictions"`
	// Specifies the CDN provider flavor ID to use. For a list of flavors, see the operation to list the available flavors.
	FlavorID string `json:"flavor_id"`
	// Specifies the current status of the service.
	Status string `json:"status"`
	// Specifies the list of errors that occurred during the previous service action.
	Errors []Error `json:"errors"`
	// Specifies the self-navigating JSON document paths.
	Links []gophercloud.Link `json:"links"`
}

// ServicePage is the page returned by a pager when traversing over a
// collection of CDN services.
type ServicePage struct {
	pagination.MarkerPageBase
}

// IsEmpty returns true if a ListResult contains no services.
func (r ServicePage) IsEmpty() (bool, error) {
	services, err := ExtractServices(r)
	return len(services) == 0, err
}

// LastMarker returns the last service in a ListResult.
func (r ServicePage) LastMarker() (string, error) {
	services, err := ExtractServices(r)
	if err != nil {
		return "", err
	}
	if len(services) == 0 {
		return "", nil
	}
	return (services[len(services)-1]).ID, nil
}

// ExtractServices is a function that takes a ListResult and returns the services' information.
func ExtractServices(r pagination.Page) ([]Service, error) {
	var s struct {
		Services []Service `json:"services"`
	}
	err := (r.(ServicePage)).ExtractInto(&s)
	return s.Services, err
}

// CreateResult represents the result of a Create operation.
type CreateResult struct {
	gophercloud.Result
}

// Extract is a method that extracts the location of a newly created service.
func (r CreateResult) Extract() (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	if l, ok := r.Header["Location"]; ok && len(l) > 0 {
		return l[0], nil
	}
	return "", nil
}

// GetResult represents the result of a get operation.
type GetResult struct {
	gophercloud.Result
}

// Extract is a function that extracts a service from a GetResult.
func (r GetResult) Extract() (*Service, error) {
	var s Service
	err := r.ExtractInto(&s)
	return &s, err
}

// UpdateResult represents the result of a Update operation.
type UpdateResult struct {
	gophercloud.Result
}

// Extract is a method that extracts the location of an updated service.
func (r UpdateResult) Extract() (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	if l, ok := r.Header["Location"]; ok && len(l) > 0 {
		return l[0], nil
	}
	return "", nil
}

// DeleteResult represents the result of a Delete operation.
type DeleteResult struct {
	gophercloud.ErrResult
}
