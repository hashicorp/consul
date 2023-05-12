// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-sockaddr/template"
	"github.com/hashicorp/memberlist"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/authmethod/ssoauth"
	consulrate "github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/dns"
	hcpconfig "github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/stringslice"
	libtempl "github.com/hashicorp/consul/lib/template"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

type FlagValuesTarget = decodeTarget

// LoadOpts used by Load to construct and validate a RuntimeConfig.
type LoadOpts struct {
	// FlagValues contains the command line arguments that can also be set
	// in a config file.
	FlagValues FlagValuesTarget

	// ConfigFiles is a slice of paths to config files and directories that will
	// be loaded.
	//
	// It is an error for any config files to have an extension other than `hcl`
	// or `json`, unless ConfigFormat is also set. However, non-HCL/JSON files in
	// a config directory are merely skipped, with a warning.
	ConfigFiles []string

	// ConfigFormat forces all config files to be interpreted as this format
	// independent of their extension. Value may be `hcl` or `json`.
	ConfigFormat string

	// DevMode indicates whether the agent should be started in development
	// mode. This cannot be configured in a config file.
	DevMode *bool

	// HCL is a slice of config data in hcl format. Each one will be loaded as
	// if it were the source of a config file. Values from HCL will override
	// values from ConfigFiles and FlagValues.
	HCL []string

	// DefaultConfig is an optional source that is applied after other defaults
	// but before ConfigFiles and all other user specified config.
	DefaultConfig Source

	// Overrides are optional config sources that are applied as the very last
	// config source so they can override any previous values.
	Overrides []Source

	// hostname is a shim for testing, allowing tests to specify a replacement
	// for os.Hostname.
	hostname func() (string, error)

	// getPrivateIPv4 and getPublicIPv6 are shims for testing, allowing tests to
	// specify a replacement for ipaddr.GetPrivateIPv4 and ipaddr.GetPublicIPv6.
	getPrivateIPv4 func() ([]*net.IPAddr, error)
	getPublicIPv6  func() ([]*net.IPAddr, error)

	// sources is a shim for testing. Many test cases used explicit sources instead
	// paths to config files. This shim allows us to preserve those test cases
	// while using Load as the entrypoint.
	sources []Source
}

// Load will build the configuration including the config source injected
// after all other defaults but before any user supplied configuration and the overrides
// source injected as the final source in the configuration parsing chain.
//
// The caller is responsible for handling any warnings in LoadResult.Warnings.
func Load(opts LoadOpts) (LoadResult, error) {
	r := LoadResult{}
	b, err := newBuilder(opts)
	if err != nil {
		return r, err
	}
	cfg, err := b.build()
	if err != nil {
		return r, err
	}
	if err := b.validate(cfg); err != nil {
		return r, err
	}
	watcherFiles := stringslice.CloneStringSlice(opts.ConfigFiles)
	return LoadResult{RuntimeConfig: &cfg, Warnings: b.Warnings, WatchedFiles: watcherFiles}, nil
}

// LoadResult is the result returned from Load. The caller is responsible for
// handling any warnings.
type LoadResult struct {
	RuntimeConfig *RuntimeConfig
	Warnings      []string
	WatchedFiles  []string
}

// builder constructs and validates a runtime configuration from multiple
// configuration sources.
//
// The sources are merged in the following order:
//
//   - default configuration
//   - config files in alphabetical order
//   - command line arguments
//   - overrides
//
// The config sources are merged sequentially and later values overwrite
// previously set values. Slice values are merged by concatenating the two slices.
// Map values are merged by over-laying the later maps on top of earlier ones.
type builder struct {
	opts LoadOpts

	// Head, Sources, and Tail are used to manage the order of the
	// config sources, as described in the comments above.
	Head    []Source
	Sources []Source
	Tail    []Source

	// Warnings contains the warnings encountered when
	// parsing the configuration.
	Warnings []string

	// err contains the first error that occurred during
	// building the runtime configuration.
	err error
}

// newBuilder returns a new configuration Builder from the LoadOpts.
func newBuilder(opts LoadOpts) (*builder, error) {
	configFormat := opts.ConfigFormat
	if configFormat != "" && configFormat != "json" && configFormat != "hcl" {
		return nil, fmt.Errorf("config: -config-format must be either 'hcl' or 'json'")
	}

	b := &builder{
		opts: opts,
		Head: []Source{DefaultSource(), DefaultEnterpriseSource()},
	}

	if boolVal(opts.DevMode) {
		b.Head = append(b.Head, DevSource())
	}

	cfg, warns := applyDeprecatedFlags(&opts.FlagValues)
	b.Warnings = append(b.Warnings, warns...)

	// Since the merge logic is to overwrite all fields with later
	// values except slices which are merged by appending later values
	// we need to merge all slice values defined in flags before we
	// merge the config files since the flag values for slices are
	// otherwise appended instead of prepended.
	slices, values := splitSlicesAndValues(cfg)
	b.Head = append(b.Head, LiteralSource{Name: "flags.slices", Config: slices})
	if opts.DefaultConfig != nil {
		b.Head = append(b.Head, opts.DefaultConfig)
	}

	b.Sources = opts.sources
	for _, path := range opts.ConfigFiles {
		sources, err := b.sourcesFromPath(path, opts.ConfigFormat)
		if err != nil {
			return nil, err
		}
		b.Sources = append(b.Sources, sources...)
	}
	b.Tail = append(b.Tail, LiteralSource{Name: "flags.values", Config: values})
	for i, s := range opts.HCL {
		b.Tail = append(b.Tail, FileSource{
			Name:   fmt.Sprintf("flags-%d.hcl", i),
			Format: "hcl",
			Data:   s,
		})
	}
	b.Tail = append(b.Tail, NonUserSource(), DefaultConsulSource(), OverrideEnterpriseSource(), defaultVersionSource())
	if boolVal(opts.DevMode) {
		b.Tail = append(b.Tail, DevConsulSource())
	}
	if len(opts.Overrides) != 0 {
		b.Tail = append(b.Tail, opts.Overrides...)
	}
	return b, nil
}

// sourcesFromPath reads a single config file or all files in a directory (but
// not its sub-directories) and returns Sources created from the
// files.
func (b *builder) sourcesFromPath(path string, format string) ([]Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: Open failed on %s. %s", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("config: Stat failed on %s. %s", path, err)
	}

	if !fi.IsDir() {
		if !shouldParseFile(path, format) {
			return nil, fmt.Errorf("file %v has unknown extension; must be .hcl or .json, or config format must be set", path)
		}

		src, err := newSourceFromFile(path, format)
		if err != nil {
			return nil, err
		}
		return []Source{src}, nil
	}

	fis, err := f.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("config: Readdir failed on %s. %s", path, err)
	}

	// sort files by name
	sort.Sort(byName(fis))

	var sources []Source
	for _, fi := range fis {
		fp := filepath.Join(path, fi.Name())
		// check for a symlink and resolve the path
		if fi.Mode()&os.ModeSymlink > 0 {
			var err error
			fp, err = filepath.EvalSymlinks(fp)
			if err != nil {
				return nil, err
			}
			fi, err = os.Stat(fp)
			if err != nil {
				return nil, err
			}
		}
		// do not recurse into sub dirs
		if fi.IsDir() {
			continue
		}

		if !shouldParseFile(fp, format) {
			b.warn("skipping file %v, extension must be .hcl or .json, or config format must be set", fp)
			continue
		}
		src, err := newSourceFromFile(fp, format)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	return sources, nil
}

// newSourceFromFile creates a Source from the contents of the file at path.
func newSourceFromFile(path string, format string) (Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: failed to read %s: %s", path, err)
	}
	if format == "" {
		format = formatFromFileExtension(path)
	}
	return FileSource{Name: path, Data: string(data), Format: format}, nil
}

// shouldParse file determines whether the file to be read is of a supported extension
func shouldParseFile(path string, configFormat string) bool {
	srcFormat := formatFromFileExtension(path)
	return configFormat != "" || srcFormat == "hcl" || srcFormat == "json"
}

func formatFromFileExtension(name string) string {
	switch {
	case strings.HasSuffix(name, ".json"):
		return "json"
	case strings.HasSuffix(name, ".hcl"):
		return "hcl"
	default:
		return ""
	}
}

type byName []os.FileInfo

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name() < a[j].Name() }

