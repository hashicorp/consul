package api

import "time"

type ServiceRouterConfigEntry struct {
	Kind string
	Name string

	Routes []ServiceRoute

	CreateIndex uint64
	ModifyIndex uint64
}

func (e *ServiceRouterConfigEntry) GetKind() string        { return e.Kind }
func (e *ServiceRouterConfigEntry) GetName() string        { return e.Name }
func (e *ServiceRouterConfigEntry) GetCreateIndex() uint64 { return e.CreateIndex }
func (e *ServiceRouterConfigEntry) GetModifyIndex() uint64 { return e.ModifyIndex }

type ServiceRoute struct {
	Match       *ServiceRouteMatch       `json:",omitempty"`
	Destination *ServiceRouteDestination `json:",omitempty"`
}

type ServiceRouteMatch struct {
	HTTP *ServiceRouteHTTPMatch `json:",omitempty"`
}

type ServiceRouteHTTPMatch struct {
	PathExact  string `json:",omitempty"`
	PathPrefix string `json:",omitempty"`
	PathRegex  string `json:",omitempty"`

	Header     []ServiceRouteHTTPMatchHeader     `json:",omitempty"`
	QueryParam []ServiceRouteHTTPMatchQueryParam `json:",omitempty"`

	Methods []string `json:",omitempty"`
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
	Name  string
	Value string `json:",omitempty"`
	Regex bool   `json:",omitempty"`
}

type ServiceRouteDestination struct {
	Service               string        `json:",omitempty"`
	ServiceSubset         string        `json:",omitempty"`
	Namespace             string        `json:",omitempty"`
	PrefixRewrite         string        `json:",omitempty"`
	RequestTimeout        time.Duration `json:",omitempty"`
	NumRetries            uint32        `json:",omitempty"`
	RetryOnConnectFailure bool          `json:",omitempty"`
	RetryOnStatusCodes    []uint32      `json:",omitempty"`
}

type ServiceSplitterConfigEntry struct {
	Kind string
	Name string

	Splits []ServiceSplit

	CreateIndex uint64
	ModifyIndex uint64
}

func (e *ServiceSplitterConfigEntry) GetKind() string        { return e.Kind }
func (e *ServiceSplitterConfigEntry) GetName() string        { return e.Name }
func (e *ServiceSplitterConfigEntry) GetCreateIndex() uint64 { return e.CreateIndex }
func (e *ServiceSplitterConfigEntry) GetModifyIndex() uint64 { return e.ModifyIndex }

type ServiceSplit struct {
	Weight        float32
	Service       string `json:",omitempty"`
	ServiceSubset string `json:",omitempty"`
	Namespace     string `json:",omitempty"`
}

type ServiceResolverConfigEntry struct {
	Kind string
	Name string

	DefaultSubset  string                             `json:",omitempty"`
	Subsets        map[string]ServiceResolverSubset   `json:",omitempty"`
	Redirect       *ServiceResolverRedirect           `json:",omitempty"`
	Failover       map[string]ServiceResolverFailover `json:",omitempty"`
	ConnectTimeout time.Duration                      `json:",omitempty"`

	CreateIndex uint64
	ModifyIndex uint64
}

func (e *ServiceResolverConfigEntry) GetKind() string        { return ServiceResolver }
func (e *ServiceResolverConfigEntry) GetName() string        { return e.Name }
func (e *ServiceResolverConfigEntry) GetCreateIndex() uint64 { return e.CreateIndex }
func (e *ServiceResolverConfigEntry) GetModifyIndex() uint64 { return e.ModifyIndex }

type ServiceResolverSubset struct {
	Filter      string `json:",omitempty"`
	OnlyPassing bool   `json:",omitempty"`
}

type ServiceResolverRedirect struct {
	Service       string `json:",omitempty"`
	ServiceSubset string `json:",omitempty"`
	Namespace     string `json:",omitempty"`
	Datacenter    string `json:",omitempty"`
}

type ServiceResolverFailover struct {
	Service                string   `json:",omitempty"`
	ServiceSubset          string   `json:",omitempty"`
	Namespace              string   `json:",omitempty"`
	Datacenters            []string `json:",omitempty"`
	OverprovisioningFactor int      `json:",omitempty"`

	// TODO(rb): bring this back after normal DC failover works
	// NearestN               int      `json:",omitempty"`
}