// build constructs the runtime configuration from the config sources
// and the command line flags. The config sources are processed in the
// order they were added with the flags being processed last to give
// precedence over the other sources. If the error is nil then
// warnings can still contain deprecation or format warnings that should
// be presented to the user.
func (b *builder) build() (rt RuntimeConfig, err error) {
	srcs := make([]Source, 0, len(b.Head)+len(b.Sources)+len(b.Tail))
	srcs = append(srcs, b.Head...)
	srcs = append(srcs, b.Sources...)
	srcs = append(srcs, b.Tail...)

	// parse the config sources into a configuration
	var c Config
	for _, s := range srcs {

		c2, md, err := s.Parse()
		switch {
		case err == ErrNoData:
			continue
		case err != nil:
			return RuntimeConfig{}, fmt.Errorf("failed to parse %v: %w", s.Source(), err)
		}

		var unusedErr error
		for _, k := range md.Unused {
			switch {
			case k == "acl_enforce_version_8":
				b.warn("config key %q is deprecated and should be removed", k)
			case strings.HasPrefix(k, "audit.sink[") && strings.HasSuffix(k, "].name"):
				b.warn("config key audit.sink[].name is deprecated and should be removed")
			default:
				unusedErr = multierror.Append(unusedErr, fmt.Errorf("invalid config key %s", k))
			}
		}
		if unusedErr != nil {
			return RuntimeConfig{}, fmt.Errorf("failed to parse %v: %s", s.Source(), unusedErr)
		}

		for _, err := range validateEnterpriseConfigKeys(&c2) {
			b.warn("%s", err)
		}
		b.Warnings = append(b.Warnings, md.Warnings...)

		// if we have a single 'check' or 'service' we need to add them to the
		// list of checks and services first since we cannot merge them
		// generically and later values would clobber earlier ones.
		// TODO: move to applyDeprecatedConfig
		if c2.Check != nil {
			c2.Checks = append(c2.Checks, *c2.Check)
			c2.Check = nil
		}
		if c2.Service != nil {
			c2.Services = append(c2.Services, *c2.Service)
			c2.Service = nil
		}
		c = Merge(c, c2)
	}

	// ----------------------------------------------------------------
	// process/merge some complex values
	//

	dnsServiceTTL := map[string]time.Duration{}
	for k, v := range c.DNS.ServiceTTL {
		dnsServiceTTL[k] = b.durationVal(fmt.Sprintf("dns_config.service_ttl[%q]", k), &v)
	}

	soa := RuntimeSOAConfig{Refresh: 3600, Retry: 600, Expire: 86400, Minttl: 0}
	if c.DNS.SOA != nil {
		if c.DNS.SOA.Expire != nil {
			soa.Expire = *c.DNS.SOA.Expire
		}
		if c.DNS.SOA.Minttl != nil {
			soa.Minttl = *c.DNS.SOA.Minttl
		}
		if c.DNS.SOA.Refresh != nil {
			soa.Refresh = *c.DNS.SOA.Refresh
		}
		if c.DNS.SOA.Retry != nil {
			soa.Retry = *c.DNS.SOA.Retry
		}
	}

	leaveOnTerm := !boolVal(c.ServerMode)
	if c.LeaveOnTerm != nil {
		leaveOnTerm = boolVal(c.LeaveOnTerm)
	}

	skipLeaveOnInt := boolVal(c.ServerMode)
	if c.SkipLeaveOnInt != nil {
		skipLeaveOnInt = boolVal(c.SkipLeaveOnInt)
	}

	// ----------------------------------------------------------------
	// checks and services
	//

	var checks []*structs.CheckDefinition
	if c.Check != nil {
		checks = append(checks, b.checkVal(c.Check))
	}
	for _, check := range c.Checks {
		checks = append(checks, b.checkVal(&check))
	}

	var services []*structs.ServiceDefinition
	for _, service := range c.Services {
		services = append(services, b.serviceVal(&service))
	}
	if c.Service != nil {
		services = append(services, b.serviceVal(c.Service))
	}

	// ----------------------------------------------------------------
	// addresses
	//

	// determine port values and replace values <= 0 and > 65535 with -1
	dnsPort := b.portVal("ports.dns", c.Ports.DNS)
	httpPort := b.portVal("ports.http", c.Ports.HTTP)
	httpsPort := b.portVal("ports.https", c.Ports.HTTPS)
	serverPort := b.portVal("ports.server", c.Ports.Server)
	grpcPort := b.portVal("ports.grpc", c.Ports.GRPC)
	grpcTlsPort := b.portVal("ports.grpc_tls", c.Ports.GRPCTLS)
	// default gRPC TLS port for servers is 8503
	if c.Ports.GRPCTLS == nil && boolVal(c.ServerMode) {
		grpcTlsPort = 8503
	}
	serfPortLAN := b.portVal("ports.serf_lan", c.Ports.SerfLAN)
	serfPortWAN := b.portVal("ports.serf_wan", c.Ports.SerfWAN)
	proxyMinPort := b.portVal("ports.proxy_min_port", c.Ports.ProxyMinPort)
	proxyMaxPort := b.portVal("ports.proxy_max_port", c.Ports.ProxyMaxPort)
	sidecarMinPort := b.portVal("ports.sidecar_min_port", c.Ports.SidecarMinPort)
	sidecarMaxPort := b.portVal("ports.sidecar_max_port", c.Ports.SidecarMaxPort)
	exposeMinPort := b.portVal("ports.expose_min_port", c.Ports.ExposeMinPort)
	exposeMaxPort := b.portVal("ports.expose_max_port", c.Ports.ExposeMaxPort)
	if serverPort <= 0 {
		return RuntimeConfig{}, fmt.Errorf(
			"server-port must be greater than zero")
	}
	if serfPortLAN <= 0 {
		return RuntimeConfig{}, fmt.Errorf(
			"serf-lan-port must be greater than zero")
	}
	if proxyMaxPort < proxyMinPort {
		return RuntimeConfig{}, fmt.Errorf(
			"proxy_min_port must be less than proxy_max_port. To disable, set both to zero.")
	}
	if sidecarMaxPort < sidecarMinPort {
		return RuntimeConfig{}, fmt.Errorf(
			"sidecar_min_port must be less than sidecar_max_port. To disable, set both to zero.")
	}
	if exposeMaxPort < exposeMinPort {
		return RuntimeConfig{}, fmt.Errorf(
			"expose_min_port must be less than expose_max_port. To disable, set both to zero.")
	}

	// determine the default bind and advertise address
	//
	// First check whether the user provided an ANY address or whether
	// the expanded template results in an ANY address. In that case we
	// derive an advertise address from the current network
	// configuration since we can listen on an ANY address for incoming
	// traffic but cannot advertise it as the address on which the
	// server can be reached.

	bindAddrs := b.expandAddrs("bind_addr", c.BindAddr)
	if len(bindAddrs) == 0 {
		return RuntimeConfig{}, fmt.Errorf("bind_addr cannot be empty")
	}
	if len(bindAddrs) > 1 {
		return RuntimeConfig{}, fmt.Errorf("bind_addr cannot contain multiple addresses. Use 'addresses.{dns,http,https}' instead.")
	}
	if isUnixAddr(bindAddrs[0]) {
		return RuntimeConfig{}, fmt.Errorf("bind_addr cannot be a unix socket")
	}
	if !isIPAddr(bindAddrs[0]) {
		return RuntimeConfig{}, fmt.Errorf("bind_addr must be an ip address")
	}
	if ipaddr.IsAny(stringVal(c.AdvertiseAddrLAN)) {
		return RuntimeConfig{}, fmt.Errorf("Advertise address cannot be 0.0.0.0, :: or [::]")
	}
	if ipaddr.IsAny(stringVal(c.AdvertiseAddrWAN)) {
		return RuntimeConfig{}, fmt.Errorf("Advertise WAN address cannot be 0.0.0.0, :: or [::]")
	}

	bindAddr := bindAddrs[0].(*net.IPAddr)
	advertiseAddr := makeIPAddr(b.expandFirstIP("advertise_addr", c.AdvertiseAddrLAN), bindAddr)

	if ipaddr.IsAny(advertiseAddr) {
		addrtyp, detect := advertiseAddrFunc(b.opts, advertiseAddr)
		advertiseAddrs, err := detect()
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("Error detecting %s address: %s", addrtyp, err)
		}
		if len(advertiseAddrs) == 0 {
			return RuntimeConfig{}, fmt.Errorf("No %s address found", addrtyp)
		}
		if len(advertiseAddrs) > 1 {
			return RuntimeConfig{}, fmt.Errorf("Multiple %s addresses found. Please configure one with 'bind' and/or 'advertise'.", addrtyp)
		}
		advertiseAddr = advertiseAddrs[0]
	}

	// derive other bind addresses from the bindAddr
	rpcBindAddr := b.makeTCPAddr(bindAddr, nil, serverPort)
	serfBindAddrLAN := b.makeTCPAddr(b.expandFirstIP("serf_lan", c.SerfBindAddrLAN), bindAddr, serfPortLAN)

	// Only initialize serf WAN bind address when its enabled
	var serfBindAddrWAN *net.TCPAddr
	if serfPortWAN >= 0 {
		serfBindAddrWAN = b.makeTCPAddr(b.expandFirstIP("serf_wan", c.SerfBindAddrWAN), bindAddr, serfPortWAN)
	}

	// derive other advertise addresses from the advertise address
	advertiseAddrLAN := makeIPAddr(b.expandFirstIP("advertise_addr", c.AdvertiseAddrLAN), advertiseAddr)
	advertiseAddrIsV6 := advertiseAddr.IP.To4() == nil
	var advertiseAddrV4, advertiseAddrV6 *net.IPAddr
	if !advertiseAddrIsV6 {
		advertiseAddrV4 = advertiseAddr
	} else {
		advertiseAddrV6 = advertiseAddr
	}
	advertiseAddrLANIPv4 := makeIPAddr(b.expandFirstIP("advertise_addr_ipv4", c.AdvertiseAddrLANIPv4), advertiseAddrV4)
	if advertiseAddrLANIPv4 != nil && advertiseAddrLANIPv4.IP.To4() == nil {
		return RuntimeConfig{}, fmt.Errorf("advertise_addr_ipv4 must be an ipv4 address")
	}
	advertiseAddrLANIPv6 := makeIPAddr(b.expandFirstIP("advertise_addr_ipv6", c.AdvertiseAddrLANIPv6), advertiseAddrV6)
	if advertiseAddrLANIPv6 != nil && advertiseAddrLANIPv6.IP.To4() != nil {
		return RuntimeConfig{}, fmt.Errorf("advertise_addr_ipv6 must be an ipv6 address")
	}

	advertiseAddrWAN := makeIPAddr(b.expandFirstIP("advertise_addr_wan", c.AdvertiseAddrWAN), advertiseAddrLAN)
	advertiseAddrWANIsV6 := advertiseAddrWAN.IP.To4() == nil
	var advertiseAddrWANv4, advertiseAddrWANv6 *net.IPAddr
	if !advertiseAddrWANIsV6 {
		advertiseAddrWANv4 = advertiseAddrWAN
	} else {
		advertiseAddrWANv6 = advertiseAddrWAN
	}
	advertiseAddrWANIPv4 := makeIPAddr(b.expandFirstIP("advertise_addr_wan_ipv4", c.AdvertiseAddrWANIPv4), advertiseAddrWANv4)
	if advertiseAddrWANIPv4 != nil && advertiseAddrWANIPv4.IP.To4() == nil {
		return RuntimeConfig{}, fmt.Errorf("advertise_addr_wan_ipv4 must be an ipv4 address")
	}
	advertiseAddrWANIPv6 := makeIPAddr(b.expandFirstIP("advertise_addr_wan_ipv6", c.AdvertiseAddrWANIPv6), advertiseAddrWANv6)
	if advertiseAddrWANIPv6 != nil && advertiseAddrWANIPv6.IP.To4() != nil {
		return RuntimeConfig{}, fmt.Errorf("advertise_addr_wan_ipv6 must be an ipv6 address")
	}

	rpcAdvertiseAddr := &net.TCPAddr{IP: advertiseAddrLAN.IP, Port: serverPort}
	serfAdvertiseAddrLAN := &net.TCPAddr{IP: advertiseAddrLAN.IP, Port: serfPortLAN}
	// Only initialize serf WAN advertise address when its enabled
	var serfAdvertiseAddrWAN *net.TCPAddr
	if serfPortWAN >= 0 {
		serfAdvertiseAddrWAN = &net.TCPAddr{IP: advertiseAddrWAN.IP, Port: serfPortWAN}
	}

	// determine client addresses
	clientAddrs := b.expandIPs("client_addr", c.ClientAddr)
	if len(clientAddrs) == 0 {
		b.warn("client_addr is empty, client services (DNS, HTTP, HTTPS, GRPC) will not be listening for connections")
	}
	dnsAddrs := b.makeAddrs(b.expandAddrs("addresses.dns", c.Addresses.DNS), clientAddrs, dnsPort)
	httpAddrs := b.makeAddrs(b.expandAddrs("addresses.http", c.Addresses.HTTP), clientAddrs, httpPort)
	httpsAddrs := b.makeAddrs(b.expandAddrs("addresses.https", c.Addresses.HTTPS), clientAddrs, httpsPort)
	grpcAddrs := b.makeAddrs(b.expandAddrs("addresses.grpc", c.Addresses.GRPC), clientAddrs, grpcPort)
	grpcTlsAddrs := b.makeAddrs(b.expandAddrs("addresses.grpc_tls", c.Addresses.GRPCTLS), clientAddrs, grpcTlsPort)

	for _, a := range dnsAddrs {
		if x, ok := a.(*net.TCPAddr); ok {
			dnsAddrs = append(dnsAddrs, &net.UDPAddr{IP: x.IP, Port: x.Port})
		}
	}

	// expand dns recursors
	uniq := map[string]bool{}
	dnsRecursors := []string{}
	for _, r := range c.DNSRecursors {
		x, err := template.Parse(r)
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("Invalid DNS recursor template %q: %s", r, err)
		}
		for _, addr := range strings.Fields(x) {
			if strings.HasPrefix(addr, "unix://") {
				return RuntimeConfig{}, fmt.Errorf("DNS Recursors cannot be unix sockets: %s", addr)
			}
			if uniq[addr] {
				continue
			}
			uniq[addr] = true
			dnsRecursors = append(dnsRecursors, addr)
		}
	}

	datacenter := strings.ToLower(stringVal(c.Datacenter))
	altDomain := stringVal(c.DNSAltDomain)

	// Create the default set of tagged addresses.
	if c.TaggedAddresses == nil {
		c.TaggedAddresses = make(map[string]string)
	}

	c.TaggedAddresses[structs.TaggedAddressLAN] = advertiseAddrLAN.IP.String()
	if advertiseAddrLANIPv4 != nil {
		c.TaggedAddresses[structs.TaggedAddressLANIPv4] = advertiseAddrLANIPv4.IP.String()
	}
	if advertiseAddrLANIPv6 != nil {
		c.TaggedAddresses[structs.TaggedAddressLANIPv6] = advertiseAddrLANIPv6.IP.String()
	}

	c.TaggedAddresses[structs.TaggedAddressWAN] = advertiseAddrWAN.IP.String()
	if advertiseAddrWANIPv4 != nil {
		c.TaggedAddresses[structs.TaggedAddressWANIPv4] = advertiseAddrWANIPv4.IP.String()
	}
	if advertiseAddrWANIPv6 != nil {
		c.TaggedAddresses[structs.TaggedAddressWANIPv6] = advertiseAddrWANIPv6.IP.String()
	}

	// segments
	var segments []structs.NetworkSegment
	for _, s := range c.Segments {
		name := stringVal(s.Name)
		port := b.portVal(fmt.Sprintf("segments[%s].port", name), s.Port)
		if port <= 0 {
			return RuntimeConfig{}, fmt.Errorf("Port for segment %q cannot be <= 0", name)
		}

		bind := b.makeTCPAddr(
			b.expandFirstIP(fmt.Sprintf("segments[%s].bind", name), s.Bind),
			bindAddr,
			port,
		)

		advertise := b.makeTCPAddr(
			b.expandFirstIP(fmt.Sprintf("segments[%s].advertise", name), s.Advertise),
			advertiseAddrLAN,
			port,
		)

		segments = append(segments, structs.NetworkSegment{
			Name:        name,
			Bind:        bind,
			Advertise:   advertise,
			RPCListener: boolVal(s.RPCListener),
		})
	}

	// Parse the metric filters
	telemetryAllowedPrefixes, telemetryBlockedPrefixes := b.parsePrefixFilter(&c.Telemetry)

	// raft performance scaling
	performanceRaftMultiplier := intVal(c.Performance.RaftMultiplier)
	if performanceRaftMultiplier < 1 || uint(performanceRaftMultiplier) > consul.MaxRaftMultiplier {
		return RuntimeConfig{}, fmt.Errorf("performance.raft_multiplier cannot be %d. Must be between 1 and %d", performanceRaftMultiplier, consul.MaxRaftMultiplier)
	}
	consulRaftElectionTimeout := b.durationVal("consul.raft.election_timeout", c.Consul.Raft.ElectionTimeout) * time.Duration(performanceRaftMultiplier)
	consulRaftHeartbeatTimeout := b.durationVal("consul.raft.heartbeat_timeout", c.Consul.Raft.HeartbeatTimeout) * time.Duration(performanceRaftMultiplier)
	consulRaftLeaderLeaseTimeout := b.durationVal("consul.raft.leader_lease_timeout", c.Consul.Raft.LeaderLeaseTimeout) * time.Duration(performanceRaftMultiplier)

	// Connect
	connectEnabled := boolVal(c.Connect.Enabled)
	connectCAProvider := stringVal(c.Connect.CAProvider)
	connectCAConfig := c.Connect.CAConfig

	// autoEncrypt and autoConfig implicitly turns on connect which is why
	// they need to be above other settings that rely on connect.
	autoEncryptDNSSAN := []string{}
	for _, d := range c.AutoEncrypt.DNSSAN {
		autoEncryptDNSSAN = append(autoEncryptDNSSAN, d)
	}
	autoEncryptIPSAN := []net.IP{}
	for _, i := range c.AutoEncrypt.IPSAN {
		ip := net.ParseIP(i)
		if ip == nil {
			b.warn(fmt.Sprintf("Cannot parse ip %q from AutoEncrypt.IPSAN", i))
			continue
		}
		autoEncryptIPSAN = append(autoEncryptIPSAN, ip)

	}
	autoEncryptAllowTLS := boolVal(c.AutoEncrypt.AllowTLS)
	autoConfig := b.autoConfigVal(c.AutoConfig, stringVal(c.Partition))
	if autoEncryptAllowTLS || autoConfig.Enabled {
		connectEnabled = true
	}

	// Connect proxy defaults
	connectMeshGatewayWANFederationEnabled := boolVal(c.Connect.MeshGatewayWANFederationEnabled)
	if connectMeshGatewayWANFederationEnabled && !connectEnabled {
		return RuntimeConfig{}, fmt.Errorf("'connect.enable_mesh_gateway_wan_federation=true' requires 'connect.enabled=true'")
	}
	if connectCAConfig != nil {
		// nolint: staticcheck // CA config should be changed to use HookTranslateKeys
		lib.TranslateKeys(connectCAConfig, map[string]string{
			// Consul CA config
			"private_key":           "PrivateKey",
			"root_cert":             "RootCert",
			"intermediate_cert_ttl": "IntermediateCertTTL",

			// Vault CA config
			"address":                    "Address",
			"token":                      "Token",
			"root_pki_path":              "RootPKIPath",
			"root_pki_namespace":         "RootPKINamespace",
			"intermediate_pki_path":      "IntermediatePKIPath",
			"intermediate_pki_namespace": "IntermediatePKINamespace",
			"ca_file":                    "CAFile",
			"ca_path":                    "CAPath",
			"cert_file":                  "CertFile",
			"key_file":                   "KeyFile",
			"tls_server_name":            "TLSServerName",
			"tls_skip_verify":            "TLSSkipVerify",

			// AWS CA config
			"existing_arn":   "ExistingARN",
			"delete_on_exit": "DeleteOnExit",

			// Common CA config
			"leaf_cert_ttl":      "LeafCertTTL",
			"csr_max_per_second": "CSRMaxPerSecond",
			"csr_max_concurrent": "CSRMaxConcurrent",
			"private_key_type":   "PrivateKeyType",
			"private_key_bits":   "PrivateKeyBits",
			"root_cert_ttl":      "RootCertTTL",
		})
	}

	aclsEnabled := false
	primaryDatacenter := strings.ToLower(stringVal(c.PrimaryDatacenter))

	if c.ACL.Enabled != nil {
		aclsEnabled = boolVal(c.ACL.Enabled)
	}

	// Set the primary DC if it wasn't set.
	if primaryDatacenter == "" {
		primaryDatacenter = datacenter
	}

	enableRemoteScriptChecks := boolVal(c.EnableScriptChecks)
	enableLocalScriptChecks := boolValWithDefault(c.EnableLocalScriptChecks, enableRemoteScriptChecks)

	var configEntries []structs.ConfigEntry

	if len(c.ConfigEntries.Bootstrap) > 0 {
		for i, rawEntry := range c.ConfigEntries.Bootstrap {
			entry, err := structs.DecodeConfigEntry(rawEntry)
			if err != nil {
				return RuntimeConfig{}, fmt.Errorf("config_entries.bootstrap[%d]: %s", i, err)
			}
			if err := entry.Normalize(); err != nil {
				return RuntimeConfig{}, fmt.Errorf("config_entries.bootstrap[%d]: %s", i, err)
			}
			if err := entry.Validate(); err != nil {
				return RuntimeConfig{}, fmt.Errorf("config_entries.bootstrap[%d]: %w", i, err)
			}
			configEntries = append(configEntries, entry)
		}
	}

	serfAllowedCIDRSLAN, err := memberlist.ParseCIDRs(c.SerfAllowedCIDRsLAN)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("serf_lan_allowed_cidrs: %s", err)
	}
	serfAllowedCIDRSWAN, err := memberlist.ParseCIDRs(c.SerfAllowedCIDRsWAN)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("serf_wan_allowed_cidrs: %s", err)
	}

	// Handle Deprecated UI config fields
	if c.UI != nil {
		b.warn("The 'ui' field is deprecated. Use the 'ui_config.enabled' field instead.")
		if c.UIConfig.Enabled == nil {
			c.UIConfig.Enabled = c.UI
		}
	}
	if c.UIDir != nil {
		b.warn("The 'ui_dir' field is deprecated. Use the 'ui_config.dir' field instead.")
		if c.UIConfig.Dir == nil {
			c.UIConfig.Dir = c.UIDir
		}
	}
	if c.UIContentPath != nil {
		b.warn("The 'ui_content_path' field is deprecated. Use the 'ui_config.content_path' field instead.")
		if c.UIConfig.ContentPath == nil {
			c.UIConfig.ContentPath = c.UIContentPath
		}
	}

	serverMode := boolVal(c.ServerMode)

	// ----------------------------------------------------------------
	// build runtime config
	//
	dataDir := stringVal(c.DataDir)
	rt = RuntimeConfig{
		// non-user configurable values
		AEInterval:                 b.durationVal("ae_interval", c.AEInterval),
		CheckDeregisterIntervalMin: b.durationVal("check_deregister_interval_min", c.CheckDeregisterIntervalMin),
		CheckReapInterval:          b.durationVal("check_reap_interval", c.CheckReapInterval),
		Revision:                   stringVal(c.Revision),
		SegmentNameLimit:           intVal(c.SegmentNameLimit),
		SyncCoordinateIntervalMin:  b.durationVal("sync_coordinate_interval_min", c.SyncCoordinateIntervalMin),
		SyncCoordinateRateTarget:   float64Val(c.SyncCoordinateRateTarget),
		Version:                    stringVal(c.Version),
		VersionPrerelease:          stringVal(c.VersionPrerelease),
		VersionMetadata:            stringVal(c.VersionMetadata),
		// What is a sensible default for BuildDate?
		BuildDate: timeValWithDefault(c.BuildDate, time.Date(1970, 1, 00, 00, 00, 01, 0, time.UTC)),

		// consul configuration
		ConsulCoordinateUpdateBatchSize:  intVal(c.Consul.Coordinate.UpdateBatchSize),
		ConsulCoordinateUpdateMaxBatches: intVal(c.Consul.Coordinate.UpdateMaxBatches),
		ConsulCoordinateUpdatePeriod:     b.durationVal("consul.coordinate.update_period", c.Consul.Coordinate.UpdatePeriod),
		ConsulRaftElectionTimeout:        consulRaftElectionTimeout,
		ConsulRaftHeartbeatTimeout:       consulRaftHeartbeatTimeout,
		ConsulRaftLeaderLeaseTimeout:     consulRaftLeaderLeaseTimeout,
		ConsulServerHealthInterval:       b.durationVal("consul.server.health_interval", c.Consul.Server.HealthInterval),

		// gossip configuration
		GossipLANGossipInterval: b.durationVal("gossip_lan..gossip_interval", c.GossipLAN.GossipInterval),
		GossipLANGossipNodes:    intVal(c.GossipLAN.GossipNodes),
		Locality:                c.Locality,
		GossipLANProbeInterval:  b.durationVal("gossip_lan..probe_interval", c.GossipLAN.ProbeInterval),
		GossipLANProbeTimeout:   b.durationVal("gossip_lan..probe_timeout", c.GossipLAN.ProbeTimeout),
		GossipLANSuspicionMult:  intVal(c.GossipLAN.SuspicionMult),
		GossipLANRetransmitMult: intVal(c.GossipLAN.RetransmitMult),
		GossipWANGossipInterval: b.durationVal("gossip_wan..gossip_interval", c.GossipWAN.GossipInterval),
		GossipWANGossipNodes:    intVal(c.GossipWAN.GossipNodes),
		GossipWANProbeInterval:  b.durationVal("gossip_wan..probe_interval", c.GossipWAN.ProbeInterval),
		GossipWANProbeTimeout:   b.durationVal("gossip_wan..probe_timeout", c.GossipWAN.ProbeTimeout),
		GossipWANSuspicionMult:  intVal(c.GossipWAN.SuspicionMult),
		GossipWANRetransmitMult: intVal(c.GossipWAN.RetransmitMult),

		// ACL
		ACLsEnabled: aclsEnabled,
		ACLResolverSettings: consul.ACLResolverSettings{
			ACLsEnabled:      aclsEnabled,
			Datacenter:       datacenter,
			NodeName:         b.nodeName(c.NodeName),
			ACLPolicyTTL:     b.durationVal("acl.policy_ttl", c.ACL.PolicyTTL),
			ACLTokenTTL:      b.durationVal("acl.token_ttl", c.ACL.TokenTTL),
			ACLRoleTTL:       b.durationVal("acl.role_ttl", c.ACL.RoleTTL),
			ACLDownPolicy:    stringVal(c.ACL.DownPolicy),
			ACLDefaultPolicy: stringVal(c.ACL.DefaultPolicy),
		},

		ACLEnableKeyListPolicy:    boolVal(c.ACL.EnableKeyListPolicy),
		ACLInitialManagementToken: stringVal(c.ACL.Tokens.InitialManagement),

		ACLTokenReplication: boolVal(c.ACL.TokenReplication),

		ACLTokens: token.Config{
			DataDir:                        dataDir,
			EnablePersistence:              boolValWithDefault(c.ACL.EnableTokenPersistence, false),
			ACLDefaultToken:                stringVal(c.ACL.Tokens.Default),
			ACLAgentToken:                  stringVal(c.ACL.Tokens.Agent),
			ACLAgentRecoveryToken:          stringVal(c.ACL.Tokens.AgentRecovery),
			ACLReplicationToken:            stringVal(c.ACL.Tokens.Replication),
			ACLConfigFileRegistrationToken: stringVal(c.ACL.Tokens.ConfigFileRegistration),
		},

		// Autopilot
		AutopilotCleanupDeadServers:      boolVal(c.Autopilot.CleanupDeadServers),
		AutopilotDisableUpgradeMigration: boolVal(c.Autopilot.DisableUpgradeMigration),
		AutopilotLastContactThreshold:    b.durationVal("autopilot.last_contact_threshold", c.Autopilot.LastContactThreshold),
		AutopilotMaxTrailingLogs:         intVal(c.Autopilot.MaxTrailingLogs),
		AutopilotMinQuorum:               uintVal(c.Autopilot.MinQuorum),
		AutopilotRedundancyZoneTag:       stringVal(c.Autopilot.RedundancyZoneTag),
		AutopilotServerStabilizationTime: b.durationVal("autopilot.server_stabilization_time", c.Autopilot.ServerStabilizationTime),
		AutopilotUpgradeVersionTag:       stringVal(c.Autopilot.UpgradeVersionTag),

		// DNS
		DNSAddrs:              dnsAddrs,
		DNSAllowStale:         boolVal(c.DNS.AllowStale),
		DNSARecordLimit:       intVal(c.DNS.ARecordLimit),
		DNSDisableCompression: boolVal(c.DNS.DisableCompression),
		DNSDomain:             stringVal(c.DNSDomain),
		DNSAltDomain:          altDomain,
		DNSEnableTruncate:     boolVal(c.DNS.EnableTruncate),
		DNSMaxStale:           b.durationVal("dns_config.max_stale", c.DNS.MaxStale),
		DNSNodeTTL:            b.durationVal("dns_config.node_ttl", c.DNS.NodeTTL),
		DNSOnlyPassing:        boolVal(c.DNS.OnlyPassing),
		DNSPort:               dnsPort,
		DNSRecursorStrategy:   b.dnsRecursorStrategyVal(stringVal(c.DNS.RecursorStrategy)),
		DNSRecursorTimeout:    b.durationVal("recursor_timeout", c.DNS.RecursorTimeout),
		DNSRecursors:          dnsRecursors,
		DNSServiceTTL:         dnsServiceTTL,
		DNSSOA:                soa,
		DNSUDPAnswerLimit:     intVal(c.DNS.UDPAnswerLimit),
		DNSNodeMetaTXT:        boolValWithDefault(c.DNS.NodeMetaTXT, true),
		DNSUseCache:           boolVal(c.DNS.UseCache),
		DNSCacheMaxAge:        b.durationVal("dns_config.cache_max_age", c.DNS.CacheMaxAge),

		// HTTP
		HTTPPort:            httpPort,
		HTTPSPort:           httpsPort,
		HTTPAddrs:           httpAddrs,
		HTTPSAddrs:          httpsAddrs,
		HTTPBlockEndpoints:  c.HTTPConfig.BlockEndpoints,
		HTTPMaxHeaderBytes:  intVal(c.HTTPConfig.MaxHeaderBytes),
		HTTPResponseHeaders: c.HTTPConfig.ResponseHeaders,
		AllowWriteHTTPFrom:  b.cidrsVal("allow_write_http_from", c.HTTPConfig.AllowWriteHTTPFrom),
		HTTPUseCache:        boolValWithDefault(c.HTTPConfig.UseCache, true),

		// Telemetry
		Telemetry: lib.TelemetryConfig{
			CirconusAPIApp:                     stringVal(c.Telemetry.CirconusAPIApp),
			CirconusAPIToken:                   stringVal(c.Telemetry.CirconusAPIToken),
			CirconusAPIURL:                     stringVal(c.Telemetry.CirconusAPIURL),
			CirconusBrokerID:                   stringVal(c.Telemetry.CirconusBrokerID),
			CirconusBrokerSelectTag:            stringVal(c.Telemetry.CirconusBrokerSelectTag),
			CirconusCheckDisplayName:           stringVal(c.Telemetry.CirconusCheckDisplayName),
			CirconusCheckForceMetricActivation: stringVal(c.Telemetry.CirconusCheckForceMetricActivation),
			CirconusCheckID:                    stringVal(c.Telemetry.CirconusCheckID),
			CirconusCheckInstanceID:            stringVal(c.Telemetry.CirconusCheckInstanceID),
			CirconusCheckSearchTag:             stringVal(c.Telemetry.CirconusCheckSearchTag),
			CirconusCheckTags:                  stringVal(c.Telemetry.CirconusCheckTags),
			CirconusSubmissionInterval:         stringVal(c.Telemetry.CirconusSubmissionInterval),
			CirconusSubmissionURL:              stringVal(c.Telemetry.CirconusSubmissionURL),
			DisableHostname:                    boolVal(c.Telemetry.DisableHostname),
			DogstatsdAddr:                      stringVal(c.Telemetry.DogstatsdAddr),
			DogstatsdTags:                      c.Telemetry.DogstatsdTags,
			RetryFailedConfiguration:           boolVal(c.Telemetry.RetryFailedConfiguration),
			FilterDefault:                      boolVal(c.Telemetry.FilterDefault),
			AllowedPrefixes:                    telemetryAllowedPrefixes,
			BlockedPrefixes:                    telemetryBlockedPrefixes,
			MetricsPrefix:                      stringVal(c.Telemetry.MetricsPrefix),
			StatsdAddr:                         stringVal(c.Telemetry.StatsdAddr),
			StatsiteAddr:                       stringVal(c.Telemetry.StatsiteAddr),
			PrometheusOpts: prometheus.PrometheusOpts{
				Expiration: b.durationVal("prometheus_retention_time", c.Telemetry.PrometheusRetentionTime),
				Name:       stringVal(c.Telemetry.MetricsPrefix),
			},
		},

		// Agent
		AdvertiseAddrLAN:          advertiseAddrLAN,
		AdvertiseAddrWAN:          advertiseAddrWAN,
		AdvertiseReconnectTimeout: b.durationVal("advertise_reconnect_timeout", c.AdvertiseReconnectTimeout),
		BindAddr:                  bindAddr,
		Bootstrap:                 boolVal(c.Bootstrap),
		BootstrapExpect:           intVal(c.BootstrapExpect),
		Cache: cache.Options{
			EntryFetchRate: limitValWithDefault(
				c.Cache.EntryFetchRate, float64(cache.DefaultEntryFetchRate),
			),
			EntryFetchMaxBurst: intValWithDefault(
				c.Cache.EntryFetchMaxBurst, cache.DefaultEntryFetchMaxBurst,
			),
		},
		AutoReloadConfig:                       boolVal(c.AutoReloadConfig),
		CheckUpdateInterval:                    b.durationVal("check_update_interval", c.CheckUpdateInterval),
		CheckOutputMaxSize:                     intValWithDefault(c.CheckOutputMaxSize, 4096),
		Checks:                                 checks,
		ClientAddrs:                            clientAddrs,
		ConfigEntryBootstrap:                   configEntries,
		AutoEncryptTLS:                         boolVal(c.AutoEncrypt.TLS),
		AutoEncryptDNSSAN:                      autoEncryptDNSSAN,
		AutoEncryptIPSAN:                       autoEncryptIPSAN,
		AutoEncryptAllowTLS:                    autoEncryptAllowTLS,
		AutoConfig:                             autoConfig,
		Cloud:                                  b.cloudConfigVal(c.Cloud),
		ConnectEnabled:                         connectEnabled,
		ConnectCAProvider:                      connectCAProvider,
		ConnectCAConfig:                        connectCAConfig,
		ConnectMeshGatewayWANFederationEnabled: connectMeshGatewayWANFederationEnabled,
		ConnectSidecarMinPort:                  sidecarMinPort,
		ConnectSidecarMaxPort:                  sidecarMaxPort,
		ConnectTestCALeafRootChangeSpread:      b.durationVal("connect.test_ca_leaf_root_change_spread", c.Connect.TestCALeafRootChangeSpread),
		ExposeMinPort:                          exposeMinPort,
		ExposeMaxPort:                          exposeMaxPort,
		DataDir:                                dataDir,
		Datacenter:                             datacenter,
		DefaultQueryTime:                       b.durationVal("default_query_time", c.DefaultQueryTime),
		DevMode:                                boolVal(b.opts.DevMode),
		DisableAnonymousSignature:              boolVal(c.DisableAnonymousSignature),
		DisableCoordinates:                     boolVal(c.DisableCoordinates),
		DisableHostNodeID:                      boolVal(c.DisableHostNodeID),
		DisableHTTPUnprintableCharFilter:       boolVal(c.DisableHTTPUnprintableCharFilter),
		DisableKeyringFile:                     boolVal(c.DisableKeyringFile),
		DisableRemoteExec:                      boolVal(c.DisableRemoteExec),
		DisableUpdateCheck:                     boolVal(c.DisableUpdateCheck),
		DiscardCheckOutput:                     boolVal(c.DiscardCheckOutput),

		DiscoveryMaxStale:          b.durationVal("discovery_max_stale", c.DiscoveryMaxStale),
		EnableAgentTLSForChecks:    boolVal(c.EnableAgentTLSForChecks),
		EnableCentralServiceConfig: boolVal(c.EnableCentralServiceConfig),
		EnableDebug:                boolVal(c.EnableDebug),
		EnableRemoteScriptChecks:   enableRemoteScriptChecks,
		EnableLocalScriptChecks:    enableLocalScriptChecks,
		EncryptKey:                 stringVal(c.EncryptKey),
		GRPCAddrs:                  grpcAddrs,
		GRPCPort:                   grpcPort,
		GRPCTLSAddrs:               grpcTlsAddrs,
		GRPCTLSPort:                grpcTlsPort,
		HTTPMaxConnsPerClient:      intVal(c.Limits.HTTPMaxConnsPerClient),
		HTTPSHandshakeTimeout:      b.durationVal("limits.https_handshake_timeout", c.Limits.HTTPSHandshakeTimeout),
		KVMaxValueSize:             uint64Val(c.Limits.KVMaxValueSize),
		LeaveDrainTime:             b.durationVal("performance.leave_drain_time", c.Performance.LeaveDrainTime),
		LeaveOnTerm:                leaveOnTerm,
		StaticRuntimeConfig: StaticRuntimeConfig{
			EncryptVerifyIncoming: boolVal(c.EncryptVerifyIncoming),
			EncryptVerifyOutgoing: boolVal(c.EncryptVerifyOutgoing),
		},

		Logging: logging.Config{
			LogLevel:          stringVal(c.LogLevel),
			LogJSON:           boolVal(c.LogJSON),
			LogFilePath:       stringVal(c.LogFile),
			EnableSyslog:      boolVal(c.EnableSyslog),
			SyslogFacility:    stringVal(c.SyslogFacility),
			LogRotateDuration: b.durationVal("log_rotate_duration", c.LogRotateDuration),
			LogRotateBytes:    intVal(c.LogRotateBytes),
			LogRotateMaxFiles: intVal(c.LogRotateMaxFiles),
		},
		MaxQueryTime:                      b.durationVal("max_query_time", c.MaxQueryTime),
		NodeID:                            types.NodeID(stringVal(c.NodeID)),
		NodeMeta:                          c.NodeMeta,
		NodeName:                          b.nodeName(c.NodeName),
		ReadReplica:                       boolVal(c.ReadReplica),
		PeeringEnabled:                    boolVal(c.Peering.Enabled),
		PeeringTestAllowPeerRegistrations: boolValWithDefault(c.Peering.TestAllowPeerRegistrations, false),
		PidFile:                           stringVal(c.PidFile),
		PrimaryDatacenter:                 primaryDatacenter,
		PrimaryGateways:                   b.expandAllOptionalAddrs("primary_gateways", c.PrimaryGateways),
		PrimaryGatewaysInterval:           b.durationVal("primary_gateways_interval", c.PrimaryGatewaysInterval),
		RPCAdvertiseAddr:                  rpcAdvertiseAddr,
		RPCBindAddr:                       rpcBindAddr,
		RPCHandshakeTimeout:               b.durationVal("limits.rpc_handshake_timeout", c.Limits.RPCHandshakeTimeout),
		RPCHoldTimeout:                    b.durationVal("performance.rpc_hold_timeout", c.Performance.RPCHoldTimeout),
		RPCClientTimeout:                  b.durationVal("limits.rpc_client_timeout", c.Limits.RPCClientTimeout),
		RPCMaxBurst:                       intVal(c.Limits.RPCMaxBurst),
		RPCMaxConnsPerClient:              intVal(c.Limits.RPCMaxConnsPerClient),
		RPCProtocol:                       intVal(c.RPCProtocol),
		RPCRateLimit:                      limitVal(c.Limits.RPCRate),
		RPCConfig:                         consul.RPCConfig{EnableStreaming: boolValWithDefault(c.RPC.EnableStreaming, serverMode)},
		RaftProtocol:                      intVal(c.RaftProtocol),
		RaftSnapshotThreshold:             intVal(c.RaftSnapshotThreshold),
		RaftSnapshotInterval:              b.durationVal("raft_snapshot_interval", c.RaftSnapshotInterval),
		RaftTrailingLogs:                  intVal(c.RaftTrailingLogs),
		RaftLogStoreConfig:                b.raftLogStoreConfigVal(&c.RaftLogStore),
		ReconnectTimeoutLAN:               b.durationVal("reconnect_timeout", c.ReconnectTimeoutLAN),
		ReconnectTimeoutWAN:               b.durationVal("reconnect_timeout_wan", c.ReconnectTimeoutWAN),
		RejoinAfterLeave:                  boolVal(c.RejoinAfterLeave),
		RequestLimitsMode:                 b.requestsLimitsModeVal(stringVal(c.Limits.RequestLimits.Mode)),
		RequestLimitsReadRate:             limitVal(c.Limits.RequestLimits.ReadRate),
		RequestLimitsWriteRate:            limitVal(c.Limits.RequestLimits.WriteRate),
		RetryJoinIntervalLAN:              b.durationVal("retry_interval", c.RetryJoinIntervalLAN),
		RetryJoinIntervalWAN:              b.durationVal("retry_interval_wan", c.RetryJoinIntervalWAN),
		RetryJoinLAN:                      b.expandAllOptionalAddrs("retry_join", c.RetryJoinLAN),
		RetryJoinMaxAttemptsLAN:           intVal(c.RetryJoinMaxAttemptsLAN),
		RetryJoinMaxAttemptsWAN:           intVal(c.RetryJoinMaxAttemptsWAN),
		RetryJoinWAN:                      b.expandAllOptionalAddrs("retry_join_wan", c.RetryJoinWAN),
		SegmentName:                       stringVal(c.SegmentName),
		Segments:                          segments,
		SegmentLimit:                      intVal(c.SegmentLimit),
		SerfAdvertiseAddrLAN:              serfAdvertiseAddrLAN,
		SerfAdvertiseAddrWAN:              serfAdvertiseAddrWAN,
		SerfAllowedCIDRsLAN:               serfAllowedCIDRSLAN,
		SerfAllowedCIDRsWAN:               serfAllowedCIDRSWAN,
		SerfBindAddrLAN:                   serfBindAddrLAN,
		SerfBindAddrWAN:                   serfBindAddrWAN,
		SerfPortLAN:                       serfPortLAN,
		SerfPortWAN:                       serfPortWAN,
		ServerMode:                        serverMode,
		ServerName:                        stringVal(c.ServerName),
		ServerPort:                        serverPort,
		ServerRejoinAgeMax:                b.durationValWithDefaultMin("server_rejoin_age_max", c.ServerRejoinAgeMax, 24*7*time.Hour, 6*time.Hour),
		Services:                          services,
		SessionTTLMin:                     b.durationVal("session_ttl_min", c.SessionTTLMin),
		SkipLeaveOnInt:                    skipLeaveOnInt,
		TaggedAddresses:                   c.TaggedAddresses,
		TranslateWANAddrs:                 boolVal(c.TranslateWANAddrs),
		TxnMaxReqLen:                      uint64Val(c.Limits.TxnMaxReqLen),
		UIConfig:                          b.uiConfigVal(c.UIConfig),
		UnixSocketGroup:                   stringVal(c.UnixSocket.Group),
		UnixSocketMode:                    stringVal(c.UnixSocket.Mode),
		UnixSocketUser:                    stringVal(c.UnixSocket.User),
		Watches:                           c.Watches,
		XDSUpdateRateLimit:                limitVal(c.XDS.UpdateMaxPerSecond),
		AutoReloadConfigCoalesceInterval:  1 * time.Second,
		LocalProxyConfigResyncInterval:    30 * time.Second,
	}

	rt.TLS, err = b.buildTLSConfig(rt, c.TLS)
	if err != nil {
		return RuntimeConfig{}, err
	}

	// `ports.grpc` previously supported TLS, but this was changed for Consul 1.14.
	// This check is done to warn users that a config change is mandatory.
	if rt.TLS.GRPC.CertFile != "" || (rt.TLS.AutoTLS && rt.TLS.GRPC.UseAutoCert) {
		// If only `ports.grpc` is enabled, and the gRPC TLS port is not explicitly defined by the user,
		// check the grpc TLS settings for incompatibilities.
		if rt.GRPCPort > 0 && c.Ports.GRPCTLS == nil {
			return RuntimeConfig{}, fmt.Errorf("the `ports.grpc` listener no longer supports TLS. Use `ports.grpc_tls` instead. This message is appearing because GRPC is configured to use TLS, but `ports.grpc_tls` is not defined")
		}
	}

	rt.UseStreamingBackend = boolValWithDefault(c.UseStreamingBackend, true)

	if rt.Cache.EntryFetchMaxBurst <= 0 {
		return RuntimeConfig{}, fmt.Errorf("cache.entry_fetch_max_burst must be strictly positive, was: %v", rt.Cache.EntryFetchMaxBurst)
	}
	if rt.Cache.EntryFetchRate <= 0 {
		return RuntimeConfig{}, fmt.Errorf("cache.entry_fetch_rate must be strictly positive, was: %v", rt.Cache.EntryFetchRate)
	}

	if rt.UIConfig.MetricsProvider == "prometheus" {
		// Handle defaulting for the built-in version of prometheus.
		if len(rt.UIConfig.MetricsProxy.PathAllowlist) == 0 {
			rt.UIConfig.MetricsProxy.PathAllowlist = []string{
				"/api/v1/query",
				"/api/v1/query_range",
			}
		}
	}

	if err := b.BuildEnterpriseRuntimeConfig(&rt, &c); err != nil {
		return rt, err
	}

	if rt.BootstrapExpect == 1 {
		rt.Bootstrap = true
		rt.BootstrapExpect = 0
		b.warn(`BootstrapExpect is set to 1; this is the same as Bootstrap mode.`)
	}

	return rt, nil
}

func advertiseAddrFunc(opts LoadOpts, advertiseAddr *net.IPAddr) (string, func() ([]*net.IPAddr, error)) {
	switch {
	case ipaddr.IsAnyV4(advertiseAddr):
		fn := opts.getPrivateIPv4
		if fn == nil {
			fn = ipaddr.GetPrivateIPv4
		}
		return "private IPv4", fn

	case ipaddr.IsAnyV6(advertiseAddr):
		fn := opts.getPublicIPv6
		if fn == nil {
			fn = ipaddr.GetPublicIPv6
		}
		return "public IPv6", fn

	default:
		panic("unsupported net.IPAddr Type")
	}
}

// reBasicName validates that a field contains only lower case alphanumerics,
// underscore and dash and is non-empty.
var reBasicName = regexp.MustCompile("^[a-z0-9_-]+$")

func validateBasicName(field, value string, allowEmpty bool) error {
	if value == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("%s cannot be empty", field)
	}
	if !reBasicName.MatchString(value) {
		return fmt.Errorf("%s can only contain lowercase alphanumeric, - or _ characters."+
			" received: %q", field, value)
	}
	return nil
}

// validate performs semantic validation of the runtime configuration.
func (b *builder) validate(rt RuntimeConfig) error {
	// validContentPath defines a regexp for a valid content path name.
	validContentPath := regexp.MustCompile(`^[A-Za-z0-9/_-]+$`)
	hasVersion := regexp.MustCompile(`^/v\d+/$`)
	// ----------------------------------------------------------------
	// check required params we cannot recover from first
	//

	if rt.RaftProtocol != 3 {
		return fmt.Errorf("raft_protocol version %d is not supported by this version of Consul", rt.RaftProtocol)
	}

	if err := validateBasicName("datacenter", rt.Datacenter, false); err != nil {
		return err
	}
	if rt.DataDir == "" && !rt.DevMode {
		return fmt.Errorf("data_dir cannot be empty")
	}

	if !validContentPath.MatchString(rt.UIConfig.ContentPath) {
		return fmt.Errorf("ui-content-path can only contain alphanumeric, -, _, or /. received: %q", rt.UIConfig.ContentPath)
	}

	if hasVersion.MatchString(rt.UIConfig.ContentPath) {
		return fmt.Errorf("ui-content-path cannot have 'v[0-9]'. received: %q", rt.UIConfig.ContentPath)
	}

	if err := validateBasicName("ui_config.metrics_provider", rt.UIConfig.MetricsProvider, true); err != nil {
		return err
	}
	if rt.UIConfig.MetricsProviderOptionsJSON != "" {
		// Attempt to parse the JSON to ensure it's valid, parsing into a map
		// ensures we get an object.
		var dummyMap map[string]interface{}
		err := json.Unmarshal([]byte(rt.UIConfig.MetricsProviderOptionsJSON), &dummyMap)
		if err != nil {
			return fmt.Errorf("ui_config.metrics_provider_options_json must be empty "+
				"or a string containing a valid JSON object. received: %q",
				rt.UIConfig.MetricsProviderOptionsJSON)
		}
	}
	if rt.UIConfig.MetricsProxy.BaseURL != "" {
		u, err := url.Parse(rt.UIConfig.MetricsProxy.BaseURL)
		if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
			return fmt.Errorf("ui_config.metrics_proxy.base_url must be a valid http"+
				" or https URL. received: %q",
				rt.UIConfig.MetricsProxy.BaseURL)
		}
	}
	for _, allowedPath := range rt.UIConfig.MetricsProxy.PathAllowlist {
		if err := validateAbsoluteURLPath(allowedPath); err != nil {
			return fmt.Errorf("ui_config.metrics_proxy.path_allowlist: %v", err)
		}
	}
	for k, v := range rt.UIConfig.DashboardURLTemplates {
		if err := validateBasicName("ui_config.dashboard_url_templates key names", k, false); err != nil {
			return err
		}
		u, err := url.Parse(v)
		if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
			return fmt.Errorf("ui_config.dashboard_url_templates values must be a"+
				" valid http or https URL. received: %q",
				rt.UIConfig.MetricsProxy.BaseURL)
		}
	}

	if !rt.DevMode {
		fi, err := os.Stat(rt.DataDir)
		switch {
		case err != nil && !os.IsNotExist(err):
			return fmt.Errorf("Error getting info on data_dir: %s", err)
		case err == nil && !fi.IsDir():
			return fmt.Errorf("data_dir %q is not a directory", rt.DataDir)
		}
	}

	switch {
	case rt.NodeName == "":
		return fmt.Errorf("node_name cannot be empty")
	case dns.InvalidNameRe.MatchString(rt.NodeName):
		b.warn("Node name %q will not be discoverable "+
			"via DNS due to invalid characters. Valid characters include "+
			"all alpha-numerics and dashes.", rt.NodeName)
	case consul.InvalidNodeName.MatchString(rt.NodeName):
		// todo(kyhavlov): Add stronger validation here for node names.
		b.warn("Found invalid characters in node name %q - whitespace and quotes "+
			"(', \", `) cannot be used with auto-config.", rt.NodeName)
	case len(rt.NodeName) > dns.MaxLabelLength:
		b.warn("Node name %q will not be discoverable "+
			"via DNS due to it being too long. Valid lengths are between "+
			"1 and 63 bytes.", rt.NodeName)
	}

	if ipaddr.IsAny(rt.AdvertiseAddrLAN.IP) {
		return fmt.Errorf("Advertise address cannot be 0.0.0.0, :: or [::]")
	}
	if ipaddr.IsAny(rt.AdvertiseAddrWAN.IP) {
		return fmt.Errorf("Advertise WAN address cannot be 0.0.0.0, :: or [::]")
	}
	if err := b.validateSegments(rt); err != nil {
		return err
	}
	for _, a := range rt.DNSAddrs {
		if _, ok := a.(*net.UnixAddr); ok {
			return fmt.Errorf("DNS address cannot be a unix socket")
		}
	}
	for _, a := range rt.DNSRecursors {
		if ipaddr.IsAny(a) {
			return fmt.Errorf("DNS recursor address cannot be 0.0.0.0, :: or [::]")
		}
	}
	if !isValidAltDomain(rt.DNSAltDomain, rt.Datacenter) {
		return fmt.Errorf("alt_domain cannot start with {service,connect,node,query,addr,%s}", rt.Datacenter)
	}
	if rt.Bootstrap && !rt.ServerMode {
		return fmt.Errorf("'bootstrap = true' requires 'server = true'")
	}
	if rt.BootstrapExpect < 0 {
		return fmt.Errorf("bootstrap_expect cannot be %d. Must be greater than or equal to zero", rt.BootstrapExpect)
	}
	if rt.BootstrapExpect > 0 && !rt.ServerMode {
		return fmt.Errorf("'bootstrap_expect > 0' requires 'server = true'")
	}
	if rt.BootstrapExpect > 0 && rt.DevMode {
		return fmt.Errorf("'bootstrap_expect > 0' not allowed in dev mode")
	}
	if rt.BootstrapExpect > 0 && rt.Bootstrap {
		return fmt.Errorf("'bootstrap_expect > 0' and 'bootstrap = true' are mutually exclusive")
	}
	if rt.CheckOutputMaxSize < 1 {
		return fmt.Errorf("check_output_max_size must be positive, to discard check output use the discard_check_output flag")
	}
	if rt.AEInterval <= 0 {
		return fmt.Errorf("ae_interval cannot be %s. Must be positive", rt.AEInterval)
	}
	if rt.AutopilotMaxTrailingLogs < 0 {
		return fmt.Errorf("autopilot.max_trailing_logs cannot be %d. Must be greater than or equal to zero", rt.AutopilotMaxTrailingLogs)
	}
	if err := validateBasicName("primary_datacenter", rt.PrimaryDatacenter, true); err != nil {
		return err
	}
	// In DevMode, UI is enabled by default, so to enable rt.UIDir, don't perform this check
	if !rt.DevMode && rt.UIConfig.Enabled && rt.UIConfig.Dir != "" {
		return fmt.Errorf(
			"Both the ui_config.enabled and ui_config.dir (or -ui and -ui-dir) were specified, please provide only one.\n" +
				"If trying to use your own web UI resources, use ui_config.dir or the -ui-dir flag.\n" +
				"The web UI is included in the binary so use ui_config.enabled or the -ui flag to enable it")
	}
	if rt.DNSUDPAnswerLimit < 0 {
		return fmt.Errorf("dns_config.udp_answer_limit cannot be %d. Must be greater than or equal to zero", rt.DNSUDPAnswerLimit)
	}
	if rt.DNSARecordLimit < 0 {
		return fmt.Errorf("dns_config.a_record_limit cannot be %d. Must be greater than or equal to zero", rt.DNSARecordLimit)
	}
	if err := structs.ValidateNodeMetadata(rt.NodeMeta, false); err != nil {
		return fmt.Errorf("node_meta invalid: %v", err)
	}
	if rt.EncryptKey != "" {
		if _, err := decodeBytes(rt.EncryptKey); err != nil {
			return fmt.Errorf("encrypt has invalid key: %s", err)
		}
	}

	if rt.ConnectMeshGatewayWANFederationEnabled && !rt.ServerMode {
		return fmt.Errorf("'connect.enable_mesh_gateway_wan_federation = true' requires 'server = true'")
	}
	if rt.ConnectMeshGatewayWANFederationEnabled && strings.ContainsAny(rt.NodeName, "/") {
		return fmt.Errorf("'connect.enable_mesh_gateway_wan_federation = true' requires that 'node_name' not contain '/' characters")
	}
	if rt.ConnectMeshGatewayWANFederationEnabled {
		if len(rt.RetryJoinWAN) > 0 {
			return fmt.Errorf("'retry_join_wan' is incompatible with 'connect.enable_mesh_gateway_wan_federation = true'")
		}
	}
	if len(rt.PrimaryGateways) > 0 {
		if !rt.ServerMode {
			return fmt.Errorf("'primary_gateways' requires 'server = true'")
		}
		if rt.PrimaryDatacenter == rt.Datacenter {
			return fmt.Errorf("'primary_gateways' should only be configured in a secondary datacenter")
		}
	}

	// Check the data dir for signs of an un-migrated Consul 0.5.x or older
	// server. Consul refuses to start if this is present to protect a server
	// with existing data from starting on a fresh data set.
	if rt.ServerMode {
		mdbPath := filepath.Join(rt.DataDir, "mdb")
		if _, err := os.Stat(mdbPath); !os.IsNotExist(err) {
			if os.IsPermission(err) {
				return fmt.Errorf(
					"CRITICAL: Permission denied for data folder at %q!\n"+
						"Consul will refuse to boot without access to this directory.\n"+
						"Please correct permissions and try starting again.", mdbPath)
			}
			return fmt.Errorf("CRITICAL: Deprecated data folder found at %q!\n"+
				"Consul will refuse to boot with this directory present.\n"+
				"See https://www.consul.io/docs/upgrade-specific.html for more information.", mdbPath)
		}

		// Raft LogStore validation
		if rt.RaftLogStoreConfig.Backend != consul.LogStoreBackendBoltDB &&
			rt.RaftLogStoreConfig.Backend != consul.LogStoreBackendWAL {
			return fmt.Errorf("raft_logstore.backend must be one of '%s' or '%s'",
				consul.LogStoreBackendBoltDB, consul.LogStoreBackendWAL)
		}
		if rt.RaftLogStoreConfig.WAL.SegmentSize < 1024*1024 {
			return fmt.Errorf("raft_logstore.wal.segment_size_mb cannot be less than 1MB")
		}
		if rt.RaftLogStoreConfig.WAL.SegmentSize > 1024*1024*1024 {
			return fmt.Errorf("raft_logstore.wal.segment_size_mb cannot be greater than 1024 (1GiB)")
		}
	}

	inuse := map[string]string{}
	if err := addrsUnique(inuse, "DNS", rt.DNSAddrs); err != nil {
		// cannot happen since this is the first address
		// we leave this for consistency
		return err
	}
	if err := addrsUnique(inuse, "HTTP", rt.HTTPAddrs); err != nil {
		return err
	}
	if err := addrsUnique(inuse, "HTTPS", rt.HTTPSAddrs); err != nil {
		return err
	}
	if err := addrUnique(inuse, "RPC Advertise", rt.RPCAdvertiseAddr); err != nil {
		return err
	}
	if err := addrUnique(inuse, "Serf Advertise LAN", rt.SerfAdvertiseAddrLAN); err != nil {
		return err
	}
	// Validate serf WAN advertise address only when its set
	if rt.SerfAdvertiseAddrWAN != nil {
		if err := addrUnique(inuse, "Serf Advertise WAN", rt.SerfAdvertiseAddrWAN); err != nil {
			return err
		}
	}
	if b.err != nil {
		return b.err
	}

	// Check for errors in the service definitions
	for _, s := range rt.Services {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("service %q: %s", s.Name, err)
		}
	}
	// Check for errors in the node check definitions
	for _, c := range rt.Checks {
		if err := c.CheckType().Validate(); err != nil {
			return fmt.Errorf("check %q: %w", c.Name, err)
		}
	}

	// Validate the given Connect CA provider config
	validCAProviders := map[string]bool{
		"":                       true,
		structs.ConsulCAProvider: true,
		structs.VaultCAProvider:  true,
		structs.AWSCAProvider:    true,
	}
	if _, ok := validCAProviders[rt.ConnectCAProvider]; !ok {
		return fmt.Errorf("%s is not a valid CA provider", rt.ConnectCAProvider)
	} else {
		switch rt.ConnectCAProvider {
		case structs.ConsulCAProvider:
			if _, err := ca.ParseConsulCAConfig(rt.ConnectCAConfig); err != nil {
				return err
			}
		case structs.VaultCAProvider:
			if _, err := ca.ParseVaultCAConfig(rt.ConnectCAConfig); err != nil {
				return err
			}
		case structs.AWSCAProvider:
			if _, err := ca.ParseAWSCAConfig(rt.ConnectCAConfig); err != nil {
				return err
			}
		}
	}

	if rt.ServerMode && rt.AutoEncryptTLS {
		return fmt.Errorf("auto_encrypt.tls can only be used on a client.")
	}
	if !rt.ServerMode && rt.AutoEncryptAllowTLS {
		return fmt.Errorf("auto_encrypt.allow_tls can only be used on a server.")
	}

	if rt.ServerMode && rt.AdvertiseReconnectTimeout != 0 {
		return fmt.Errorf("advertise_reconnect_timeout can only be used on a client")
	}

	// ----------------------------------------------------------------
	// warnings
	//

	if rt.ServerMode && !rt.DevMode && !rt.Bootstrap && rt.BootstrapExpect == 2 {
		b.warn(`bootstrap_expect = 2: A cluster with 2 servers will provide no failure tolerance. See https://www.consul.io/docs/internals/consensus.html#deployment-table`)
	}

	if rt.ServerMode && !rt.Bootstrap && rt.BootstrapExpect > 2 && rt.BootstrapExpect%2 == 0 {
		b.warn(`bootstrap_expect is even number: A cluster with an even number of servers does not achieve optimum fault tolerance. See https://www.consul.io/docs/internals/consensus.html#deployment-table`)
	}

	if rt.ServerMode && rt.Bootstrap && rt.BootstrapExpect == 0 {
		b.warn(`bootstrap = true: do not enable unless necessary`)
	}

	if rt.ServerMode && !rt.DevMode && !rt.Bootstrap && rt.BootstrapExpect > 1 {
		b.warn("bootstrap_expect > 0: expecting %d servers", rt.BootstrapExpect)
	}

	if rt.ServerMode {
		if rt.UseStreamingBackend && !rt.RPCConfig.EnableStreaming {
			b.warn("use_streaming_backend = true requires rpc.enable_streaming on servers to work properly")
		}
	} else if rt.RPCConfig.EnableStreaming {
		b.warn("rpc.enable_streaming = true has no effect when not running in server mode")
	}

	if rt.AutoEncryptAllowTLS && !rt.TLS.InternalRPC.VerifyIncoming {
		b.warn("if auto_encrypt.allow_tls is turned on, tls.internal_rpc.verify_incoming should be enabled (either explicitly or via tls.defaults.verify_incoming). It is necessary to turn it off during a migration to TLS, but it should definitely be turned on afterwards.")
	}

	if err := checkLimitsFromMaxConnsPerClient(rt.HTTPMaxConnsPerClient); err != nil {
		return err
	}

	if rt.AutoConfig.Enabled && rt.AutoEncryptTLS {
		return fmt.Errorf("both auto_encrypt.tls and auto_config.enabled cannot be set to true.")
	}

	if err := b.validateAutoConfig(rt); err != nil {
		return err
	}

	if err := validateRemoteScriptsChecks(rt); err != nil {
		// TODO: make this an error in a future version
		b.warn(err.Error())
	}

	err := b.validateEnterpriseConfig(rt)
	return err
}

// addrUnique checks if the given address is already in use for another
// protocol.
func addrUnique(inuse map[string]string, name string, addr net.Addr) error {
	key := addr.Network() + ":" + addr.String()
	if other, ok := inuse[key]; ok {
		return fmt.Errorf("%s address %s already configured for %s", name, addr.String(), other)
	}
	inuse[key] = name
	return nil
}

// addrsUnique checks if any of the give addresses is already in use for
// another protocol.
func addrsUnique(inuse map[string]string, name string, addrs []net.Addr) error {
	for _, a := range addrs {
		if err := addrUnique(inuse, name, a); err != nil {
			return err
		}
	}
	return nil
}

// splitSlicesAndValues moves all slice values defined in c to 'slices'
// and all other values to 'values'.
func splitSlicesAndValues(c Config) (slices, values Config) {
	v, t := reflect.ValueOf(c), reflect.TypeOf(c)
	rs, rv := reflect.New(t), reflect.New(t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type.Kind() == reflect.Slice {
			rs.Elem().Field(i).Set(v.Field(i))
		} else {
			rv.Elem().Field(i).Set(v.Field(i))
		}
	}
	return rs.Elem().Interface().(Config), rv.Elem().Interface().(Config)
}

func (b *builder) warn(msg string, args ...interface{}) {
	b.Warnings = append(b.Warnings, fmt.Sprintf(msg, args...))
}

func (b *builder) checkVal(v *CheckDefinition) *structs.CheckDefinition {
	if v == nil {
		return nil
	}

	var H2PingUseTLSVal bool
	if stringVal(v.H2PING) != "" {
		H2PingUseTLSVal = boolValWithDefault(v.H2PingUseTLS, true)
	} else {
		H2PingUseTLSVal = boolVal(v.H2PingUseTLS)
	}

	id := types.CheckID(stringVal(v.ID))

	return &structs.CheckDefinition{
		ID:                             id,
		Name:                           stringVal(v.Name),
		Notes:                          stringVal(v.Notes),
		ServiceID:                      stringVal(v.ServiceID),
		Token:                          stringVal(v.Token),
		Status:                         stringVal(v.Status),
		ScriptArgs:                     v.ScriptArgs,
		HTTP:                           stringVal(v.HTTP),
		Header:                         v.Header,
		Method:                         stringVal(v.Method),
		Body:                           stringVal(v.Body),
		DisableRedirects:               boolVal(v.DisableRedirects),
		TCP:                            stringVal(v.TCP),
		UDP:                            stringVal(v.UDP),
		Interval:                       b.durationVal(fmt.Sprintf("check[%s].interval", id), v.Interval),
		DockerContainerID:              stringVal(v.DockerContainerID),
		Shell:                          stringVal(v.Shell),
		GRPC:                           stringVal(v.GRPC),
		GRPCUseTLS:                     boolVal(v.GRPCUseTLS),
		TLSServerName:                  stringVal(v.TLSServerName),
		TLSSkipVerify:                  boolVal(v.TLSSkipVerify),
		AliasNode:                      stringVal(v.AliasNode),
		AliasService:                   stringVal(v.AliasService),
		Timeout:                        b.durationVal(fmt.Sprintf("check[%s].timeout", id), v.Timeout),
		TTL:                            b.durationVal(fmt.Sprintf("check[%s].ttl", id), v.TTL),
		SuccessBeforePassing:           intVal(v.SuccessBeforePassing),
		FailuresBeforeCritical:         intVal(v.FailuresBeforeCritical),
		FailuresBeforeWarning:          intValWithDefault(v.FailuresBeforeWarning, intVal(v.FailuresBeforeCritical)),
		H2PING:                         stringVal(v.H2PING),
		H2PingUseTLS:                   H2PingUseTLSVal,
		OSService:                      stringVal(v.OSService),
		DeregisterCriticalServiceAfter: b.durationVal(fmt.Sprintf("check[%s].deregister_critical_service_after", id), v.DeregisterCriticalServiceAfter),
		OutputMaxSize:                  intValWithDefault(v.OutputMaxSize, checks.DefaultBufSize),
		EnterpriseMeta:                 v.EnterpriseMeta.ToStructs(),
	}
}

func (b *builder) svcTaggedAddresses(v map[string]ServiceAddress) map[string]structs.ServiceAddress {
	if len(v) <= 0 {
		return nil
	}

	svcAddrs := make(map[string]structs.ServiceAddress)
	for addrName, addrConf := range v {
		addr := structs.ServiceAddress{}
		if addrConf.Address != nil {
			addr.Address = *addrConf.Address
		}
		if addrConf.Port != nil {
			addr.Port = *addrConf.Port
		}

		svcAddrs[addrName] = addr
	}
	return svcAddrs
}

func (b *builder) serviceVal(v *ServiceDefinition) *structs.ServiceDefinition {
	if v == nil {
		return nil
	}

	var checks structs.CheckTypes
	for _, check := range v.Checks {
		checks = append(checks, b.checkVal(&check).CheckType())
	}
	if v.Check != nil {
		checks = append(checks, b.checkVal(v.Check).CheckType())
	}

	kind := b.serviceKindVal(v.Kind)

	meta := make(map[string]string)
	if err := structs.ValidateServiceMetadata(kind, v.Meta, false); err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("invalid meta for service %s: %v", stringVal(v.Name), err))
	} else {
		meta = v.Meta
	}
	serviceWeights := &structs.Weights{Passing: 1, Warning: 1}
	if v.Weights != nil {
		if v.Weights.Passing != nil {
			serviceWeights.Passing = *v.Weights.Passing
		}
		if v.Weights.Warning != nil {
			serviceWeights.Warning = *v.Weights.Warning
		}
	}

	if err := structs.ValidateWeights(serviceWeights); err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("Invalid weight definition for service %s: %s", stringVal(v.Name), err))
	}

	if (v.Port != nil || v.Address != nil) && (v.SocketPath != nil) {
		b.err = multierror.Append(b.err,
			fmt.Errorf("service %s cannot have both socket path %s and address/port",
				stringVal(v.Name), stringVal(v.SocketPath)))
	}

	return &structs.ServiceDefinition{
		Kind:              kind,
		ID:                stringVal(v.ID),
		Name:              stringVal(v.Name),
		Tags:              v.Tags,
		Address:           stringVal(v.Address),
		TaggedAddresses:   b.svcTaggedAddresses(v.TaggedAddresses),
		Meta:              meta,
		Port:              intVal(v.Port),
		SocketPath:        stringVal(v.SocketPath),
		Token:             stringVal(v.Token),
		EnableTagOverride: boolVal(v.EnableTagOverride),
		Weights:           serviceWeights,
		Checks:            checks,
		Proxy:             b.serviceProxyVal(v.Proxy),
		Connect:           b.serviceConnectVal(v.Connect),
		EnterpriseMeta:    v.EnterpriseMeta.ToStructs(),
	}
}

func (b *builder) serviceKindVal(v *string) structs.ServiceKind {
	if v == nil {
		return structs.ServiceKindTypical
	}
	switch *v {
	case string(structs.ServiceKindConnectProxy):
		return structs.ServiceKindConnectProxy
	case string(structs.ServiceKindMeshGateway):
		return structs.ServiceKindMeshGateway
	case string(structs.ServiceKindTerminatingGateway):
		return structs.ServiceKindTerminatingGateway
	case string(structs.ServiceKindIngressGateway):
		return structs.ServiceKindIngressGateway
	case string(structs.ServiceKindAPIGateway):
		return structs.ServiceKindAPIGateway
	default:
		return structs.ServiceKindTypical
	}
}

func (b *builder) serviceProxyVal(v *ServiceProxy) *structs.ConnectProxyConfig {
	if v == nil {
		return nil
	}

	return &structs.ConnectProxyConfig{
		DestinationServiceName: stringVal(v.DestinationServiceName),
		DestinationServiceID:   stringVal(v.DestinationServiceID),
		LocalServiceAddress:    stringVal(v.LocalServiceAddress),
		LocalServicePort:       intVal(v.LocalServicePort),
		LocalServiceSocketPath: stringVal(&v.LocalServiceSocketPath),
		Config:                 v.Config,
		Upstreams:              b.upstreamsVal(v.Upstreams),
		MeshGateway:            b.meshGatewayConfVal(v.MeshGateway),
		Expose:                 b.exposeConfVal(v.Expose),
		Mode:                   b.proxyModeVal(v.Mode),
		TransparentProxy:       b.transparentProxyConfVal(v.TransparentProxy),
	}
}

func (b *builder) upstreamsVal(v []Upstream) structs.Upstreams {
	ups := make(structs.Upstreams, len(v))
	for i, u := range v {
		ups[i] = structs.Upstream{
			DestinationType:      stringVal(u.DestinationType),
			DestinationNamespace: stringVal(u.DestinationNamespace),
			DestinationPartition: stringVal(u.DestinationPartition),
			DestinationPeer:      stringVal(u.DestinationPeer),
			DestinationName:      stringVal(u.DestinationName),
			Datacenter:           stringVal(u.Datacenter),
			LocalBindAddress:     stringVal(u.LocalBindAddress),
			LocalBindPort:        intVal(u.LocalBindPort),
			LocalBindSocketPath:  stringVal(u.LocalBindSocketPath),
			LocalBindSocketMode:  b.unixPermissionsVal("local_bind_socket_mode", u.LocalBindSocketMode),
			Config:               u.Config,
			MeshGateway:          b.meshGatewayConfVal(u.MeshGateway),
		}
		if ups[i].DestinationType == "" {
			ups[i].DestinationType = structs.UpstreamDestTypeService
		}
	}
	return ups
}

func (b *builder) meshGatewayConfVal(mgConf *MeshGatewayConfig) structs.MeshGatewayConfig {
	cfg := structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeDefault}
	if mgConf == nil || mgConf.Mode == nil {
		// return defaults
		return cfg
	}

	mode, err := structs.ValidateMeshGatewayMode(*mgConf.Mode)
	if err != nil {
		b.err = multierror.Append(b.err, err)
		return cfg
	}

	cfg.Mode = mode
	return cfg
}

func (b *builder) dnsRecursorStrategyVal(v string) dns.RecursorStrategy {
	var out dns.RecursorStrategy

	switch dns.RecursorStrategy(v) {
	case dns.RecursorStrategyRandom:
		out = dns.RecursorStrategyRandom
	case dns.RecursorStrategySequential, "":
		out = dns.RecursorStrategySequential
	default:
		b.err = multierror.Append(b.err, fmt.Errorf("dns_config.recursor_strategy: invalid strategy: %q", v))
	}
	return out
}

func (b *builder) requestsLimitsModeVal(v string) consulrate.Mode {
	var out consulrate.Mode

	mode, ok := consulrate.RequestLimitsModeFromName(v)
	if !ok {
		b.err = multierror.Append(b.err, fmt.Errorf("limits.request_limits.mode: invalid mode: %q", v))
	} else {
		out = mode
	}

	return out
}

func (b *builder) exposeConfVal(v *ExposeConfig) structs.ExposeConfig {
	var out structs.ExposeConfig
	if v == nil {
		return out
	}

	out.Checks = boolVal(v.Checks)
	out.Paths = b.pathsVal(v.Paths)
	return out
}

func (b *builder) transparentProxyConfVal(tproxyConf *TransparentProxyConfig) structs.TransparentProxyConfig {
	var out structs.TransparentProxyConfig
	if tproxyConf == nil {
		return out
	}

	out.OutboundListenerPort = intVal(tproxyConf.OutboundListenerPort)
	out.DialedDirectly = boolVal(tproxyConf.DialedDirectly)
	return out
}

func (b *builder) proxyModeVal(v *string) structs.ProxyMode {
	if v == nil {
		return structs.ProxyModeDefault
	}

	mode, err := structs.ValidateProxyMode(*v)
	if err != nil {
		b.err = multierror.Append(b.err, err)
	}
	return mode
}

func (b *builder) pathsVal(v []ExposePath) []structs.ExposePath {
	paths := make([]structs.ExposePath, len(v))
	for i, p := range v {
		paths[i] = structs.ExposePath{
			ListenerPort:  intVal(p.ListenerPort),
			Path:          stringVal(p.Path),
			LocalPathPort: intVal(p.LocalPathPort),
			Protocol:      stringVal(p.Protocol),
		}
	}
	return paths
}

func (b *builder) serviceConnectVal(v *ServiceConnect) *structs.ServiceConnect {
	if v == nil {
		return nil
	}

	sidecar := b.serviceVal(v.SidecarService)
	if sidecar != nil {
		// Sanity checks
		if sidecar.ID != "" {
			b.err = multierror.Append(b.err, fmt.Errorf("sidecar_service can't specify an ID"))
			sidecar.ID = ""
		}
		if sidecar.Connect != nil {
			if sidecar.Connect.SidecarService != nil {
				b.err = multierror.Append(b.err, fmt.Errorf("sidecar_service can't have a nested sidecar_service"))
				sidecar.Connect.SidecarService = nil
			}
		}
	}

	return &structs.ServiceConnect{
		Native:         boolVal(v.Native),
		SidecarService: sidecar,
	}
}

func (b *builder) uiConfigVal(v RawUIConfig) UIConfig {
	return UIConfig{
		Enabled:                    boolVal(v.Enabled),
		Dir:                        stringVal(v.Dir),
		ContentPath:                UIPathBuilder(stringVal(v.ContentPath)),
		MetricsProvider:            stringVal(v.MetricsProvider),
		MetricsProviderFiles:       v.MetricsProviderFiles,
		MetricsProviderOptionsJSON: stringVal(v.MetricsProviderOptionsJSON),
		MetricsProxy:               b.uiMetricsProxyVal(v.MetricsProxy),
		DashboardURLTemplates:      v.DashboardURLTemplates,
		HCPEnabled:                 os.Getenv("CONSUL_HCP_ENABLED") == "true",
	}
}

func (b *builder) uiMetricsProxyVal(v RawUIMetricsProxy) UIMetricsProxy {
	var hdrs []UIMetricsProxyAddHeader

	for _, hdr := range v.AddHeaders {
		hdrs = append(hdrs, UIMetricsProxyAddHeader{
			Name:  stringVal(hdr.Name),
			Value: stringVal(hdr.Value),
		})
	}

	return UIMetricsProxy{
		BaseURL:       stringVal(v.BaseURL),
		AddHeaders:    hdrs,
		PathAllowlist: v.PathAllowlist,
	}
}

func boolValWithDefault(v *bool, defaultVal bool) bool {
	if v == nil {
		return defaultVal
	}
	return *v
}

func boolVal(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func (b *builder) durationValWithDefault(name string, v *string, defaultVal time.Duration) (d time.Duration) {
	if v == nil {
		return defaultVal
	}
	d, err := time.ParseDuration(*v)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid duration: %q: %s", name, *v, err))
	}
	return d
}

// durationValWithDefaultMin is equivalent to durationValWithDefault, but enforces a minimum duration.
func (b *builder) durationValWithDefaultMin(name string, v *string, defaultVal, minVal time.Duration) (d time.Duration) {
	d = b.durationValWithDefault(name, v, defaultVal)
	if d < minVal {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: duration '%s' cannot be less than: %s", name, *v, minVal))
	}

	return d
}

func (b *builder) durationVal(name string, v *string) (d time.Duration) {
	return b.durationValWithDefault(name, v, 0)
}

func intValWithDefault(v *int, defaultVal int) int {
	if v == nil {
		return defaultVal
	}
	return *v
}

func intVal(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func uintVal(v *uint) uint {
	if v == nil {
		return 0
	}
	return *v
}

func uint64Val(v *uint64) uint64 {
	if v == nil {
		return 0
	}
	return *v
}

// Expect an octal permissions string, e.g. 0644
func (b *builder) unixPermissionsVal(name string, v *string) string {
	if v == nil {
		return ""
	}
	if _, err := strconv.ParseUint(*v, 8, 32); err == nil {
		return *v
	}
	b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid mode: %s", name, *v))
	return "0"
}

func (b *builder) portVal(name string, v *int) int {
	if v == nil || *v <= 0 {
		return -1
	}
	if *v > 65535 {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid port: %d", name, *v))
	}
	return *v
}

func stringValWithDefault(v *string, defaultVal string) string {
	if v == nil {
		return defaultVal
	}
	return *v
}

func stringVal(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func timeValWithDefault(v *time.Time, defaultVal time.Time) time.Time {
	if v == nil {
		return defaultVal
	}
	return *v
}

func float64ValWithDefault(v *float64, defaultVal float64) float64 {
	if v == nil {
		return defaultVal
	}
	return *v
}

func float64Val(v *float64) float64 {
	return float64ValWithDefault(v, 0)
}

func limitVal(v *float64) rate.Limit {
	f := float64Val(v)
	if f < 0 {
		return rate.Inf
	}

	return rate.Limit(f)
}

func limitValWithDefault(v *float64, defaultVal float64) rate.Limit {
	f := float64ValWithDefault(v, defaultVal)
	return limitVal(&f)
}

func (b *builder) cidrsVal(name string, v []string) (nets []*net.IPNet) {
	if v == nil {
		return
	}

	for _, p := range v {
		_, net, err := net.ParseCIDR(strings.TrimSpace(p))
		if err != nil {
			b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid cidr: %s", name, p))
		}
		nets = append(nets, net)
	}

	return
}

func (b *builder) tlsVersion(name string, v *string) types.TLSVersion {
	// Handles unspecified config and empty string case.
	//
	// This check is not inside types.ValidateTLSVersionString because Envoy config
	// distinguishes between an unset empty string which inherits parent config and
	// an explicit TLS_AUTO which allows overriding parent config with the proxy
	// defaults.
	if v == nil || *v == "" {
		return types.TLSVersionAuto
	}

	a := types.TLSVersion(*v)

	err := types.ValidateTLSVersion(a)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid TLS version: %s", name, err))
		return types.TLSVersionInvalid
	}
	return a
}

// validateTLSVersionCipherSuitesCompat checks that the specified TLS version supports
// specifying cipher suites
func validateTLSVersionCipherSuitesCompat(tlsMinVersion types.TLSVersion) error {
	if tlsMinVersion == types.TLSv1_3 {
		return fmt.Errorf("TLS 1.3 cipher suites are not configurable")
	}
	return nil
}

// tlsCipherSuites parses cipher suites from a comma-separated string into a
// recognized slice
func (b *builder) tlsCipherSuites(name string, v *string, tlsMinVersion types.TLSVersion) []types.TLSCipherSuite {
	if v == nil {
		return nil
	}

	if err := validateTLSVersionCipherSuitesCompat(tlsMinVersion); err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: %s", name, err))
		return nil
	}

	*v = strings.TrimSpace(*v)
	if *v == "" {
		return []types.TLSCipherSuite{}
	}
	ciphers := strings.Split(*v, ",")

	a := make([]types.TLSCipherSuite, len(ciphers))
	for i, cipher := range ciphers {
		a[i] = types.TLSCipherSuite(cipher)
	}

	err := types.ValidateConsulAgentCipherSuites(a)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid TLS cipher suites: %s", name, err))
		return []types.TLSCipherSuite{}
	}
	return a
}

func (b *builder) nodeName(v *string) string {
	nodeName := stringVal(v)
	if nodeName == "" {
		fn := b.opts.hostname
		if fn == nil {
			fn = os.Hostname
		}
		name, err := fn()
		if err != nil {
			b.err = multierror.Append(b.err, fmt.Errorf("node_name: %s", err))
			return ""
		}
		nodeName = name
	}
	return strings.TrimSpace(nodeName)
}

// expandAddrs expands the go-sockaddr template in s and returns the
// result as a list of *net.IPAddr and *net.UnixAddr.
func (b *builder) expandAddrs(name string, s *string) []net.Addr {
	if s == nil || *s == "" {
		return nil
	}

	x, err := template.Parse(*s)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: error parsing %q: %s", name, *s, err))
		return nil
	}

	var addrs []net.Addr
	for _, a := range strings.Fields(x) {
		switch {
		case strings.HasPrefix(a, "unix://"):
			addrs = append(addrs, &net.UnixAddr{Name: a[len("unix://"):], Net: "unix"})
		default:
			// net.ParseIP does not like '[::]'
			ip := net.ParseIP(a)
			if a == "[::]" {
				ip = net.ParseIP("::")
			}
			if ip == nil {
				b.err = multierror.Append(b.err, fmt.Errorf("%s: invalid ip address: %s", name, a))
				return nil
			}
			addrs = append(addrs, &net.IPAddr{IP: ip})
		}
	}

	return addrs
}

// expandOptionalAddrs expands the go-sockaddr template in s and returns the
// result as a list of strings. If s does not contain a go-sockaddr template,
// the result list will contain the input string as a single element with no
// error set. In contrast to expandAddrs, expandOptionalAddrs does not validate
// if the result contains valid addresses and returns a list of strings.
// However, if the expansion of the go-sockaddr template fails an error is set.
func (b *builder) expandOptionalAddrs(name string, s *string) []string {
	if s == nil || *s == "" {
		return nil
	}

	x, err := template.Parse(*s)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("%s: error parsing %q: %s", name, *s, err))
		return nil
	}

	if x != *s {
		// A template has been expanded, split the results from go-sockaddr
		return strings.Fields(x)
	} else {
		// No template has been expanded, pass through the input
		return []string{*s}
	}
}

func (b *builder) expandAllOptionalAddrs(name string, addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		expanded := b.expandOptionalAddrs(name, &a)
		if expanded != nil {
			out = append(out, expanded...)
		}
	}
	return out
}

// expandIPs expands the go-sockaddr template in s and returns a list of
// *net.IPAddr. If one of the expanded addresses is a unix socket
// address an error is set and nil is returned.
func (b *builder) expandIPs(name string, s *string) []*net.IPAddr {
	if s == nil || *s == "" {
		return nil
	}

	addrs := b.expandAddrs(name, s)
	var x []*net.IPAddr
	for _, addr := range addrs {
		switch a := addr.(type) {
		case *net.IPAddr:
			x = append(x, a)
		case *net.UnixAddr:
			b.err = multierror.Append(b.err, fmt.Errorf("%s cannot be a unix socket", name))
			return nil
		default:
			b.err = multierror.Append(b.err, fmt.Errorf("%s has invalid address type %T", name, a))
			return nil
		}
	}
	return x
}

// expandFirstAddr expands the go-sockaddr template in s and returns the
// first address which is either a *net.IPAddr or a *net.UnixAddr. If
// the template expands to multiple addresses an error is set and nil
// is returned.
func (b *builder) expandFirstAddr(name string, s *string) net.Addr {
	if s == nil || *s == "" {
		return nil
	}

	addrs := b.expandAddrs(name, s)
	if len(addrs) == 0 {
		return nil
	}
	if len(addrs) > 1 {
		var x []string
		for _, a := range addrs {
			x = append(x, a.String())
		}
		b.err = multierror.Append(b.err, fmt.Errorf("%s: multiple addresses found: %s", name, strings.Join(x, " ")))
		return nil
	}
	return addrs[0]
}

// expandFirstIP expands the go-sockaddr template in s and returns the
// first address if it is not a unix socket address. If the template
// expands to multiple addresses an error is set and nil is returned.
func (b *builder) expandFirstIP(name string, s *string) *net.IPAddr {
	if s == nil || *s == "" {
		return nil
	}

	addr := b.expandFirstAddr(name, s)
	if addr == nil {
		return nil
	}
	switch a := addr.(type) {
	case *net.IPAddr:
		return a
	case *net.UnixAddr:
		b.err = multierror.Append(b.err, fmt.Errorf("%s cannot be a unix socket", name))
		return nil
	default:
		b.err = multierror.Append(b.err, fmt.Errorf("%s has invalid address type %T", name, a))
		return nil
	}
}

func makeIPAddr(pri *net.IPAddr, sec *net.IPAddr) *net.IPAddr {
	if pri != nil {
		return pri
	}
	return sec
}

func (b *builder) makeTCPAddr(pri *net.IPAddr, sec net.Addr, port int) *net.TCPAddr {
	if pri == nil && reflect.ValueOf(sec).IsNil() || port <= 0 {
		return nil
	}
	addr := pri
	if addr == nil {
		switch a := sec.(type) {
		case *net.IPAddr:
			addr = a
		case *net.TCPAddr:
			addr = &net.IPAddr{IP: a.IP}
		default:
			panic(fmt.Sprintf("makeTCPAddr requires a net.IPAddr or a net.TCPAddr. Got %T", a))
		}
	}
	return &net.TCPAddr{IP: addr.IP, Port: port}
}

// makeAddr creates an *net.TCPAddr or a *net.UnixAddr from either the
// primary or secondary address and the given port. If the port is <= 0
// then the address is considered to be disabled and nil is returned.
func (b *builder) makeAddr(pri, sec net.Addr, port int) net.Addr {
	if reflect.ValueOf(pri).IsNil() && reflect.ValueOf(sec).IsNil() || port <= 0 {
		return nil
	}
	addr := pri
	if addr == nil {
		addr = sec
	}
	switch a := addr.(type) {
	case *net.IPAddr:
		return &net.TCPAddr{IP: a.IP, Port: port}
	case *net.UnixAddr:
		return a
	default:
		panic(fmt.Sprintf("invalid address type %T", a))
	}
}

// makeAddrs creates a list of *net.TCPAddr or *net.UnixAddr entries
// from either the primary or secondary addresses and the given port.
// If the port is <= 0 then the address is considered to be disabled
// and nil is returned.
func (b *builder) makeAddrs(pri []net.Addr, sec []*net.IPAddr, port int) []net.Addr {
	if len(pri) == 0 && len(sec) == 0 || port <= 0 {
		return nil
	}
	addrs := pri
	if len(addrs) == 0 {
		addrs = []net.Addr{}
		for _, a := range sec {
			addrs = append(addrs, a)
		}
	}
	var x []net.Addr
	for _, a := range addrs {
		x = append(x, b.makeAddr(a, nil, port))
	}
	return x
}

func (b *builder) autoConfigVal(raw AutoConfigRaw, agentPartition string) AutoConfig {
	var val AutoConfig

	val.Enabled = boolValWithDefault(raw.Enabled, false)
	val.IntroToken = stringVal(raw.IntroToken)

	// default the IntroToken to the env variable if specified.
	if envToken := os.Getenv("CONSUL_INTRO_TOKEN"); envToken != "" {
		if val.IntroToken != "" {
			b.warn("Both auto_config.intro_token and the CONSUL_INTRO_TOKEN environment variable are set. Using the value from the environment variable")
		}

		val.IntroToken = envToken
	}
	val.IntroTokenFile = stringVal(raw.IntroTokenFile)
	// These can be go-discover values and so don't have to resolve fully yet
	val.ServerAddresses = b.expandAllOptionalAddrs("auto_config.server_addresses", raw.ServerAddresses)
	val.DNSSANs = raw.DNSSANs

	for _, i := range raw.IPSANs {
		ip := net.ParseIP(i)
		if ip == nil {
			b.warn(fmt.Sprintf("Cannot parse ip %q from auto_config.ip_sans", i))
			continue
		}
		val.IPSANs = append(val.IPSANs, ip)
	}

	val.Authorizer = b.autoConfigAuthorizerVal(raw.Authorization, agentPartition)

	return val
}

func (b *builder) autoConfigAuthorizerVal(raw AutoConfigAuthorizationRaw, agentPartition string) AutoConfigAuthorizer {
	// Our config file syntax wraps the static authorizer configuration in a "static" stanza. However
	// internally we do not support multiple configured authorization types so the RuntimeConfig just
	// inlines the static one. While we can and probably should extend the authorization types in the
	// future to support dynamic authorizers (ACL Auth Methods configured via normal APIs) its not
	// needed right now so the configuration types will remain simplistic until they need to be otherwise.
	var val AutoConfigAuthorizer

	entMeta := structs.DefaultEnterpriseMetaInPartition(agentPartition)
	entMeta.Normalize()

	val.Enabled = boolValWithDefault(raw.Enabled, false)
	val.ClaimAssertions = raw.Static.ClaimAssertions
	val.AllowReuse = boolValWithDefault(raw.Static.AllowReuse, false)
	val.AuthMethod = structs.ACLAuthMethod{
		Name:           "Auto Config Authorizer",
		Type:           "jwt",
		EnterpriseMeta: *entMeta,
		Config: map[string]interface{}{
			"JWTSupportedAlgs":     raw.Static.JWTSupportedAlgs,
			"BoundAudiences":       raw.Static.BoundAudiences,
			"ClaimMappings":        raw.Static.ClaimMappings,
			"ListClaimMappings":    raw.Static.ListClaimMappings,
			"OIDCDiscoveryURL":     stringVal(raw.Static.OIDCDiscoveryURL),
			"OIDCDiscoveryCACert":  stringVal(raw.Static.OIDCDiscoveryCACert),
			"JWKSURL":              stringVal(raw.Static.JWKSURL),
			"JWKSCACert":           stringVal(raw.Static.JWKSCACert),
			"JWTValidationPubKeys": raw.Static.JWTValidationPubKeys,
			"BoundIssuer":          stringVal(raw.Static.BoundIssuer),
			"ExpirationLeeway":     b.durationVal("auto_config.authorization.static.expiration_leeway", raw.Static.ExpirationLeeway),
			"NotBeforeLeeway":      b.durationVal("auto_config.authorization.static.not_before_leeway", raw.Static.NotBeforeLeeway),
			"ClockSkewLeeway":      b.durationVal("auto_config.authorization.static.clock_skew_leeway", raw.Static.ClockSkewLeeway),
		},
	}

	return val
}

func (b *builder) validateAutoConfig(rt RuntimeConfig) error {
	autoconf := rt.AutoConfig

	if err := validateAutoConfigAuthorizer(rt); err != nil {
		return err
	}

	if !autoconf.Enabled {
		return nil
	}

	// Right now we require TLS as everything we are going to transmit via auto-config is sensitive. Signed Certificates, Tokens
	// and other encryption keys. This must be transmitted over a secure connection so we don't allow doing otherwise.
	if !rt.TLS.InternalRPC.VerifyOutgoing {
		return fmt.Errorf("auto_config.enabled cannot be set without configuring TLS for server communications")
	}

	// Auto Config doesn't currently support configuring servers
	if rt.ServerMode {
		return fmt.Errorf("auto_config.enabled cannot be set to true for server agents.")
	}

	// When both are set we will prefer the given value over the file.
	if autoconf.IntroToken != "" && autoconf.IntroTokenFile != "" {
		b.warn("Both an intro token and intro token file are set. The intro token will be used instead of the file")
	} else if autoconf.IntroToken == "" && autoconf.IntroTokenFile == "" {
		return fmt.Errorf("One of auto_config.intro_token, auto_config.intro_token_file or the CONSUL_INTRO_TOKEN environment variable must be set to enable auto_config")
	}

	if len(autoconf.ServerAddresses) == 0 {
		// TODO (autoconf) can we/should we infer this from the join/retry join addresses. I think no, as we will potentially
		// be overriding those retry join addresses with the autoconf process anyways.
		return fmt.Errorf("auto_config.enabled is set without providing a list of addresses")
	}

	return nil
}

func validateAutoConfigAuthorizer(rt RuntimeConfig) error {
	authz := rt.AutoConfig.Authorizer

	if !authz.Enabled {
		return nil
	}

	// When in a secondary datacenter with ACLs enabled, we require token replication to be enabled
	// as that is what allows us to create the local tokens to distribute to the clients. Otherwise
	// we would have to have a token with the ability to create ACL tokens in the primary and make
	// RPCs in response to auto config requests.
	if rt.ACLsEnabled && rt.PrimaryDatacenter != rt.Datacenter && !rt.ACLTokenReplication {
		return fmt.Errorf("Enabling auto-config authorization (auto_config.authorization.enabled) in non primary datacenters with ACLs enabled (acl.enabled) requires also enabling ACL token replication (acl.enable_token_replication)")
	}

	// Auto Config Authorization is only supported on servers
	if !rt.ServerMode {
		return fmt.Errorf("auto_config.authorization.enabled cannot be set to true for client agents")
	}

	// Right now we require TLS as everything we are going to transmit via auto-config is sensitive. Signed Certificates, Tokens
	// and other encryption keys. This must be transmitted over a secure connection so we don't allow doing otherwise.
	if rt.TLS.InternalRPC.CertFile == "" {
		return fmt.Errorf("auto_config.authorization.enabled cannot be set without providing a TLS certificate for the server")
	}

	// build out the validator to ensure that the given configuration was valid
	null := hclog.NewNullLogger()
	validator, err := ssoauth.NewValidator(null, &authz.AuthMethod)
	if err != nil {
		return fmt.Errorf("auto_config.authorization.static has invalid configuration: %v", err)
	}

	// create a blank identity for use to validate the claim assertions.
	blankID := validator.NewIdentity()
	varMap := map[string]string{
		"node":      "fake",
		"segment":   "fake",
		"partition": "fake",
	}

	// validate all the claim assertions
	for _, raw := range authz.ClaimAssertions {
		// validate any HIL
		filled, err := libtempl.InterpolateHIL(raw, varMap, true)
		if err != nil {
			return fmt.Errorf("auto_config.authorization.static.claim_assertion %q is invalid: %v", raw, err)
		}

		// validate the bexpr syntax - note that for now all the keys mapped by the claim mappings
		// are not validateable due to them being put inside a map. Some bexpr updates to setup keys
		// from current map keys would probably be nice here.
		if _, err := bexpr.CreateEvaluatorForType(filled, nil, blankID.SelectableFields); err != nil {
			return fmt.Errorf("auto_config.authorization.static.claim_assertion %q is invalid: %v", raw, err)
		}
	}
	return nil
}

func (b *builder) cloudConfigVal(v *CloudConfigRaw) hcpconfig.CloudConfig {
	val := hcpconfig.CloudConfig{
		ResourceID: os.Getenv("HCP_RESOURCE_ID"),
	}
	if v == nil {
		return val
	}

	val.ClientID = stringVal(v.ClientID)
	val.ClientSecret = stringVal(v.ClientSecret)
	val.AuthURL = stringVal(v.AuthURL)
	val.Hostname = stringVal(v.Hostname)
	val.ScadaAddress = stringVal(v.ScadaAddress)

	if resourceID := stringVal(v.ResourceID); resourceID != "" {
		val.ResourceID = resourceID
	}
	return val
}

// decodeBytes returns the encryption key decoded.
func decodeBytes(key string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(key)
}

func isIPAddr(a net.Addr) bool {
	_, ok := a.(*net.IPAddr)
	return ok
}

func isUnixAddr(a net.Addr) bool {
	_, ok := a.(*net.UnixAddr)
	return ok
}

// isValidAltDomain returns true if the given domain is not prefixed
// by keywords used when dispatching DNS requests
func isValidAltDomain(domain, datacenter string) bool {
	reAltDomain := regexp.MustCompile(
		fmt.Sprintf(
			"^(service|connect|node|query|addr|%s)\\.(%s\\.)?",
			datacenter, datacenter,
		),
	)
	return !reAltDomain.MatchString(domain)
}

// UIPathBuilder checks to see if there was a path set
// If so, adds beginning and trailing slashes to UI path
func UIPathBuilder(UIContentString string) string {
	if UIContentString != "" {
		var fmtedPath string
		fmtedPath = strings.Trim(UIContentString, "/")
		fmtedPath = "/" + fmtedPath + "/"
		return fmtedPath

	}
	return "/ui/"
}

const remoteScriptCheckSecurityWarning = "using enable-script-checks without ACLs and without allow_write_http_from is DANGEROUS, use enable-local-script-checks instead, see https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations/"

// validateRemoteScriptsChecks returns an error if EnableRemoteScriptChecks is
// enabled without other security features, which mitigate the risk of executing
// remote scripts.
func validateRemoteScriptsChecks(conf RuntimeConfig) error {
	if conf.EnableRemoteScriptChecks && !conf.ACLsEnabled && len(conf.AllowWriteHTTPFrom) == 0 {
		return errors.New(remoteScriptCheckSecurityWarning)
	}
	return nil
}

func validateAbsoluteURLPath(p string) error {
	if !path.IsAbs(p) {
		return fmt.Errorf("path %q is not an absolute path", p)
	}

	// A bit more extra validation that these are actually paths.
	u, err := url.Parse(p)
	if err != nil ||
		u.Scheme != "" ||
		u.Opaque != "" ||
		u.User != nil ||
		u.Host != "" ||
		u.RawQuery != "" ||
		u.Fragment != "" ||
		u.Path != p {
		return fmt.Errorf("path %q is not an absolute path", p)
	}

	return nil
}

func (b *builder) buildTLSConfig(rt RuntimeConfig, t TLS) (tlsutil.Config, error) {
	var c tlsutil.Config

	// Consul makes no outgoing connections to the public gRPC port (internal gRPC
	// traffic goes through the multiplexed internal RPC port) so return an error
	// rather than let the user think this setting is going to do anything useful.
	if t.GRPC.VerifyOutgoing != nil {
		return c, errors.New("verify_outgoing is not valid in the tls.grpc stanza")
	}

	// Similarly, only the internal RPC configuration honors VerifyServerHostname
	// so we call it out here too.
	if t.Defaults.VerifyServerHostname != nil || t.GRPC.VerifyServerHostname != nil || t.HTTPS.VerifyServerHostname != nil {
		return c, errors.New("verify_server_hostname is only valid in the tls.internal_rpc stanza")
	}

	// And UseAutoCert right now only applies to external gRPC interface.
	if t.Defaults.UseAutoCert != nil || t.HTTPS.UseAutoCert != nil || t.InternalRPC.UseAutoCert != nil {
		return c, errors.New("use_auto_cert is only valid in the tls.grpc stanza")
	}

	defaultTLSMinVersion := b.tlsVersion("tls.defaults.tls_min_version", t.Defaults.TLSMinVersion)
	defaultCipherSuites := b.tlsCipherSuites("tls.defaults.tls_cipher_suites", t.Defaults.TLSCipherSuites, defaultTLSMinVersion)

	mapCommon := func(name string, src TLSProtocolConfig, dst *tlsutil.ProtocolConfig) {
		dst.CAPath = stringValWithDefault(src.CAPath, stringVal(t.Defaults.CAPath))
		dst.CAFile = stringValWithDefault(src.CAFile, stringVal(t.Defaults.CAFile))
		dst.CertFile = stringValWithDefault(src.CertFile, stringVal(t.Defaults.CertFile))
		dst.KeyFile = stringValWithDefault(src.KeyFile, stringVal(t.Defaults.KeyFile))
		dst.VerifyIncoming = boolValWithDefault(src.VerifyIncoming, boolVal(t.Defaults.VerifyIncoming))

		// We prevent this from being set explicity in the tls.grpc stanza above, but
		// let's also prevent it from getting the tls.defaults value to avoid confusion
		// if we decide to support it in the future.
		if name != "grpc" {
			dst.VerifyOutgoing = boolValWithDefault(src.VerifyOutgoing, boolVal(t.Defaults.VerifyOutgoing))
		}

		if src.TLSMinVersion == nil {
			dst.TLSMinVersion = defaultTLSMinVersion
		} else {
			dst.TLSMinVersion = b.tlsVersion(
				fmt.Sprintf("tls.%s.tls_min_version", name),
				src.TLSMinVersion,
			)
		}

		if src.TLSCipherSuites == nil {
			// If cipher suite config incompatible with a specified TLS min version
			// would be inherited, omit it but don't return an error in the builder.
			if validateTLSVersionCipherSuitesCompat(dst.TLSMinVersion) == nil {
				dst.CipherSuites = defaultCipherSuites
			}
		} else {
			dst.CipherSuites = b.tlsCipherSuites(
				fmt.Sprintf("tls.%s.tls_cipher_suites", name),
				src.TLSCipherSuites,
				dst.TLSMinVersion,
			)
		}
	}

	mapCommon("internal_rpc", t.InternalRPC, &c.InternalRPC)
	c.InternalRPC.VerifyServerHostname = boolVal(t.InternalRPC.VerifyServerHostname)

	// Setting only verify_server_hostname is documented to imply verify_outgoing.
	// If it doesn't then we risk sending communication over plain TCP when we
	// documented it as forcing TLS for RPCs. Enforce this here rather than in
	// several different places through the code that need to reason about it.
	//
	// See: CVE-2018-19653
	c.InternalRPC.VerifyOutgoing = c.InternalRPC.VerifyOutgoing || c.InternalRPC.VerifyServerHostname

	mapCommon("https", t.HTTPS, &c.HTTPS)
	mapCommon("grpc", t.GRPC, &c.GRPC)
	c.GRPC.UseAutoCert = boolValWithDefault(t.GRPC.UseAutoCert, false)

	c.ServerMode = rt.ServerMode
	c.ServerName = rt.ServerName
	c.NodeName = rt.NodeName
	c.Domain = rt.DNSDomain
	c.EnableAgentTLSForChecks = rt.EnableAgentTLSForChecks
	c.AutoTLS = rt.AutoEncryptTLS || rt.AutoConfig.Enabled

	return c, nil
}

func (b *builder) parsePrefixFilter(telemetry *Telemetry) ([]string, []string) {
	var telemetryAllowedPrefixes, telemetryBlockedPrefixes []string

	// TODO(FFMMM): Once one twelve style RPC metrics get out of Beta, don't remove them by default.
	operatorPassedOneTwelveRPCMetric := false
	oneTwelveRPCMetric := *telemetry.MetricsPrefix + "." + strings.Join(middleware.OneTwelveRPCSummary[0].Name, ".")

	for _, rule := range telemetry.PrefixFilter {
		if rule == "" {
			b.warn("Cannot have empty filter rule in prefix_filter")
			continue
		}
		switch rule[0] {
		case '+':
			if rule[1:] == oneTwelveRPCMetric {
				operatorPassedOneTwelveRPCMetric = true
			}
			telemetryAllowedPrefixes = append(telemetryAllowedPrefixes, rule[1:])
		case '-':
			if rule[1:] == oneTwelveRPCMetric {
				operatorPassedOneTwelveRPCMetric = true
			}
			telemetryBlockedPrefixes = append(telemetryBlockedPrefixes, rule[1:])
		default:
			b.warn("Filter rule must begin with either '+' or '-': %q", rule)
		}
	}

	if !operatorPassedOneTwelveRPCMetric {
		telemetryBlockedPrefixes = append(telemetryBlockedPrefixes, oneTwelveRPCMetric)
	}

	return telemetryAllowedPrefixes, telemetryBlockedPrefixes
}

func (b *builder) raftLogStoreConfigVal(raw *RaftLogStoreRaw) consul.RaftLogStoreConfig {
	var cfg consul.RaftLogStoreConfig
	if raw != nil {
		cfg.Backend = stringValWithDefault(raw.Backend, consul.LogStoreBackendBoltDB)
		cfg.DisableLogCache = boolVal(raw.DisableLogCache)

		cfg.Verification.Enabled = boolVal(raw.Verification.Enabled)
		cfg.Verification.Interval = b.durationVal("raft_logstore.verification.interval", raw.Verification.Interval)

		cfg.BoltDB.NoFreelistSync = boolVal(raw.BoltDBConfig.NoFreelistSync)

		cfg.WAL.SegmentSize = intVal(raw.WALConfig.SegmentSizeMB) * 1024 * 1024
	}
	return cfg
}
